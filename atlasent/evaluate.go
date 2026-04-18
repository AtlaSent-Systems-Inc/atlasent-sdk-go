package atlasent

import (
	"context"
	"encoding/json"
	"fmt"
)

// EvaluateRequest is the input to Client.Evaluate. Matches the wire
// contract of @atlasent/sdk (TypeScript) and atlasent (Python):
//
//   - Agent is the calling agent identifier (e.g. "clinical-data-agent").
//   - Action is the action being authorized (e.g. "modify_patient_record").
//   - Context is an arbitrary policy context (user, environment, resource IDs).
type EvaluateRequest struct {
	Agent   string         `json:"-"`
	Action  string         `json:"-"`
	Context map[string]any `json:"-"`
}

// evaluateWire is the on-the-wire serialization of EvaluateRequest.
// The server's current field names are `actor_id` / `action_type`;
// `api_key` is included for backward compat with body-based auth.
type evaluateWire struct {
	ActionType string         `json:"action_type"`
	ActorID    string         `json:"actor_id"`
	Context    map[string]any `json:"context"`
	APIKey     string         `json:"api_key,omitempty"`
}

// EvaluateResponse is the SDK-canonical result from POST /v1-evaluate.
// Fields mirror the TypeScript and Python SDK response types.
type EvaluateResponse struct {
	// Permitted is the enforcement bit — true iff the action is allowed.
	// A false value is returned as data, not as an error.
	Permitted bool `json:"permitted"`
	// DecisionID is the opaque permit identifier, passed to VerifyPermit.
	DecisionID string `json:"decision_id"`
	// Reason is the human-readable policy explanation.
	Reason string `json:"reason"`
	// AuditHash is the hash-chained audit-trail entry.
	AuditHash string `json:"audit_hash"`
	// Timestamp is the ISO 8601 timestamp of the decision.
	Timestamp string `json:"timestamp"`
}

// Evaluate asks the AtlaSent policy engine whether an agent action is
// permitted. A clean DENY is returned as EvaluateResponse{Permitted: false};
// network, 4xx, 5xx, and malformed-response failures are returned as *Error.
func (c *Client) Evaluate(ctx context.Context, req EvaluateRequest) (EvaluateResponse, error) {
	wire := evaluateWire{
		ActionType: req.Action,
		ActorID:    req.Agent,
		Context:    req.Context,
		APIKey:     c.apiKey,
	}
	if wire.Context == nil {
		wire.Context = map[string]any{}
	}
	var raw map[string]any
	if err := c.postJSON(ctx, "/v1-evaluate", wire, &raw); err != nil {
		return EvaluateResponse{}, err
	}
	normalized := normalizeEvaluateWire(raw)
	var resp EvaluateResponse
	b, _ := json.Marshal(normalized)
	if err := json.Unmarshal(b, &resp); err != nil {
		return EvaluateResponse{}, &Error{
			Code:    ErrBadResponse,
			Message: fmt.Sprintf("decoding /v1-evaluate response: %v", err),
			Cause:   err,
		}
	}
	// `permitted` and `decision_id` must be present — otherwise the
	// server contract is broken.
	if _, ok := normalized["permitted"].(bool); !ok {
		return EvaluateResponse{}, &Error{
			Code:    ErrBadResponse,
			Message: "Malformed /v1-evaluate response: missing `permitted`",
		}
	}
	if _, ok := normalized["decision_id"].(string); !ok {
		return EvaluateResponse{}, &Error{
			Code:    ErrBadResponse,
			Message: "Malformed /v1-evaluate response: missing `decision_id`",
		}
	}
	return resp, nil
}

// normalizeEvaluateWire handles servers that emit only native fields
// (decision: "allow", permit_token, audit_entry_hash, request_id) by
// deriving the SDK-canonical keys. Canonical keys win over derived ones.
//
// Mirrors atlasent._response_adapter.normalize_evaluate_response in the
// Python SDK.
func normalizeEvaluateWire(data map[string]any) map[string]any {
	if data == nil {
		return data
	}
	out := make(map[string]any, len(data)+4)
	for k, v := range data {
		out[k] = v
	}
	if _, ok := out["permitted"]; !ok {
		if d, ok := out["decision"].(string); ok {
			out["permitted"] = d == "allow"
		}
	}
	if _, ok := out["decision_id"]; !ok {
		if t, ok := out["permit_token"].(string); ok && t != "" {
			out["decision_id"] = t
		} else if r, ok := out["request_id"].(string); ok && r != "" {
			out["decision_id"] = r
		}
	}
	if _, ok := out["audit_hash"]; !ok {
		if h, ok := out["audit_entry_hash"].(string); ok {
			out["audit_hash"] = h
		}
	}
	if _, ok := out["reason"]; !ok {
		if r, ok := out["deny_reason"].(string); ok {
			out["reason"] = r
		}
	}
	return out
}
