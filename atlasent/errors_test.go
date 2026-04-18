package atlasent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// httpStatusHandler returns a fixed status + optional body + headers.
func httpStatusHandler(status int, body string, headers map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}

func newClient(t *testing.T, h http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, _ := New("k", WithBaseURL(srv.URL))
	return c
}

func TestError401InvalidAPIKey(t *testing.T) {
	c := newClient(t, httpStatusHandler(401, `{"message":"Invalid API key"}`, nil))
	_, err := c.Evaluate(context.Background(), EvaluateRequest{Agent: "a", Action: "x"})
	if !IsCode(err, ErrInvalidAPIKey) {
		t.Fatalf("want invalid_api_key, got %+v", AsError(err))
	}
	if e := AsError(err); e.Status != 401 || e.Message != "Invalid API key" {
		t.Errorf("Status/Message wrong: %+v", e)
	}
}

func TestError403Forbidden(t *testing.T) {
	c := newClient(t, httpStatusHandler(403, `{"message":"scope mismatch"}`, nil))
	_, err := c.Evaluate(context.Background(), EvaluateRequest{Agent: "a", Action: "x"})
	if !IsCode(err, ErrForbidden) {
		t.Fatalf("want forbidden, got %+v", AsError(err))
	}
}

func TestError429WithRetryAfterSeconds(t *testing.T) {
	c := newClient(t, httpStatusHandler(429, `{"message":"slow down"}`,
		map[string]string{"Retry-After": "30"}))
	_, err := c.Evaluate(context.Background(), EvaluateRequest{Agent: "a", Action: "x"})
	e := AsError(err)
	if e == nil || e.Code != ErrRateLimited {
		t.Fatalf("want rate_limited, got %+v", e)
	}
	if e.RetryAfter != 30*time.Second {
		t.Fatalf("RetryAfter = %v, want 30s", e.RetryAfter)
	}
}

func TestError500ServerError(t *testing.T) {
	c := newClient(t, httpStatusHandler(500, "oops", nil))
	_, err := c.Evaluate(context.Background(), EvaluateRequest{Agent: "a", Action: "x"})
	if !IsCode(err, ErrServerError) {
		t.Fatalf("want server_error, got %+v", AsError(err))
	}
}

func TestError422SurfacesServerMessage(t *testing.T) {
	c := newClient(t, httpStatusHandler(422, `{"message":"bad field: agent"}`,
		map[string]string{"Content-Type": "application/json"}))
	_, err := c.Evaluate(context.Background(), EvaluateRequest{Agent: "a", Action: "x"})
	e := AsError(err)
	if e == nil || e.Code != ErrBadRequest {
		t.Fatalf("want bad_request, got %+v", e)
	}
	if e.Message != "bad field: agent" {
		t.Errorf("Message = %q, want 'bad field: agent'", e.Message)
	}
}

func TestErrorsIsInterop(t *testing.T) {
	e := &Error{Code: ErrRateLimited, Status: 429}
	// Wrapping via fmt.Errorf("%w", e) should still be found by errors.As.
	wrapped := wrap(e)
	var got *Error
	if !errors.As(wrapped, &got) {
		t.Fatal("errors.As failed to recover *Error through wrap()")
	}
	if got.Code != ErrRateLimited {
		t.Fatalf("got.Code = %q", got.Code)
	}
	if !IsCode(wrapped, ErrRateLimited) {
		t.Fatal("IsCode did not see through wrap()")
	}
}

func wrap(err error) error { return errors.Join(errors.New("context"), err) }

func TestParseRetryAfter(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"garbage", 0},
		{"0", 0},
		{"5", 5 * time.Second},
		{"30.5", 30500 * time.Millisecond},
	}
	for _, tc := range cases {
		if got := parseRetryAfter(tc.in); got != tc.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
