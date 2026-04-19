package atlasent

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// ApprovalStatus is the lifecycle state of an approval request.
type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"
	ApprovalExpired  ApprovalStatus = "expired"
)

// Approval is a human-in-the-loop approval request opened when an
// evaluation returns outcome=require_approval.
type Approval struct {
	ID           string         `json:"id"`
	OrgID        string         `json:"org_id"`
	ActorID      string         `json:"actor_id"`
	ActionID     string         `json:"action_id,omitempty"`
	TargetID     string         `json:"target_id,omitempty"`
	Status       ApprovalStatus `json:"status"`
	Justification string        `json:"justification,omitempty"`
	Reason       string         `json:"reason,omitempty"`
	Context      map[string]any `json:"context,omitempty"`
	RequestedAt  time.Time      `json:"requested_at"`
	ResolvedAt   *time.Time     `json:"resolved_at,omitempty"`
	ResolvedBy   string         `json:"resolved_by,omitempty"`
}

// ApprovalRequest is the payload for CreateApproval.
type ApprovalRequest struct {
	ActionID      string         `json:"action_id"`
	TargetID      string         `json:"target_id,omitempty"`
	Justification string         `json:"justification"`
	Context       map[string]any `json:"context,omitempty"`
}

// ListApprovals calls GET /v1/approvals. status "" returns all statuses.
func (c *Client) ListApprovals(ctx context.Context, status ApprovalStatus) ([]Approval, error) {
	path := "/v1/approvals"
	if status != "" {
		q := url.Values{"status": {string(status)}}
		path += "?" + q.Encode()
	}
	var out struct {
		Approvals []Approval `json:"approvals"`
	}
	if _, err := c.getJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	return out.Approvals, nil
}

// CreateApproval calls POST /v1/approvals and opens an approval request.
func (c *Client) CreateApproval(ctx context.Context, req ApprovalRequest) (*Approval, error) {
	if req.ActionID == "" {
		return nil, fmt.Errorf("atlasent: CreateApproval: action_id is required")
	}
	if req.Justification == "" {
		return nil, fmt.Errorf("atlasent: CreateApproval: justification is required")
	}
	var out Approval
	if _, err := c.postJSON(ctx, "/v1/approvals", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ResolveApproval calls POST /v1/approvals/{id}/resolve to approve or
// reject a pending request. decision must be ApprovalApproved or
// ApprovalRejected.
func (c *Client) ResolveApproval(ctx context.Context, id string, decision ApprovalStatus, reason string) (*Approval, error) {
	if id == "" {
		return nil, fmt.Errorf("atlasent: ResolveApproval: id is required")
	}
	if decision != ApprovalApproved && decision != ApprovalRejected {
		return nil, fmt.Errorf("atlasent: ResolveApproval: decision must be approved or rejected, got %q", decision)
	}
	body := map[string]string{
		"decision": string(decision),
		"reason":   reason,
	}
	var out Approval
	if _, err := c.postJSON(ctx, "/v1/approvals/"+id+"/resolve", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
