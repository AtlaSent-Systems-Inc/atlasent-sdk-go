package atlasent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
}

func newMiddlewareClient(t *testing.T, allow bool, reason string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Decision{Allowed: allow, Reason: reason, PolicyID: "p"})
	}))
	t.Cleanup(srv.Close)
	c, err := New("k", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestMiddleware_Allow(t *testing.T) {
	c := newMiddlewareClient(t, true, "")
	resolve := func(r *http.Request) (string, Resource, map[string]any, error) {
		return "read", Resource{ID: "x", Type: "doc"}, nil, nil
	}
	h := c.HTTPMiddleware(resolve)(okHandler())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req = req.WithContext(WithPrincipal(req.Context(), Principal{ID: "u"}))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMiddleware_Deny403(t *testing.T) {
	c := newMiddlewareClient(t, false, "not owner")
	resolve := func(r *http.Request) (string, Resource, map[string]any, error) {
		return "pay", Resource{ID: "x", Type: "invoice"}, nil, nil
	}
	h := c.HTTPMiddleware(resolve)(okHandler())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req = req.WithContext(WithPrincipal(req.Context(), Principal{ID: "u"}))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v: %s", err, rec.Body.String())
	}
	if body["reason"] != "not owner" || body["policy_id"] != "p" {
		t.Fatalf("wrong body: %+v", body)
	}
}

func TestMiddleware_Unauth401(t *testing.T) {
	c := newMiddlewareClient(t, true, "")
	h := c.HTTPMiddleware(func(r *http.Request) (string, Resource, map[string]any, error) {
		return "read", Resource{ID: "x"}, nil, nil
	})(okHandler())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil) // no principal
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestMiddleware_ResolveError400(t *testing.T) {
	c := newMiddlewareClient(t, true, "")
	h := c.HTTPMiddleware(func(r *http.Request) (string, Resource, map[string]any, error) {
		return "", Resource{}, nil, errResolve
	})(okHandler())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/bad", nil)
	req = req.WithContext(WithPrincipal(req.Context(), Principal{ID: "u"}))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "bad resource") {
		t.Fatalf("want resolver error in body, got %q", rec.Body.String())
	}
}

func TestMiddleware_FailClosed503(t *testing.T) {
	c, _ := New("k", WithBaseURL("http://127.0.0.1:1"))
	h := c.HTTPMiddleware(func(r *http.Request) (string, Resource, map[string]any, error) {
		return "read", Resource{ID: "x"}, nil, nil
	})(okHandler())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req = req.WithContext(WithPrincipal(req.Context(), Principal{ID: "u"}))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", rec.Code)
	}
}

func TestMiddleware_FailOpenPasses(t *testing.T) {
	c, _ := New("k", WithBaseURL("http://127.0.0.1:1"), WithFailOpen())
	h := c.HTTPMiddleware(func(r *http.Request) (string, Resource, map[string]any, error) {
		return "read", Resource{ID: "x"}, nil, nil
	})(okHandler())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req = req.WithContext(WithPrincipal(req.Context(), Principal{ID: "u"}))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("fail-open: want 200, got %d", rec.Code)
	}
}

func TestPrincipalFromMissing(t *testing.T) {
	if _, ok := PrincipalFrom(context.Background()); ok {
		t.Fatal("expected PrincipalFrom to miss on bare context")
	}
}

// errResolve is a sentinel resolver error used in middleware tests.
var errResolve = &resolveErr{"bad resource"}

type resolveErr struct{ s string }

func (e *resolveErr) Error() string { return e.s }
