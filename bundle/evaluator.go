package bundle

import (
	"context"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
)

// Evaluator decides authorization questions locally when possible.
// ok=true means "I have an opinion"; the returned Decision is used and
// the remote PDP is not consulted. ok=false means "not covered, ask
// upstream". err is reserved for engine failures (parse errors,
// malformed input); a safe response is ok=false, zero Decision.
type Evaluator interface {
	Evaluate(ctx context.Context, req atlasent.CheckRequest) (atlasent.Decision, bool, error)
}

// PolicyEngine is the in-memory decision function over a single bundle
// payload. One payload lives for the duration the bundle is current;
// when the Syncer rotates, the Manager swaps in a new engine state
// atomically.
//
// Implementations are engine-specific (Cedar, Rego, CEL, hand-rolled).
// This package ships no default — picking a policy language is a
// product decision, not a library decision.
type PolicyEngine interface {
	// Load parses and compiles raw bundle bytes. Called on every
	// successful sync. A non-nil error leaves the previous bundle
	// active.
	Load(payload []byte) (EngineState, error)
}

// EngineState is a compiled, ready-to-evaluate snapshot of one bundle.
// Exactly one of these is "current" at any time. Evaluate is called
// on the hot path — it must be concurrent-safe and allocation-light.
type EngineState interface {
	Evaluate(ctx context.Context, req atlasent.CheckRequest) (atlasent.Decision, bool, error)
}
