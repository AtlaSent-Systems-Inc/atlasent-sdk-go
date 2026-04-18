package atlasent

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Principal identifies who is acting: a user, a service account, a machine.
type Principal struct {
	ID         string            `json:"id"`
	Type       string            `json:"type,omitempty"`
	Attributes map[string]any    `json:"attributes,omitempty"`
	Groups     []string          `json:"groups,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
}

// Resource identifies the thing being acted upon.
type Resource struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// CheckRequest describes a single authorization question: may Principal
// perform Action on Resource, given Context.
type CheckRequest struct {
	Principal Principal      `json:"principal"`
	Action    string         `json:"action"`
	Resource  Resource       `json:"resource"`
	Context   map[string]any `json:"context,omitempty"`
	// DryRun asks the PDP to evaluate the request without emitting audit
	// records or side-effects (useful for "what-if" pages, policy authoring,
	// UI affordance pre-checks).
	DryRun bool `json:"dry_run,omitempty"`
}

// Validate reports whether a request has the minimum fields required. Only
// Action is strictly required — empty principals and resources sometimes
// make sense (global admin actions, anonymous reads). The PDP may reject
// more; this is a cheap local fast-fail.
func (r CheckRequest) Validate() error {
	if r.Action == "" {
		return &APIError{Kind: KindValidation, Cause: errAction}
	}
	return nil
}

var errAction = errors.New("empty action")

// Decision is the PDP's answer.
type Decision struct {
	// Allowed is the enforcement bit. Do not check any other field before this.
	Allowed bool `json:"allowed"`
	// Reason is a short human-readable explanation, safe to log.
	Reason string `json:"reason,omitempty"`
	// PolicyID identifies the policy that produced the decision (for audit).
	PolicyID string `json:"policy_id,omitempty"`
	// Obligations are side-effects the caller MUST honor when enforcing
	// (e.g. "redact:ssn", "log:high-risk"). Ignore obligations you don't
	// understand at your own risk.
	Obligations []string `json:"obligations,omitempty"`
	// TTLMillis is the PDP's recommended cache lifetime. When >0 and a Cache
	// is configured, the SDK caches this decision for that long. Zero means
	// "fall back to the client-configured default TTL".
	TTLMillis int64 `json:"ttl_ms,omitempty"`
}

// ErrDenied is returned by Guard when the PDP denies a request. Callers can
// type-assert to *DeniedError to recover the underlying Decision.
var ErrDenied = errors.New("atlasent: access denied")

// DeniedError wraps ErrDenied with the full Decision.
type DeniedError struct{ Decision Decision }

func (e *DeniedError) Error() string {
	if e.Decision.Reason != "" {
		return fmt.Sprintf("%s: %s", ErrDenied.Error(), e.Decision.Reason)
	}
	return ErrDenied.Error()
}

func (e *DeniedError) Unwrap() error { return ErrDenied }

// HasObligation reports whether d carries the given obligation string. Case
// sensitive, exact match.
func (d Decision) HasObligation(o string) bool {
	for _, x := range d.Obligations {
		if x == o {
			return true
		}
	}
	return false
}

// cacheTTL returns the effective TTL for a decision, preferring the PDP hint
// over the client-configured default.
func (c *Client) cacheTTL(d Decision) time.Duration {
	if d.TTLMillis > 0 {
		return time.Duration(d.TTLMillis) * time.Millisecond
	}
	return c.cacheDefaultTTL
}

// Check asks the PDP whether the request is allowed. It returns a Decision
// and a non-nil error only on transport/protocol failures. On transport
// failure, the returned Decision honors Client.FailClosed.
//
// If a Cache is configured, Check consults it first and skips the HTTP call
// on a hit. Observers are notified on every call (cache hit or miss).
func (c *Client) Check(ctx context.Context, req CheckRequest) (Decision, error) {
	if err := req.Validate(); err != nil {
		return Decision{Allowed: false, Reason: "invalid request"}, err
	}
	if c.cache != nil {
		if dec, ok := c.cache.Get(ctx, cacheKey(req)); ok {
			c.observe(ctx, CheckEvent{
				Request:  req,
				Decision: dec,
				CacheHit: true,
			})
			return dec, nil
		}
	}

	start := time.Now()
	var d Decision
	attempts, err := c.postJSON(ctx, "/v1/authorize", req, &d)
	latency := time.Since(start)

	if err != nil {
		ev := CheckEvent{Request: req, Err: err, Latency: latency, Attempts: attempts}
		if c.FailClosed {
			ev.Decision = Decision{Allowed: false, Reason: "pdp unavailable (fail-closed)"}
			c.observe(ctx, ev)
			return ev.Decision, err
		}
		ev.Decision = Decision{Allowed: true, Reason: "pdp unavailable (fail-open)"}
		c.observe(ctx, ev)
		return ev.Decision, err
	}

	if c.cache != nil {
		if ttl := c.cacheTTL(d); ttl > 0 {
			c.cache.Set(ctx, cacheKey(req), d, ttl)
		}
	}
	c.observe(ctx, CheckEvent{Request: req, Decision: d, Latency: latency, Attempts: attempts})
	return d, nil
}

// observe invokes the configured Observer (if any) without panicking on
// observer misbehavior.
func (c *Client) observe(ctx context.Context, ev CheckEvent) {
	if c.observer == nil {
		return
	}
	defer func() { _ = recover() }()
	c.observer.OnCheck(ctx, ev)
}

// Guard is the execution-time enforcement primitive: it calls Check and, if
// the request is allowed, runs fn. If denied, it returns a *DeniedError
// wrapping the Decision and fn is never executed.
//
// Use Guard to wrap the exact call-site where a sensitive action happens —
// a database mutation, a file write, an outbound payment — so the
// authorization decision is colocated with the side-effect it gates.
func Guard[T any](ctx context.Context, c *Client, req CheckRequest, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	decision, err := c.Check(ctx, req)
	if err != nil && c.FailClosed {
		return zero, err
	}
	if !decision.Allowed {
		return zero, &DeniedError{Decision: decision}
	}
	return fn(ctx)
}
