package atlasent

import (
	"context"
	"sync"
)

// BatchResult holds the outcome of a single evaluation in an AuthorizeMany call.
type BatchResult struct {
	Payload EvaluationPayload
	Result  *EvaluationResult
	Err     error
}

// AuthorizeMany evaluates multiple payloads concurrently and returns results in order.
func (c *Client) AuthorizeMany(ctx context.Context, payloads []EvaluationPayload) []BatchResult {
	results := make([]BatchResult, len(payloads))
	var wg sync.WaitGroup
	wg.Add(len(payloads))
	for i, p := range payloads {
		i, p := i, p
		go func() {
			defer wg.Done()
			r, err := c.Evaluate(ctx, p)
			results[i] = BatchResult{Payload: p, Result: r, Err: err}
		}()
	}
	wg.Wait()
	return results
}
