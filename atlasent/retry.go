package atlasent

import (
	"context"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

// RetryPolicy configures how Check retries failed PDP calls. The zero value
// disables retries.
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts, including the first. A
	// value of 1 (or 0) disables retries.
	MaxAttempts int
	// InitialBackoff is the delay before the first retry.
	InitialBackoff time.Duration
	// MaxBackoff caps the exponential backoff.
	MaxBackoff time.Duration
	// Multiplier grows the backoff each attempt (typical: 2.0).
	Multiplier float64
	// Jitter randomizes each backoff by up to ±50% to avoid thundering herds.
	Jitter bool
}

// DefaultRetryPolicy is a conservative retry policy suitable for most callers:
// up to 3 attempts, 100ms → 1s with jitter.
var DefaultRetryPolicy = RetryPolicy{
	MaxAttempts:    3,
	InitialBackoff: 100 * time.Millisecond,
	MaxBackoff:     1 * time.Second,
	Multiplier:     2.0,
	Jitter:         true,
}

// WithRetry installs a retry policy on the Client.
func WithRetry(p RetryPolicy) Option { return func(c *Client) { c.retry = p } }

// retryableStatus reports whether status should trigger a retry. 429 and
// 5xx (except 501 Not Implemented) are retryable; everything else is not.
func retryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

// parseRetryAfter honors the Retry-After header (seconds or HTTP-date). It
// returns 0 when the header is absent or unparseable.
func parseRetryAfter(h http.Header) time.Duration {
	v := h.Get("Retry-After")
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// backoffFor returns the delay before attempt n (1-indexed retry count).
// jitterFloat supplies a [0,1) random; tests inject a fake to make output
// deterministic. Production callers should pass jitterRand, which uses the
// goroutine-safe math/rand/v2 package source.
func (p RetryPolicy) backoffFor(attempt int, jitterFloat func() float64) time.Duration {
	if p.InitialBackoff <= 0 {
		return 0
	}
	mult := p.Multiplier
	if mult <= 0 {
		mult = 2.0
	}
	d := float64(p.InitialBackoff)
	for i := 1; i < attempt; i++ {
		d *= mult
	}
	if max := float64(p.MaxBackoff); max > 0 && d > max {
		d = max
	}
	if p.Jitter && jitterFloat != nil {
		// ±50% jitter.
		d = d * (0.5 + jitterFloat())
	}
	return time.Duration(d)
}

// jitterRand is the default jitter source. math/rand/v2 package-level
// functions are safe for concurrent use.
func jitterRand() float64 { return rand.Float64() }

// sleepCtx sleeps for d or returns ctx.Err() if the context is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
