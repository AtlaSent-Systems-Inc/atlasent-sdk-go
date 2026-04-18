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

// Decision is the outcome of an evaluation.
type Decision string

const (
	DecisionAllow           Decision = "allow"
	DecisionDeny            Decision = "deny"
	DecisionRequireApproval Decision = "require_approval"
)

// PermitStatus is the lifecycle state of a permit.
type PermitStatus string

const (
	PermitActive   PermitStatus = "active"
	PermitConsumed PermitStatus = "consumed"
	PermitExpired  PermitStatus = "expired"
	PermitRevoked  PermitStatus = "revoked"
)

// Actor describes the entity performing an action.
type Actor struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"` // user | service | agent | system
	Email    string            `json:"email,omitempty"`
	Name     string            `json:"name,omitempty"`
	OrgID    string            `json:"orgId,omitempty"`
	Metadata map[string]any    `json:"metadata,omitempty"`
}

// Action describes what is being done.
type Action struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Description string         `json:"description,omitempty"`
	IsBulk      bool           `json:"isBulk,omitempty"`
	BulkCount   int            `json:"bulkCount,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Target describes what is being acted upon.
type Target struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Sensitivity string         `json:"sensitivity,omitempty"` // public | internal | confidential | restricted
	Environment string         `json:"environment,omitempty"` // production | staging | development
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// RiskAssessment holds the risk score and contributing factors.
type RiskAssessment struct {
	Score       int      `json:"score"` // 0-100
	Level       RiskLevel `json:"level"`
	Factors     []string `json:"factors"`
	Mitigations []string `json:"mitigations,omitempty"`
}

// EvaluationPayload is sent to POST /v1/evaluate.
type EvaluationPayload struct {
	Actor   Actor          `json:"actor"`
	Action  Action         `json:"action"`
	Target  Target         `json:"target"`
	Context map[string]any `json:"context,omitempty"`
}

// EvaluationResult is returned from POST /v1/evaluate.
type EvaluationResult struct {
	ID               string          `json:"id"`
	Decision         Decision        `json:"decision"`
	Risk             RiskAssessment  `json:"risk"`
	MatchedRuleID    string          `json:"matchedRuleId,omitempty"`
	PermitID         string          `json:"permitId,omitempty"`
	RequiresApproval bool            `json:"requiresApproval,omitempty"`
	EvaluatedAt      time.Time       `json:"evaluatedAt"`
	PolicyID         string          `json:"policyId,omitempty"`
	PolicyVersion    string          `json:"policyVersion,omitempty"`
}

// Permit represents an issued authorization token.
type Permit struct {
	ID           string       `json:"id"`
	EvaluationID string       `json:"evaluationId"`
	OrgID        string       `json:"orgId"`
	ActorID      string       `json:"actorId"`
	ActionType   string       `json:"actionType"`
	TargetID     string       `json:"targetId"`
	Status       PermitStatus `json:"status"`
	ExpiresAt    time.Time    `json:"expiresAt"`
	ConsumedAt   *time.Time   `json:"consumedAt,omitempty"`
	IssuedAt     time.Time    `json:"issuedAt"`
}

// Session holds the authenticated caller's identity and permissions.
type Session struct {
	UserID      string    `json:"userId"`
	OrgID       string    `json:"orgId"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	Permissions []string  `json:"permissions"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

// APIError is returned by the AtlaSent API on failure.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *APIError) Error() string {
	return e.Message
}
