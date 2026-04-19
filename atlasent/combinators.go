package atlasent

import "context"

// CheckAny returns the index of the first request the PDP allows, along
// with its Decision. When no request is allowed it returns -1, the last
// Decision, and a *DeniedError (or the transport error if one occurred).
// Uses a single CheckMany round trip.
//
// Use this when your code has multiple ways to satisfy a capability and
// you just need to know which one (e.g. "can alice use her personal API
// key OR her team key?").
func (c *Client) CheckAny(ctx context.Context, reqs []CheckRequest) (int, Decision, error) {
	decs, err := c.CheckMany(ctx, reqs)
	if err != nil {
		// On fail-closed CheckMany returns all-denies; fall through.
		if c.FailClosed {
			return -1, Decision{}, err
		}
	}
	for i, d := range decs {
		if d.Allowed {
			return i, d, nil
		}
	}
	last := Decision{}
	if n := len(decs); n > 0 {
		last = decs[n-1]
	}
	if err != nil {
		return -1, last, err
	}
	return -1, last, &DeniedError{Decision: last}
}

// CheckAll succeeds only when the PDP allows every request. On the first
// denial it returns the index + Decision + *DeniedError; transport errors
// short-circuit the same way. Uses a single CheckMany round trip.
//
// Use this when an action needs multiple capabilities simultaneously
// (e.g. "read invoice" AND "read customer" to render one page).
func (c *Client) CheckAll(ctx context.Context, reqs []CheckRequest) ([]Decision, error) {
	decs, err := c.CheckMany(ctx, reqs)
	if err != nil {
		return decs, err
	}
	for _, d := range decs {
		if !d.Allowed {
			return decs, &DeniedError{Decision: d}
		}
	}
	return decs, nil
}
