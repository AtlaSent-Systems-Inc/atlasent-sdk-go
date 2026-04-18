package atlasent

import (
	"context"
	"encoding/json"
	"fmt"
)

// VerifyPermitRequest is the input to Client.VerifyPermit. PermitID is
// the DecisionID returned by a prior Evaluate call.
type VerifyPermitRequest struct {
	PermitID string         `json:"-"`
	Agent    string         `json:"-"`
	Action   string         `json:"-"`
	Context  map[string]any `json:"-"`
}

// verifyWire is the on-the-wire serialization. Server's current shape
// uses `decision_id` as the permit token field name.
type verifyWire struct {
	DecisionID string         `json:"decision_id"`
	ActionType string         `json:"action_type,omitempty"`
	ActorID    string         `json:"actor_id,omitempty"`
	Context    map[string]any `json:"context,omitempty"`
	APIKey     string         `json:"api_key,omitempty"`
	PermitToken string         `json:"permit_token,omitempty"`
}

// VerifyPermitResponse is the SDK-canonical result from
// POST /v1-verify-permit.
type VerifyPermitResponse struct {
	Verified   bool   `json:"verified"`
	Outcome    string `json:"outcome"`
	PermitHash string `json:"permit_hash"`
	Timestamp  string `json:"timestamp"`
}

// VerifyPermit confirms that a previously issued permit is still valid.
// A verified==false response is returned as data, not as an error.
func (c *Client) VerifyPermit(ctx context.Context, req VerifyPermitRequest) (VerifyPermitResponse, error) {
	wire := verifyWire{
		DecisionID:  req.PermitID,
		ActionType:  req.Action,
		ActorID:     req.Agent,
		Context:     req.Context,
		APIKey:      c.apiKey,
		PermitToken: req.PermitID, // server accepts both keys during rollout
	}
	if wire.Context == nil {
		wire.Context = map[string]any{}
	}
	var raw map[string]any
	if err := c.postJSON(ctx, "/v1-verify-permit", wire, &raw); err != nil {
		return VerifyPermitResponse{}, err
	}
	normalized := normalizeVerifyWire(raw)
	var resp VerifyPermitResponse
	b, _ := json.Marshal(normalized)
	if err := json.Unmarshal(b, &resp); err != nil {
		return VerifyPermitResponse{}, &Error{
			Code:    ErrBadResponse,
			Message: fmt.Sprintf("decoding /v1-verify-permit response: %v", err),
			Cause:   err,
		}
	}
	if _, ok := normalized["verified"].(bool); !ok {
		return VerifyPermitResponse{}, &Error{
			Code:    ErrBadResponse,
			Message: "Malformed /v1-verify-permit response: missing `verified`",
		}
	}
	return resp, nil
}

// normalizeVerifyWire handles legacy servers that send only native
// fields (valid, outcome, decision) by deriving the SDK-canonical
// keys. Canonical keys win over derived ones.
func normalizeVerifyWire(data map[string]any) map[string]any {
	if data == nil {
		return data
	}
	out := make(map[string]any, len(data)+2)
	for k, v := range data {
		out[k] = v
	}
	if _, ok := out["verified"]; !ok {
		if v, ok := out["valid"].(bool); ok {
			out["verified"] = v
		}
	}
	if _, ok := out["permit_hash"]; !ok {
		out["permit_hash"] = ""
	}
	return out
}
