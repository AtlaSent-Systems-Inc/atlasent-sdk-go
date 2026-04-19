package atlasent

import "context"

// LocalEvaluator is a policy decision source the Client consults BEFORE
// calling the PDP. Returning ok=true causes Client.Check to use the
// returned Decision directly and skip the network call; ok=false falls
// through to the normal remote flow.
//
// The canonical implementation is a bundle-sync evaluator that
// periodically pulls signed policy snapshots from the PDP and evaluates
// them locally — see github.com/atlasent-systems-inc/atlasent-sdk-go/bundle.
// Implementations must be safe for concurrent use and should return
// quickly; any meaningful latency here defeats the purpose.
type LocalEvaluator interface {
	Evaluate(ctx context.Context, req CheckRequest) (Decision, bool, error)
}

// WithLocalEvaluator installs e on the Client. When configured, every
// Check consults e first; if e answers (ok=true) the Decision is used
// as-is and no HTTP call is made. An error from e is logged through the
// Observer and the Client falls through to remote — never fails closed
// on a local-eval bug.
func WithLocalEvaluator(e LocalEvaluator) Option {
	return func(c *Client) { c.local = e }
}
