package atlasent

import "context"

// VerifyPermit calls POST /v1/permits/{id}/verify and returns the permit's
// current status. A 410 indicates the permit was consumed or has expired;
// that surfaces as an *APIError.
func (c *Client) VerifyPermit(ctx context.Context, permitID string) (*Permit, error) {
	var p Permit
	if _, err := c.postJSON(ctx, "/v1/permits/"+permitID+"/verify", nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ConsumePermit calls POST /v1/permits/{id}/consume. Single-use: a second
// call for the same permit id returns a 409 as *APIError.
func (c *Client) ConsumePermit(ctx context.Context, permitID string) (*Permit, error) {
	var p Permit
	if _, err := c.postJSON(ctx, "/v1/permits/"+permitID+"/consume", nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// RevokePermit calls POST /v1/permits/{id}/revoke to invalidate an unused
// permit. Idempotent for already-revoked permits.
func (c *Client) RevokePermit(ctx context.Context, permitID string) (*Permit, error) {
	var p Permit
	if _, err := c.postJSON(ctx, "/v1/permits/"+permitID+"/revoke", nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
