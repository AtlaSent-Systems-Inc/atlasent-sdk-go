package atlasentotel_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
	atlasentotel "github.com/atlasent-systems-inc/atlasent-sdk-go/otel"
)

// TestNewObserverNilOK proves the observer constructor tolerates nil meter
// and tracer — a common case when the app opts in to only one signal.
func TestNewObserverNilOK(t *testing.T) {
	o, err := atlasentotel.NewObserver(nil, nil)
	if err != nil {
		t.Fatalf("NewObserver: %v", err)
	}
	// OnCheck with no wired instruments must be a no-op, not a panic.
	o.OnCheck(context.Background(), atlasent.CheckEvent{
		Request:  atlasent.CheckRequest{Action: "read", Resource: atlasent.Resource{Type: "doc"}},
		Decision: atlasent.Decision{Allowed: true},
		Latency:  5 * time.Millisecond,
	})
	o.OnCheck(context.Background(), atlasent.CheckEvent{
		Request:  atlasent.CheckRequest{Action: "pay"},
		Decision: atlasent.Decision{Allowed: false},
		Err:      errors.New("boom"),
		Latency:  time.Millisecond,
	})
}
