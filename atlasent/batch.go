package atlasent

import (
	"context"
	"fmt"
	"time"
)

// batchRequest is the wire shape for POST /v1/authorize/batch.
type batchRequest struct {
	Checks []CheckRequest `json:"checks"`
}

type batchResponse struct {
	Decisions []Decision `json:"decisions"`
}

// CheckMany asks the PDP about N requests in a single round trip and returns
// the decisions in the same order. It is the preferred shape for list
// endpoints ("which of these 200 invoices can this user read?") where
// issuing N individual Checks would serialize the page render on N RTTs.
//
// Cache lookups and writes are honored per request. Any request with a
// cached decision is filled locally; the rest are sent in one batch.
//
// On transport error CheckMany returns the error and a slice of fail-closed
// denies (or fail-open allows, per Client.FailClosed). The returned slice is
// always len(reqs) so callers can index into it unconditionally.
func (c *Client) CheckMany(ctx context.Context, reqs []CheckRequest) ([]Decision, error) {
	out := make([]Decision, len(reqs))
	if len(reqs) == 0 {
		return out, nil
	}

	pending := make([]int, 0, len(reqs))
	pendingReqs := make([]CheckRequest, 0, len(reqs))

	if c.cache != nil {
		for i, r := range reqs {
			if dec, ok := c.cache.Get(cacheKey(r)); ok {
				out[i] = dec
				c.observe(ctx, CheckEvent{Request: r, Decision: dec, CacheHit: true})
				continue
			}
			pending = append(pending, i)
			pendingReqs = append(pendingReqs, r)
		}
	} else {
		for i, r := range reqs {
			pending = append(pending, i)
			pendingReqs = append(pendingReqs, r)
		}
		_ = reqs
	}

	if len(pendingReqs) == 0 {
		return out, nil
	}

	start := time.Now()
	var resp batchResponse
	attempts, err := c.postJSON(ctx, "/v1/authorize/batch", batchRequest{Checks: pendingReqs}, &resp)
	latency := time.Since(start)

	if err != nil {
		fallback := Decision{Allowed: false, Reason: "pdp unavailable (fail-closed)"}
		if !c.FailClosed {
			fallback = Decision{Allowed: true, Reason: "pdp unavailable (fail-open)"}
		}
		for k, i := range pending {
			out[i] = fallback
			c.observe(ctx, CheckEvent{
				Request:  pendingReqs[k],
				Decision: fallback,
				Err:      err,
				Latency:  latency,
				Attempts: attempts,
			})
		}
		return out, err
	}
	if len(resp.Decisions) != len(pendingReqs) {
		return out, fmt.Errorf("atlasent: batch size mismatch: sent %d, got %d",
			len(pendingReqs), len(resp.Decisions))
	}

	for k, i := range pending {
		d := resp.Decisions[k]
		out[i] = d
		if c.cache != nil {
			if ttl := c.cacheTTL(d); ttl > 0 {
				c.cache.Set(cacheKey(pendingReqs[k]), d, ttl)
			}
		}
		c.observe(ctx, CheckEvent{
			Request:  pendingReqs[k],
			Decision: d,
			Latency:  latency,
			Attempts: attempts,
		})
	}
	return out, nil
}
