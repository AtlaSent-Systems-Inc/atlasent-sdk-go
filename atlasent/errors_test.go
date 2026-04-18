package atlasent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClassifyHTTP(t *testing.T) {
	cases := map[int]ErrorKind{
		401: KindUnauthorized,
		403: KindForbidden,
		429: KindRateLimit,
		400: KindInvalid,
		404: KindInvalid,
		500: KindServer,
		503: KindServer,
	}
	for status, want := range cases {
		if got := classifyHTTP(status); got != want {
			t.Errorf("classifyHTTP(%d) = %v, want %v", status, got, want)
		}
	}
}

func TestCheckReturnsTypedError(t *testing.T) {
	cases := []struct {
		status int
		want   ErrorKind
		pred   func(error) bool
	}{
		{401, KindUnauthorized, IsUnauthorized},
		{403, KindForbidden, IsForbidden},
		{429, KindRateLimit, IsRateLimit},
		{400, KindInvalid, IsInvalid},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.want.String(), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte("nope"))
			}))
			defer srv.Close()

			c, _ := New("k", WithBaseURL(srv.URL))
			_, err := c.Check(context.Background(), CheckRequest{
				Principal: Principal{ID: "u"},
				Action:    "x",
				Resource:  Resource{ID: "r", Type: "doc"},
			})
			if err == nil {
				t.Fatal("expected error")
			}
			if !tc.pred(err) {
				t.Fatalf("predicate failed for %v, got err=%v", tc.want, err)
			}
		})
	}
}

func TestTransportErrorTyped(t *testing.T) {
	c, _ := New("k", WithBaseURL("http://127.0.0.1:1"))
	_, err := c.Check(context.Background(), CheckRequest{
		Principal: Principal{ID: "u"},
		Action:    "x",
		Resource:  Resource{ID: "r", Type: "doc"},
	})
	if !IsTransport(err) {
		t.Fatalf("want IsTransport, got %v", err)
	}
}

func TestValidationEmptyAction(t *testing.T) {
	c, _ := New("k")
	_, err := c.Check(context.Background(), CheckRequest{
		Principal: Principal{ID: "u"},
		Resource:  Resource{Type: "doc"},
	})
	if !IsValidation(err) {
		t.Fatalf("want IsValidation, got %v", err)
	}
}
