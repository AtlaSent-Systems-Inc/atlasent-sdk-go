package atlasent

import (
	"context"
	"errors"
	"fmt"
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
}

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

// Check asks the PDP whether the request is allowed. It returns a Decision
// and a non-nil error only on transport/protocol failures. On transport
// failure, the returned Decision honors Client.FailClosed.
func (c *Client) Check(ctx context.Context, req CheckRequest) (Decision, error) {
	var d Decision
	if err := c.postJSON(ctx, "/v1/authorize", req, &d); err != nil {
		if c.FailClosed {
			return Decision{Allowed: false, Reason: "pdp unavailable (fail-closed)"}, err
		}
		return Decision{Allowed: true, Reason: "pdp unavailable (fail-open)"}, err
	}
	return d, nil
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
