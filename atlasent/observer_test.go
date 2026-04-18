package atlasent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestObserverAllowAndDenyCounted(t *testing.T) {
	var allow, deny bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allow {
			_ = json.NewEncoder(w).Encode(Decision{Allowed: false, Reason: "nope"})
			return
		}
		_ = json.NewEncoder(w).Encode(Decision{Allowed: true, PolicyID: "p"})
	}))
	defer srv.Close()

	var counters Counters
	c, _ := New("k", WithBaseURL(srv.URL), WithObserver(&counters))

	allow, deny = false, true
	_, _ = c.Check(context.Background(), CheckRequest{Action: "x"})
	allow = true
	_, _ = c.Check(context.Background(), CheckRequest{Action: "y"})

	if counters.Allow.Load() != 1 || counters.Deny.Load() != 1 {
		t.Fatalf("want 1/1, got allow=%d deny=%d", counters.Allow.Load(), counters.Deny.Load())
	}
	_ = deny
}

func TestObserverCacheHitFlagged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Decision{Allowed: true, TTLMillis: 60_000})
	}))
	defer srv.Close()

	var hits int
	obs := ObserverFunc(func(_ context.Context, ev CheckEvent) {
		if ev.CacheHit {
			hits++
		}
	})
	c, _ := New("k", WithBaseURL(srv.URL), WithCache(NewMemoryCache(8), 0), WithObserver(obs))

	req := CheckRequest{Action: "a", Resource: Resource{ID: "r", Type: "doc"}}
	_, _ = c.Check(context.Background(), req) // miss
	_, _ = c.Check(context.Background(), req) // hit

	if hits != 1 {
		t.Fatalf("want exactly 1 cache-hit event, got %d", hits)
	}
}

func TestObserverSurvivesPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Decision{Allowed: true})
	}))
	defer srv.Close()

	obs := ObserverFunc(func(_ context.Context, _ CheckEvent) { panic("boom") })
	c, _ := New("k", WithBaseURL(srv.URL), WithObserver(obs))

	// Must not panic.
	if _, err := c.Check(context.Background(), CheckRequest{Action: "x"}); err != nil {
		t.Fatalf("Check: %v", err)
	}
}

func TestDecisionHasObligation(t *testing.T) {
	d := Decision{Obligations: []string{"redact:ssn", "log:high-risk"}}
	if !d.HasObligation("log:high-risk") {
		t.Fatal("missed existing obligation")
	}
	if d.HasObligation("nope") {
		t.Fatal("false positive obligation")
	}
}
