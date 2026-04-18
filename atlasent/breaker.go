package atlasent

import (
	"sync"
	"sync/atomic"
	"time"
)

// BreakerState is the current state of the circuit breaker.
type BreakerState int

const (
	// BreakerClosed is the normal state: all requests pass through.
	BreakerClosed BreakerState = iota
	// BreakerOpen trips after sustained failures: requests fail fast
	// without touching the PDP until the cool-down elapses.
	BreakerOpen
	// BreakerHalfOpen lets one probe through after cool-down; a success
	// closes the breaker, a failure re-opens it.
	BreakerHalfOpen
)

// breakerError is the synthetic error returned when the breaker is open.
// It is classified as KindTransport so IsTransport / fail-closed / retry
// semantics match the real PDP-unreachable case.
func breakerError() error {
	return &APIError{Kind: KindTransport, Cause: errBreakerOpen}
}

// BreakerConfig configures a circuit breaker.
type BreakerConfig struct {
	// FailureThreshold trips the breaker after this many consecutive
	// failures. Zero disables the breaker.
	FailureThreshold int
	// CoolDown is how long the breaker stays open before attempting a
	// probe in half-open.
	CoolDown time.Duration
	// now is injectable for tests.
	now func() time.Time
}

// DefaultBreakerConfig: trip after 5 consecutive failures, probe after 30s.
var DefaultBreakerConfig = BreakerConfig{
	FailureThreshold: 5,
	CoolDown:         30 * time.Second,
}

// breaker tracks consecutive failures and gates HTTP calls accordingly.
type breaker struct {
	cfg BreakerConfig

	mu          sync.Mutex
	state       BreakerState
	failures    int
	openedAt    time.Time
	probeInFlight atomic.Bool
}

func newBreaker(cfg BreakerConfig) *breaker {
	if cfg.now == nil {
		cfg.now = time.Now
	}
	return &breaker{cfg: cfg}
}

// allow reports whether the caller may send a request right now.
func (b *breaker) allow() bool {
	if b.cfg.FailureThreshold <= 0 {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case BreakerClosed:
		return true
	case BreakerOpen:
		if b.cfg.now().Sub(b.openedAt) >= b.cfg.CoolDown {
			b.state = BreakerHalfOpen
			b.probeInFlight.Store(false)
			return b.claimProbeLocked()
		}
		return false
	case BreakerHalfOpen:
		return b.claimProbeLocked()
	}
	return true
}

// claimProbeLocked returns true if this goroutine gets to send the probe;
// subsequent callers are rejected until the probe completes. Caller holds mu.
func (b *breaker) claimProbeLocked() bool {
	return b.probeInFlight.CompareAndSwap(false, true)
}

// onSuccess closes the breaker and clears failure count.
func (b *breaker) onSuccess() {
	if b.cfg.FailureThreshold <= 0 {
		return
	}
	b.mu.Lock()
	b.failures = 0
	b.state = BreakerClosed
	b.probeInFlight.Store(false)
	b.mu.Unlock()
}

// onFailure increments the failure count and trips the breaker if the
// threshold is hit. In half-open, any failure immediately re-opens.
func (b *breaker) onFailure() {
	if b.cfg.FailureThreshold <= 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == BreakerHalfOpen {
		b.state = BreakerOpen
		b.openedAt = b.cfg.now()
		b.probeInFlight.Store(false)
		return
	}
	b.failures++
	if b.failures >= b.cfg.FailureThreshold {
		b.state = BreakerOpen
		b.openedAt = b.cfg.now()
	}
}

// State returns the current state (BreakerClosed, BreakerOpen, BreakerHalfOpen).
func (b *breaker) State() BreakerState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// WithCircuitBreaker installs a circuit breaker on the Client. When tripped,
// subsequent Checks fail fast with a transport-kind APIError until the
// cool-down elapses — avoiding dog-piling on a known-down PDP.
//
// Fail-closed clients still return a Deny decision on breaker-open; the
// PDP is just never contacted.
func WithCircuitBreaker(cfg BreakerConfig) Option {
	return func(c *Client) { c.breaker = newBreaker(cfg) }
}
