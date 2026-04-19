package atlasent

import (
	"context"
	"errors"
	"testing"
)

func TestObligationRegistry_Apply(t *testing.T) {
	r := NewObligationRegistry()
	var redacted, logged bool
	r.Register("redact:ssn", func(_ context.Context, _ string) error { redacted = true; return nil })
	r.Register("log:high-risk", func(_ context.Context, _ string) error { logged = true; return nil })

	d := Decision{Allowed: true, Obligations: []string{"redact:ssn", "log:high-risk"}}
	if err := r.Apply(context.Background(), d); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !redacted || !logged {
		t.Fatalf("handler not invoked: redacted=%v logged=%v", redacted, logged)
	}
}

func TestObligationRegistry_UnknownFailsByDefault(t *testing.T) {
	r := NewObligationRegistry()
	d := Decision{Allowed: true, Obligations: []string{"unknown"}}
	if err := r.Apply(context.Background(), d); err == nil {
		t.Fatal("want error for unknown obligation")
	}
}

func TestObligationRegistry_AllowUnknown(t *testing.T) {
	r := NewObligationRegistry()
	r.AllowUnknown(true)
	d := Decision{Allowed: true, Obligations: []string{"unknown"}}
	if err := r.Apply(context.Background(), d); err != nil {
		t.Fatalf("Apply with AllowUnknown: %v", err)
	}
}

func TestObligationRegistry_HandlerError(t *testing.T) {
	r := NewObligationRegistry()
	boom := errors.New("boom")
	r.Register("crash", func(_ context.Context, _ string) error { return boom })
	err := r.Apply(context.Background(), Decision{Obligations: []string{"crash"}})
	if !errors.Is(err, boom) {
		t.Fatalf("want wrapped boom, got %v", err)
	}
}
