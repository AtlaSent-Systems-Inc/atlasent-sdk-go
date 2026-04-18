package atlasent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := New("test-key", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// evaluateAllowHandler returns a canonical ALLOW response for /v1-evaluate.
func evaluateAllowHandler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer test-key"; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want application/json", got)
		}
		if r.Header.Get("X-Request-ID") == "" {
			t.Error("X-Request-ID header missing")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"permitted":   true,
			"decision_id": "dec-1",
			"reason":      "ok",
			"audit_hash":  "h-1",
			"timestamp":   "2026-04-18T00:00:00Z",
		})
	})
}

// evaluateDenyHandler returns a canonical DENY.
func evaluateDenyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"permitted":   false,
			"decision_id": "dec-deny",
			"reason":      "not owner",
		})
	})
}

func TestCheckAllow(t *testing.T) {
	c := newTestClient(t, evaluateAllowHandler(t))

	d, err := c.Check(context.Background(), CheckRequest{
		Principal: Principal{ID: "u"},
		Action:    "read",
		Resource:  Resource{ID: "r", Type: "doc"},
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !d.Allowed || d.PolicyID != "dec-1" {
		t.Fatalf("unexpected decision %+v", d)
	}
}

func TestGuardDeny(t *testing.T) {
	c := newTestClient(t, evaluateDenyHandler())

	ran := false
	_, err := Guard(context.Background(), c, CheckRequest{Action: "pay"}, func(ctx context.Context) (int, error) {
		ran = true
		return 1, nil
	})
	if ran {
		t.Fatal("fn ran on denied decision")
	}
	var denied *DeniedError
	if !errors.As(err, &denied) {
		t.Fatalf("want DeniedError, got %v", err)
	}
	if !errors.Is(err, ErrDenied) {
		t.Fatalf("want errors.Is ErrDenied, got %v", err)
	}
	if denied.Decision.Reason != "not owner" {
		t.Fatalf("lost decision reason: %+v", denied.Decision)
	}
}

func TestCheckFailClosed(t *testing.T) {
	// Point at an address that refuses connections.
	c, _ := New("k", WithBaseURL("http://127.0.0.1:1"))
	d, err := c.Check(context.Background(), CheckRequest{Action: "x"})
	if err == nil {
		t.Fatal("expected transport error")
	}
	if d.Allowed {
		t.Fatal("fail-closed must deny on transport error")
	}
	if !IsCode(err, ErrNetwork) {
		t.Fatalf("want ErrNetwork, got %+v", AsError(err))
	}
}

// TestCheckSendsAgentAndContext verifies that CheckRequest.Principal.ID
// maps to the wire's actor_id and Resource is embedded in context.
func TestCheckSendsAgentAndContext(t *testing.T) {
	var got map[string]any
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"permitted":   true,
			"decision_id": "d",
		})
	}))
	_, err := c.Check(context.Background(), CheckRequest{
		Principal: Principal{ID: "user_alice", Groups: []string{"finance"}},
		Action:    "invoice.pay",
		Resource:  Resource{ID: "inv_42", Type: "invoice"},
		Context:   map[string]any{"ip": "203.0.113.7"},
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got["actor_id"] != "user_alice" {
		t.Errorf("actor_id = %v, want user_alice", got["actor_id"])
	}
	if got["action_type"] != "invoice.pay" {
		t.Errorf("action_type = %v, want invoice.pay", got["action_type"])
	}
	ctx, _ := got["context"].(map[string]any)
	if _, ok := ctx["resource"]; !ok {
		t.Errorf("context.resource missing, got %+v", ctx)
	}
	if ctx["ip"] != "203.0.113.7" {
		t.Errorf("context.ip = %v, want 203.0.113.7", ctx["ip"])
	}
}
