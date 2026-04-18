package atlasent

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// Observer receives a callback for every Check (including cache hits and
// fail-open/fail-closed outcomes). Implementations must be safe for
// concurrent use and should return quickly; do not block the call site.
//
// Wire an Observer to emit metrics, structured logs, or OpenTelemetry spans.
type Observer interface {
	OnCheck(ctx context.Context, ev CheckEvent)
}

// CheckEvent describes the outcome of a single authorization check.
type CheckEvent struct {
	Request  CheckRequest
	Decision Decision
	// Err is the transport/protocol error, if any. A non-nil Err with
	// Decision.Allowed=false indicates a fail-closed outcome.
	Err error
	// Latency is the wall-clock time spent (including retries). Zero for
	// cache hits.
	Latency time.Duration
	// CacheHit is true when the decision came from the local cache.
	CacheHit bool
	// Attempts is the number of HTTP attempts made (1 on success-first-try,
	// 0 on cache hit).
	Attempts int
}

// ObserverFunc adapts a plain function to the Observer interface.
type ObserverFunc func(ctx context.Context, ev CheckEvent)

// OnCheck implements Observer.
func (f ObserverFunc) OnCheck(ctx context.Context, ev CheckEvent) { f(ctx, ev) }

// WithObserver installs o on the Client. Later calls replace earlier ones;
// compose multiple observers with MultiObserver.
func WithObserver(o Observer) Option { return func(c *Client) { c.observer = o } }

// MultiObserver fans an event out to every observer in order. A nil entry is
// skipped. Use when you want both metrics and tracing, for example. A panic
// in one observer is recovered so later observers still run; the panic is
// discarded.
func MultiObserver(obs ...Observer) Observer {
	return ObserverFunc(func(ctx context.Context, ev CheckEvent) {
		for _, o := range obs {
			if o != nil {
				safeInvoke(ctx, o, ev)
			}
		}
	})
}

func safeInvoke(ctx context.Context, o Observer, ev CheckEvent) {
	defer func() { _ = recover() }()
	o.OnCheck(ctx, ev)
}

// SlogObserver logs each event at info (allow) or warn (deny / error) on the
// given slog.Logger. Fields are fixed to keep cardinality bounded.
func SlogObserver(logger *slog.Logger) Observer {
	if logger == nil {
		logger = slog.Default()
	}
	return ObserverFunc(func(ctx context.Context, ev CheckEvent) {
		attrs := []any{
			slog.String("action", ev.Request.Action),
			slog.String("resource_type", ev.Request.Resource.Type),
			slog.String("principal_id", ev.Request.Principal.ID),
			slog.Bool("allowed", ev.Decision.Allowed),
			slog.String("policy_id", ev.Decision.PolicyID),
			slog.Duration("latency", ev.Latency),
			slog.Bool("cache_hit", ev.CacheHit),
			slog.Int("attempts", ev.Attempts),
		}
		if ev.Err != nil {
			attrs = append(attrs, slog.String("err", ev.Err.Error()))
			logger.LogAttrs(ctx, slog.LevelWarn, "atlasent.check", asAttrs(attrs)...)
			return
		}
		if !ev.Decision.Allowed {
			logger.LogAttrs(ctx, slog.LevelWarn, "atlasent.check", asAttrs(attrs)...)
			return
		}
		logger.LogAttrs(ctx, slog.LevelInfo, "atlasent.check", asAttrs(attrs)...)
	})
}

func asAttrs(in []any) []slog.Attr {
	out := make([]slog.Attr, 0, len(in))
	for _, v := range in {
		if a, ok := v.(slog.Attr); ok {
			out = append(out, a)
		}
	}
	return out
}

// Counters is a lightweight, atomic allow/deny/error counter. Useful when
// wiring a Prometheus/OTel metric is overkill. Safe for concurrent use.
type Counters struct {
	Allow     atomic.Int64
	Deny      atomic.Int64
	Errors    atomic.Int64
	CacheHits atomic.Int64
}

// OnCheck implements Observer.
func (c *Counters) OnCheck(_ context.Context, ev CheckEvent) {
	if ev.CacheHit {
		c.CacheHits.Add(1)
	}
	if ev.Err != nil {
		c.Errors.Add(1)
	}
	if ev.Decision.Allowed {
		c.Allow.Add(1)
	} else {
		c.Deny.Add(1)
	}
}
