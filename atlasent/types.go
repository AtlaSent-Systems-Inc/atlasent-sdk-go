package atlasent

import "time"

// RiskLevel represents the severity of a risk assessment.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// Outcome is the coarse verdict on an evaluation. It mirrors the OpenAPI
// Decision.outcome enum. Most callers should use Client.Check and the richer
// Decision type; Outcome is exposed for surfaces that only need allow/deny.
type Outcome string

const (
	OutcomeAllow           Outcome = "allow"
	OutcomeDeny            Outcome = "deny"
	OutcomeRequireApproval Outcome = "require_approval"
)

// PermitStatus is the lifecycle state of a permit. Values match the
// AtlaSent API permit status enum.
type PermitStatus string

const (
	PermitIssued   PermitStatus = "issued"
	PermitVerified PermitStatus = "verified"
	PermitConsumed PermitStatus = "consumed"
	PermitExpired  PermitStatus = "expired"
	PermitRevoked  PermitStatus = "revoked"
)

// Actor describes the entity performing an action.
type Actor struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Email      string         `json:"email,omitempty"`
	Name       string         `json:"name,omitempty"`
	OrgID      string         `json:"org_id,omitempty"`
	Roles      []string       `json:"roles,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// Action describes what is being done.
type Action struct {
	ID          string         `json:"id"`
	Type        string         `json:"type,omitempty"`
	Category    string         `json:"category,omitempty"`
	Description string         `json:"description,omitempty"`
	IsBulk      bool           `json:"isBulk,omitempty"`
	BulkCount   int            `json:"bulkCount,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Target describes what is being acted upon.
type Target struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Sensitivity string         `json:"sensitivity,omitempty"`
	Environment string         `json:"environment,omitempty"`
	Attributes  map[string]any `json:"attributes,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// RiskAssessment holds the risk score and contributing factors.
type RiskAssessment struct {
	Score       int       `json:"score"`
	Level       RiskLevel `json:"level"`
	Reasons     []string  `json:"reasons,omitempty"`
	Factors     []string  `json:"factors,omitempty"`
	Mitigations []string  `json:"mitigations,omitempty"`
}

// EvaluationPayload is sent to POST /v1/evaluate.
type EvaluationPayload struct {
	Actor       Actor          `json:"actor"`
	Action      Action         `json:"action"`
	Target      Target         `json:"target"`
	Context     map[string]any `json:"context,omitempty"`
	Environment string         `json:"environment,omitempty"`
	RequestID   string         `json:"request_id,omitempty"`
}

// EvaluationResult is returned from POST /v1/evaluate. It is the OpenAPI
// Decision-aligned response shape. Most callers should use Client.Check and
// the richer Decision type.
type EvaluationResult struct {
	ID               string         `json:"id"`
	EvaluationID     string         `json:"evaluation_id,omitempty"`
	Outcome          Outcome        `json:"outcome"`
	Risk             RiskAssessment `json:"risk"`
	MatchedRuleID    string         `json:"matched_rule_id,omitempty"`
	PermitID         string         `json:"permit_id,omitempty"`
	RequiresApproval bool           `json:"requires_approval,omitempty"`
	EvaluatedAt      time.Time      `json:"evaluated_at"`
	PolicyID         string         `json:"policy_id,omitempty"`
	PolicyVersion    string         `json:"policy_version,omitempty"`
}

// Allowed reports whether the outcome is allow.
func (r *EvaluationResult) Allowed() bool { return r.Outcome == OutcomeAllow }

// Permit represents an issued authorization token.
type Permit struct {
	ID           string       `json:"id"`
	EvaluationID string       `json:"evaluation_id,omitempty"`
	OrgID        string       `json:"org_id,omitempty"`
	ActorID      string       `json:"actor_id,omitempty"`
	ActionID     string       `json:"action_id,omitempty"`
	TargetID     string       `json:"target_id,omitempty"`
	Environment  string       `json:"environment,omitempty"`
	Status       PermitStatus `json:"status"`
	IssuedAt     time.Time    `json:"issued_at"`
	ExpiresAt    time.Time    `json:"expires_at"`
	ConsumedAt   *time.Time   `json:"consumed_at,omitempty"`
	Signature    string       `json:"signature,omitempty"`
}

// Session holds the authenticated caller's identity and server-issued
// permissions for the current request.
type Session struct {
	User        SessionUser `json:"user"`
	Org         OrgSummary  `json:"org"`
	Permissions []string    `json:"permissions"`
	IssuedAt    time.Time   `json:"issued_at"`
	ExpiresAt   time.Time   `json:"expires_at"`
}

// SessionUser is the user portion of a Session.
type SessionUser struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// OrgSummary is a compact view of an organization.
type OrgSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	Plan string `json:"plan"`
}

// APIError is returned by the AtlaSent API on failure. RetryAfter is parsed
// from the Retry-After header when present.
type APIError struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Details    map[string]any `json:"details,omitempty"`
	Status     int            `json:"-"`
	RetryAfter time.Duration  `json:"-"`
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return e.Message
}
