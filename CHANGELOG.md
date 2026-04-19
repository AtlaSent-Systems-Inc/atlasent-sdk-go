# Changelog

All notable changes to the AtlaSent Go SDK are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] — 2026-04-19

First public release. Packaged for pkg.go.dev.

### Added

- `atlasent.Client` with the Check/Guard ergonomics Go callers expect:
  - `Check`, `CheckMany` (single-roundtrip batch with cache passthrough)
  - `Guard[T]` — colocates the PDP call with the side-effect it gates
  - `HTTPMiddleware` for `net/http`
  - Configurable cache (`WithCache`, `MemoryCache` LRU with per-entry TTL)
  - Configurable retry (`WithRetry`, `DefaultRetryPolicy`, honors `Retry-After`)
  - Pluggable `Observer` with `SlogObserver`, `Counters`, `MultiObserver`
  - Fail-closed by default; opt into fail-open with `WithFailOpen`
- OpenAPI v2 surface on `*Client`:
  - `Evaluate`, `Authorize`, `EvaluateStream` (SSE)
  - `VerifyPermit`, `ConsumePermit`, `RevokePermit`
  - `ListAuditEvents`, `CreateAuditExport`, `VerifyBundle`
  - `ListApprovals`, `CreateApproval`, `ResolveApproval`
  - `RequestOverride`, `ResolveOverride`
  - `GetSession`
- `atlasent.LegacyClient` — v1 wire-compat client matching the cross-SDK
  contract (`/v1-evaluate`, `/v1-verify-permit`). Passes the Python/TS
  contract vectors vendored in `atlasent/testdata/contract/vectors/`.
- `atlasent.MockClient` — test double with no network I/O.
- gRPC subpackage: unary + stream server interceptors (separate Go module
  so non-gRPC users don't pay for gRPC deps).
- Examples: `quickstart`, `dbguard`, `worker`, `grpc`, `deploy-gate`,
  `lims-write`, `batch-release`.
- oapi-codegen wiring under `internal/atlasentapi` for the canonical
  atlasent-api REST spec (regenerate via `go generate ./internal/...`).
- Contract conformance tests (`atlasent/contract_test.go`) replay the
  cross-SDK vectors against an httptest server.
- goreleaser config (`.goreleaser.yaml`) with sigstore cosign keyless
  signing of source archives + SHA-256 checksums.
- CI matrix on Go 1.22 / 1.23 / 1.24 with `-race`; gated staging-integration
  job; dedicated contract workflow.

### Changed

- Breaking: the package-level `Decision` type is now the rich PDP decision
  struct (fields: `Allowed`, `Reason`, `PolicyID`, `Obligations`,
  `TTLMillis`). The coarse allow/deny verdict string alias is now
  `atlasent.Outcome` with constants `OutcomeAllow`, `OutcomeDeny`,
  `OutcomeRequireApproval`. `EvaluationResult.Decision` → `.Outcome`.
  `MockRule.Decision` → `.Outcome`.
- `New(opts ClientOptions) *Client` → `New(apiKey string, opts ...Option)
  (*Client, error)`. Construction is now option-based; `FailClosed`
  defaults to `true`; API key auth uses `Authorization: Bearer`.

### Removed

- Unused top-level `types/` subpackage (the identical types live in
  `atlasent/`).

### Security

- Transport failures and PDP timeouts are fail-closed by default.
- API keys sent only as `Authorization: Bearer <key>`.
- All HTTP calls over TLS; certificate validation left to `http.DefaultClient`.
- Release artefacts keyless-signed via sigstore cosign.

[Unreleased]: https://github.com/AtlaSent-Systems-Inc/atlasent-sdk-go/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/AtlaSent-Systems-Inc/atlasent-sdk-go/releases/tag/v0.1.0
