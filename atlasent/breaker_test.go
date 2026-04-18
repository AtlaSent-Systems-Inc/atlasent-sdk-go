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

func TestBreakerTripsAfterThreshold(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c, _ := New("k",
		WithBaseURL(srv.URL),
		WithCircuitBreaker(BreakerConfig{FailureThreshold: 3, CoolDown: time.Hour}),
	)
	req := CheckRequest{Principal: Principal{ID: "u"}, Action: "x", Resource: Resource{ID: "r", Type: "doc"}}

	for i := 0; i < 3; i++ {
		if _, err := c.Check(context.Background(), req); err == nil {
			t.Fatalf("attempt %d: want error", i)
		}
	}
	beforeTrip := hits.Load()
	// Next call must not reach the server.
	_, err := c.Check(context.Background(), req)
	if !IsBreakerOpen(err) {
		t.Fatalf("want IsBreakerOpen, got %v", err)
	}
	if hits.Load() != beforeTrip {
		t.Fatalf("breaker did not block: hits %d -> %d", beforeTrip, hits.Load())
	}
}

func TestBreakerHalfOpenRecovers(t *testing.T) {
	var healthy atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !healthy.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(Decision{Allowed: true})
	}))
	defer srv.Close()

	now := time.Unix(0, 0)
	cfg := BreakerConfig{FailureThreshold: 2, CoolDown: 10 * time.Second}
	cfg.now = func() time.Time { return now }

	c, _ := New("k", WithBaseURL(srv.URL), WithCircuitBreaker(cfg))
	req := CheckRequest{Principal: Principal{ID: "u"}, Action: "x", Resource: Resource{ID: "r", Type: "doc"}}

	// Trip the breaker.
	for i := 0; i < 2; i++ {
		_, _ = c.Check(context.Background(), req)
	}
	// Confirm it's open.
	if _, err := c.Check(context.Background(), req); !IsBreakerOpen(err) {
		t.Fatalf("want breaker open, got %v", err)
	}

	// Advance past cool-down and fix the server.
	now = now.Add(11 * time.Second)
	healthy.Store(true)

	// One probe should go through and succeed, closing the breaker.
	d, err := c.Check(context.Background(), req)
	if err != nil || !d.Allowed {
		t.Fatalf("probe should succeed, got %+v err=%v", d, err)
	}
	// And subsequent calls pass.
	if _, err := c.Check(context.Background(), req); err != nil {
		t.Fatalf("after recovery: %v", err)
	}
}

func TestBreakerHalfOpenReopensOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	now := time.Unix(0, 0)
	cfg := BreakerConfig{FailureThreshold: 2, CoolDown: 5 * time.Second}
	cfg.now = func() time.Time { return now }

	c, _ := New("k", WithBaseURL(srv.URL), WithCircuitBreaker(cfg))
	req := CheckRequest{Principal: Principal{ID: "u"}, Action: "x", Resource: Resource{ID: "r", Type: "doc"}}

	// Trip.
	for i := 0; i < 2; i++ {
		_, _ = c.Check(context.Background(), req)
	}
	// Wait past cool-down.
	now = now.Add(6 * time.Second)
	// Probe fails.
	_, _ = c.Check(context.Background(), req)
	// Should be open again immediately.
	if _, err := c.Check(context.Background(), req); !IsBreakerOpen(err) {
		t.Fatalf("expected breaker to re-open, got %v", err)
	}
}
