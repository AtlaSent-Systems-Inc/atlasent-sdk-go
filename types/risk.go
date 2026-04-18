package types

// RiskLevel represents the severity of a risk assessment.
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

// RiskAssessment is the canonical risk shape (aligned with @atlasent/types v2).
type RiskAssessment struct {
	Level         RiskLevel              `json:"level"`
	Score         float64                `json:"score"` // 0-100
	Reasons       []string               `json:"reasons"`
	DomainSignals map[string]interface{} `json:"domain_signals,omitempty"`
}
