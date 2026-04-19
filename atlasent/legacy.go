package atlasent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// LegacyClient speaks the cross-SDK v1 wire contract (see
// atlasent-sdk/contract/vectors). It is wire-compatible with the Python and
// TypeScript SDKs. New code should use Client; LegacyClient exists so users
// migrating from the v1 HTTP surface can upgrade to Go without flipping
// servers first.
//
// Differences from Client:
//   - Endpoint: /v1-evaluate (not /v1/evaluate)
//   - Request shape: {agent, action, context, api_key} (no actor/target)
//   - Response shape: {permitted, decision_id, reason, audit_hash, timestamp}
//   - api_key is ALSO sent in the JSON body alongside the Bearer header
type LegacyClient struct {
	inner *Client
}

// NewLegacy wraps an existing Client so its connection pool, retry, cache,
// and observer are reused. Fail-closed semantics still apply at the Client
// level; LegacyClient never silently defaults.
func NewLegacy(c *Client) *LegacyClient { return &LegacyClient{inner: c} }

// LegacyEvaluateRequest is the v1 wire shape. Mirrors the SDK vectors
// exactly: keys stay in snake_case.
type LegacyEvaluateRequest struct {
	Agent   string         `json:"agent"`
	Action  string         `json:"action"`
	Context map[string]any `json:"context"`
	APIKey  string         `json:"api_key"`
}

// LegacyEvaluateResponse is the v1 wire response.
type LegacyEvaluateResponse struct {
	Permitted  bool   `json:"permitted"`
	DecisionID string `json:"decision_id"`
	Reason     string `json:"reason"`
	AuditHash  string `json:"audit_hash"`
	Timestamp  string `json:"timestamp"`
}

// LegacyEvaluateResult is the idiomatic SDK-shape returned to callers. The
// decision_id is surfaced as PermitID to match Python/TS behavior.
type LegacyEvaluateResult struct {
	Decision   string
	Permitted  bool
	PermitID   string
	Reason     string
	AuditHash  string
	Timestamp  string
}

// Evaluate calls POST /v1-evaluate with the v1 wire contract. Context is
// defaulted to {} (never nil) so the wire shape matches exactly.
func (l *LegacyClient) Evaluate(ctx context.Context, agent, action string, reqCtx map[string]any) (*LegacyEvaluateResult, error) {
	if reqCtx == nil {
		reqCtx = map[string]any{}
	}
	wire := LegacyEvaluateRequest{
		Agent:   agent,
		Action:  action,
		Context: reqCtx,
		APIKey:  l.inner.apiKey,
	}
	raw, err := l.inner.rawPostJSON(ctx, "/v1-evaluate", wire)
	if err != nil {
		return nil, err
	}
	var resp LegacyEvaluateResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("atlasent: legacy evaluate decode: %w", err)
	}
	// Per contract: malformed responses missing required fields must error
	// rather than silently default to DENY or ALLOW.
	var probe map[string]any
	_ = json.Unmarshal(raw, &probe)
	if _, ok := probe["permitted"]; !ok {
		return nil, errors.New("atlasent: legacy evaluate: response missing permitted")
	}
	if _, ok := probe["decision_id"]; !ok {
		return nil, errors.New("atlasent: legacy evaluate: response missing decision_id")
	}
	decision := "DENY"
	if resp.Permitted {
		decision = "ALLOW"
	}
	return &LegacyEvaluateResult{
		Decision:  decision,
		Permitted: resp.Permitted,
		PermitID:  resp.DecisionID,
		Reason:    resp.Reason,
		AuditHash: resp.AuditHash,
		Timestamp: resp.Timestamp,
	}, nil
}

// LegacyVerifyRequest is the v1 wire shape for permit verification.
type LegacyVerifyRequest struct {
	DecisionID string         `json:"decision_id"`
	Action     string         `json:"action"`
	Agent      string         `json:"agent"`
	Context    map[string]any `json:"context"`
	APIKey     string         `json:"api_key"`
}

// LegacyVerifyResponse is the v1 wire response for permit verification.
type LegacyVerifyResponse struct {
	Verified   bool   `json:"verified"`
	Outcome    string `json:"outcome"`
	PermitHash string `json:"permit_hash"`
	Timestamp  string `json:"timestamp"`
}

// VerifyPermit calls POST /v1-verify-permit. When the caller omits
// action/agent/context, they are sent as "" / "" / {} — matching the other
// SDKs' default behavior.
func (l *LegacyClient) VerifyPermit(ctx context.Context, permitID, action, agent string, reqCtx map[string]any) (*LegacyVerifyResponse, error) {
	if reqCtx == nil {
		reqCtx = map[string]any{}
	}
	wire := LegacyVerifyRequest{
		DecisionID: permitID,
		Action:     action,
		Agent:      agent,
		Context:    reqCtx,
		APIKey:     l.inner.apiKey,
	}
	raw, err := l.inner.rawPostJSON(ctx, "/v1-verify-permit", wire)
	if err != nil {
		return nil, err
	}
	var resp LegacyVerifyResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("atlasent: legacy verify decode: %w", err)
	}
	var probe map[string]any
	_ = json.Unmarshal(raw, &probe)
	if _, ok := probe["verified"]; !ok {
		return nil, errors.New("atlasent: legacy verify: response missing verified")
	}
	return &resp, nil
}

// rawPostJSON is a single-attempt POST returning the raw response body.
// Used by LegacyClient so it can do contract-level decoding rather than
// re-reading the body through generic unmarshal.
func (c *Client) rawPostJSON(ctx context.Context, path string, body any) ([]byte, error) {
	var (
		raw []byte
		err error
	)
	// Reuse doJSON's retry/backoff machinery by routing through a small
	// intermediate decoder that captures the raw bytes.
	capture := &rawCapture{}
	_, err = c.doJSON(ctx, http.MethodPost, path, body, capture)
	if err != nil {
		return nil, err
	}
	raw = capture.bytes
	if raw == nil {
		// doJSON unmarshals nothing when out is nil-or-empty; fall back to
		// a simple decode into json.RawMessage so tests that return empty
		// bodies still surface cleanly.
		raw = []byte("{}")
	}
	return raw, nil
}

// rawCapture implements json.Unmarshaler so doJSON hands us the raw body.
type rawCapture struct {
	bytes []byte
}

// UnmarshalJSON captures the raw bytes exactly as received.
func (r *rawCapture) UnmarshalJSON(b []byte) error {
	r.bytes = append(r.bytes[:0], b...)
	return nil
}

// compile-time assertion that rawCapture implements json.Unmarshaler.
var _ json.Unmarshaler = (*rawCapture)(nil)

// silence unused imports for the time type referenced in docs.
var _ = time.Second
