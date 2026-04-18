package atlasent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryAfterHeaderSeconds(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "7")
	if got := parseRetryAfter(h); got != 7*time.Second {
		t.Fatalf("want 7s, got %v", got)
	}
}

func TestRetryAfterHeaderDate(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", time.Now().Add(3*time.Second).UTC().Format(http.TimeFormat))
	got := parseRetryAfter(h)
	if got < time.Second || got > 4*time.Second {
		t.Fatalf("want ~3s, got %v", got)
	}
}

func TestRetryAfterMissing(t *testing.T) {
	if got := parseRetryAfter(http.Header{}); got != 0 {
		t.Fatalf("want 0, got %v", got)
	}
}

func TestRetryableStatus(t *testing.T) {
	for _, s := range []int{429, 500, 502, 503, 504} {
		if !retryableStatus(s) {
			t.Fatalf("%d should be retryable", s)
		}
	}
	for _, s := range []int{200, 400, 401, 403, 404, 501} {
		if retryableStatus(s) {
			t.Fatalf("%d should not be retryable", s)
		}
	}
}

func TestClientRetriesOn503(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(Decision{Allowed: true, PolicyID: "p"})
	}))
	defer srv.Close()

	c, _ := New("k",
		WithBaseURL(srv.URL),
		WithRetry(RetryPolicy{MaxAttempts: 3, InitialBackoff: time.Millisecond, Multiplier: 2}),
	)
	d, err := c.Check(context.Background(), CheckRequest{Action: "a"})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !d.Allowed {
		t.Fatalf("want allowed, got %+v", d)
	}
	if hits.Load() != 3 {
		t.Fatalf("want 3 attempts, got %d", hits.Load())
	}
}

func TestClientDoesNotRetry4xx(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c, _ := New("k",
		WithBaseURL(srv.URL),
		WithRetry(RetryPolicy{MaxAttempts: 4, InitialBackoff: time.Millisecond}),
	)
	_, err := c.Check(context.Background(), CheckRequest{Action: "a"})
	if err == nil {
		t.Fatal("expected error")
	}
	if hits.Load() != 1 {
		t.Fatalf("400 should not retry, got %d attempts", hits.Load())
	}
}

func TestBackoffGrowsAndCaps(t *testing.T) {
	p := RetryPolicy{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     400 * time.Millisecond,
		Multiplier:     2.0,
	}
	// No jitter, rng nil is fine.
	if got := p.backoffFor(1, nil); got != 100*time.Millisecond {
		t.Fatalf("attempt 1: want 100ms, got %v", got)
	}
	if got := p.backoffFor(2, nil); got != 200*time.Millisecond {
		t.Fatalf("attempt 2: want 200ms, got %v", got)
	}
	if got := p.backoffFor(3, nil); got != 400*time.Millisecond {
		t.Fatalf("attempt 3: want 400ms, got %v", got)
	}
	if got := p.backoffFor(10, nil); got != 400*time.Millisecond {
		t.Fatalf("attempt 10 cap: want 400ms, got %v", got)
	}
}
