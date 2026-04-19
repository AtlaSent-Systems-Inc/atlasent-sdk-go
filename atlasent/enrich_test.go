package atlasent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func captureServer(t *testing.T) (*httptest.Server, *[]CheckRequest) {
	t.Helper()
	var got []CheckRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CheckRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		got = append(got, req)
		_ = json.NewEncoder(w).Encode(Decision{Allowed: true})
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func TestContextEnricherMergesIntoContext(t *testing.T) {
	srv, got := captureServer(t)
	c, _ := New("k",
		WithBaseURL(srv.URL),
		WithContextEnricher(func(_ context.Context) map[string]any {
			return map[string]any{"tenant": "acme", "request_id": "abc"}
		}),
	)
	_, err := c.Check(context.Background(), CheckRequest{
		Principal: Principal{ID: "u"},
		Action:    "read",
		Resource:  Resource{ID: "r", Type: "doc"},
		Context:   map[string]any{"ip": "1.2.3.4"},
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	rc := (*got)[0].Context
	if rc["ip"] != "1.2.3.4" {
		t.Errorf("caller context lost: %v", rc)
	}
	if rc["tenant"] != "acme" || rc["request_id"] != "abc" {
		t.Errorf("enricher value missing: %v", rc)
	}
}

func TestContextEnricherCallerWins(t *testing.T) {
	srv, got := captureServer(t)
	c, _ := New("k",
		WithBaseURL(srv.URL),
		WithContextEnricher(func(_ context.Context) map[string]any {
			return map[string]any{"request_id": "from-enricher"}
		}),
	)
	_, _ = c.Check(context.Background(), CheckRequest{
		Principal: Principal{ID: "u"},
		Action:    "read",
		Resource:  Resource{ID: "r", Type: "doc"},
		Context:   map[string]any{"request_id": "from-caller"},
	})
	if (*got)[0].Context["request_id"] != "from-caller" {
		t.Fatalf("caller value should win, got %v", (*got)[0].Context)
	}
}

func TestRequestIDEnricher(t *testing.T) {
	srv, got := captureServer(t)
	c, _ := New("k",
		WithBaseURL(srv.URL),
		WithContextEnricher(RequestIDEnricher()),
	)
	ctx := WithRequestID(context.Background(), "req-42")
	_, _ = c.Check(ctx, CheckRequest{
		Principal: Principal{ID: "u"},
		Action:    "read",
		Resource:  Resource{ID: "r", Type: "doc"},
	})
	if (*got)[0].Context["request_id"] != "req-42" {
		t.Fatalf("want request_id=req-42, got %v", (*got)[0].Context)
	}
}

func TestRequestIDEnricherAbsent(t *testing.T) {
	srv, got := captureServer(t)
	c, _ := New("k",
		WithBaseURL(srv.URL),
		WithContextEnricher(RequestIDEnricher()),
	)
	_, _ = c.Check(context.Background(), CheckRequest{
		Principal: Principal{ID: "u"},
		Action:    "read",
		Resource:  Resource{ID: "r", Type: "doc"},
	})
	if _, has := (*got)[0].Context["request_id"]; has {
		t.Fatalf("request_id should not be set when ctx has no ID: %v", (*got)[0].Context)
	}
}

func TestChainEnrichers(t *testing.T) {
	e := ChainEnrichers(
		func(_ context.Context) map[string]any { return map[string]any{"a": 1, "shared": "first"} },
		func(_ context.Context) map[string]any { return map[string]any{"b": 2, "shared": "second"} },
	)
	out := e(context.Background())
	if out["a"] != 1 || out["b"] != 2 {
		t.Fatalf("missing keys: %v", out)
	}
	if out["shared"] != "first" {
		t.Fatalf("earlier enricher should win on collision, got %v", out["shared"])
	}
}

func TestBreakerOnStateChange(t *testing.T) {
	type trans struct{ from, to BreakerState }
	var transitions []trans

	now := time.Unix(0, 0)
	cfg := BreakerConfig{
		FailureThreshold: 2,
		CoolDown:         5 * time.Second,
		OnStateChange: func(from, to BreakerState) {
			transitions = append(transitions, trans{from, to})
		},
	}
	cfg.now = func() time.Time { return now }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c, _ := New("k", WithBaseURL(srv.URL), WithCircuitBreaker(cfg))
	req := CheckRequest{Principal: Principal{ID: "u"}, Action: "x", Resource: Resource{ID: "r", Type: "doc"}}

	for i := 0; i < 2; i++ {
		_, _ = c.Check(context.Background(), req)
	}
	// Closed → Open once threshold hit.
	if len(transitions) != 1 || transitions[0] != (trans{BreakerClosed, BreakerOpen}) {
		t.Fatalf("after trip: %+v", transitions)
	}

	// Advance past cool-down and make one more call; Open → HalfOpen.
	now = now.Add(6 * time.Second)
	_, _ = c.Check(context.Background(), req)
	if len(transitions) < 2 || transitions[1] != (trans{BreakerOpen, BreakerHalfOpen}) {
		t.Fatalf("after cool-down: %+v", transitions)
	}
}
