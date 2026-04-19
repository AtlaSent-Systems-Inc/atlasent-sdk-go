package atlasent

import (
	"context"
	"fmt"
	"time"
)

// OverrideStatus is the lifecycle state of an override request.
type OverrideStatus string

const (
	OverridePending  OverrideStatus = "pending"
	OverrideApproved OverrideStatus = "approved"
	OverrideRejected OverrideStatus = "rejected"
)

// Override is a request to override a DENY decision. Created by the action
// owner, resolved by an approver; fully audited.
type Override struct {
	ID            string         `json:"id"`
	OrgID         string         `json:"org_id"`
	ActionID      string         `json:"action_id"`
	TargetID      string         `json:"target_id,omitempty"`
	Justification string         `json:"justification"`
	Status        OverrideStatus `json:"status"`
	Reason        string         `json:"reason,omitempty"`
	Context       map[string]any `json:"context,omitempty"`
	RequestedBy   string         `json:"requested_by"`
	RequestedAt   time.Time      `json:"requested_at"`
	ResolvedBy    string         `json:"resolved_by,omitempty"`
	ResolvedAt    *time.Time     `json:"resolved_at,omitempty"`
}

// OverrideRequest is the payload for RequestOverride.
type OverrideRequest struct {
	ActionID      string         `json:"action_id"`
	TargetID      string         `json:"target_id,omitempty"`
	Justification string         `json:"justification"`
	Context       map[string]any `json:"context,omitempty"`
}

// RequestOverride calls POST /v1/overrides to open a DENY-override ticket.
func (c *Client) RequestOverride(ctx context.Context, req OverrideRequest) (*Override, error) {
	if req.ActionID == "" {
		return nil, fmt.Errorf("atlasent: RequestOverride: action_id is required")
	}
	if req.Justification == "" {
		return nil, fmt.Errorf("atlasent: RequestOverride: justification is required")
	}
	var out Override
	if _, err := c.postJSON(ctx, "/v1/overrides", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ResolveOverride calls POST /v1/overrides/{id}/resolve. decision must be
// OverrideApproved or OverrideRejected.
func (c *Client) ResolveOverride(ctx context.Context, id string, decision OverrideStatus, reason string) (*Override, error) {
	if id == "" {
		return nil, fmt.Errorf("atlasent: ResolveOverride: id is required")
	}
	if decision != OverrideApproved && decision != OverrideRejected {
		return nil, fmt.Errorf("atlasent: ResolveOverride: decision must be approved or rejected, got %q", decision)
	}
	body := map[string]string{
		"decision": string(decision),
		"reason":   reason,
	}
	var out Override
	if _, err := c.postJSON(ctx, "/v1/overrides/"+id+"/resolve", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
