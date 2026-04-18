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

func TestCheckAllow(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing/wrong bearer token: %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(Decision{Allowed: true, PolicyID: "p1"})
	}))

	d, err := c.Check(context.Background(), CheckRequest{
		Principal: Principal{ID: "u"},
		Action:    "read",
		Resource:  Resource{ID: "r", Type: "doc"},
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !d.Allowed || d.PolicyID != "p1" {
		t.Fatalf("unexpected decision %+v", d)
	}
}

func TestGuardDeny(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Decision{Allowed: false, Reason: "not owner"})
	}))

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
}
