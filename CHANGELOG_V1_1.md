# atlasent-sdk-go v1.1 Changelog

## v1.1 (Incremental — non-breaking type alignment)

> v1.1 aligns Go type shapes with the canonical `@atlasent/types` v2.0 definitions.
> No architectural changes. All v1.0 client code remains compatible.

### Type Shape Alignment

#### `RiskAssessment`

```go
// v1.0 (diverged shape)
type RiskAssessment struct {
    RiskScore  float64  `json:"risk_score"`
    RiskBand   string   `json:"risk_band"`
    RiskReasons []string `json:"risk_reasons"`
}

// v1.1 (canonical shape, matches @atlasent/types)
type RiskAssessment struct {
    Level         string                 `json:"level"`          // "low"|"medium"|"high"|"critical"
    Score         float64                `json:"score"`          // 0-100
    Reasons       []string               `json:"reasons"`
    DomainSignals map[string]interface{} `json:"domain_signals,omitempty"`
}
```

#### `PermitStatus`

```go
// v1.0 (diverged enum)
const (
    PermitValid   PermitStatus = "valid"
    PermitUsed    PermitStatus = "used"
    PermitExpired PermitStatus = "expired"
    PermitRevoked PermitStatus = "revoked"
)

// v1.1 (canonical enum, matches @atlasent/types)
const (
    PermitIssued   PermitStatus = "issued"
    PermitVerified PermitStatus = "verified"
    PermitConsumed PermitStatus = "consumed"
    PermitExpired  PermitStatus = "expired"
    PermitRevoked  PermitStatus = "revoked"
)
```

#### Migration

If you were using `PermitUsed`, replace with `PermitConsumed`.
If you were using `RiskScore`/`RiskBand`/`RiskReasons`, replace with `Score`/`Level`/`Reasons`.

### No Other Breaking Changes

- `AtlaSentClient` struct and all method signatures unchanged
- `Evaluate()`, `VerifyPermit()`, `ConsumePermit()` signatures unchanged
- Config struct unchanged
