package atlasent

import (
	"context"
	"fmt"
	"time"
)

// batchRequest / batchResponse are the wire types for POST /v1/authorize/batch.
// Exposed unexported so tests in the same package can mirror the server shape.
type batchRequest struct {
	Checks []CheckRequest `json:"checks"`
}

type batchResponse struct {
	Decisions []Decision `json:"decisions"`
}

// CheckMany asks the PDP about many requests in a single round trip. Results
// are returned in input order. Cached entries are served locally; only
// uncached requests hit the wire. On transport failure, every uncached slot
// gets a fail-closed deny (or fail-open allow, per Client.FailClosed) and
// the error is returned alongside the slice so callers can still iterate.
func (c *Client) CheckMany(ctx context.Context, reqs []CheckRequest) ([]Decision, error) {
	decisions := make([]Decision, len(reqs))

	type pending struct {
		idx int
		req CheckRequest
	}
	var todo []pending
	for i, r := range reqs {
		if c.cache != nil {
			if dec, ok := c.cache.Get(cacheKey(r)); ok {
				decisions[i] = dec
				c.observe(ctx, CheckEvent{Request: r, Decision: dec, CacheHit: true})
				continue
			}
		}
		todo = append(todo, pending{idx: i, req: r})
	}
	if len(todo) == 0 {
		return decisions, nil
	}

	wireReqs := make([]CheckRequest, len(todo))
	for i, p := range todo {
		wireReqs[i] = p.req
	}
	start := time.Now()
	var resp batchResponse
	attempts, err := c.postJSON(ctx, "/v1/authorize/batch", batchRequest{Checks: wireReqs}, &resp)
	latency := time.Since(start)

	if err != nil {
		for _, p := range todo {
			fallback := Decision{Allowed: !c.FailClosed, Reason: "pdp unavailable (batch)"}
			decisions[p.idx] = fallback
			c.observe(ctx, CheckEvent{
				Request:  p.req,
				Decision: fallback,
				Err:      err,
				Latency:  latency,
				Attempts: attempts,
			})
		}
		return decisions, err
	}

	if len(resp.Decisions) != len(todo) {
		return decisions, fmt.Errorf("atlasent: batch response size mismatch: want %d, got %d", len(todo), len(resp.Decisions))
	}

	for i, p := range todo {
		dec := resp.Decisions[i]
		decisions[p.idx] = dec
		if c.cache != nil {
			if ttl := c.cacheTTL(dec); ttl > 0 {
				c.cache.Set(cacheKey(p.req), dec, ttl)
			}
		}
		c.observe(ctx, CheckEvent{
			Request:  p.req,
			Decision: dec,
			Latency:  latency,
			Attempts: attempts,
		})
	}
	return decisions, nil
}
