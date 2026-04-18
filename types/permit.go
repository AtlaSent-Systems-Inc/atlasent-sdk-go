package types

import "time"

// PermitStatus represents the lifecycle state of a permit.
type PermitStatus string

const (
	PermitIssued   PermitStatus = "issued"
	PermitVerified PermitStatus = "verified"
	PermitConsumed PermitStatus = "consumed"
	PermitExpired  PermitStatus = "expired"
	PermitRevoked  PermitStatus = "revoked"
)

// Permit is a single-use authorization token issued by POST /v1/evaluate.
type Permit struct {
	ID          string       `json:"id"`
	OrgID       string       `json:"org_id"`
	ActorID     string       `json:"actor_id"`
	ActionID    string       `json:"action_id"`
	TargetID    string       `json:"target_id,omitempty"`
	Environment string       `json:"environment,omitempty"`
	Status      PermitStatus `json:"status"`
	IssuedAt    time.Time    `json:"issued_at"`
	ExpiresAt   time.Time    `json:"expires_at"`
	ConsumedAt  *time.Time   `json:"consumed_at,omitempty"`
	Signature   string       `json:"signature,omitempty"` // Ed25519 base64url
}
