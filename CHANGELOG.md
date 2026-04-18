# Changelog

## 0.5.0 — 2026-04-18

Parity sprint: brings the Go SDK to contract parity with `@atlasent/sdk`
(TypeScript v0.5.0) and `atlasent` (Python v1.1.0).

### Added

- `Client.Evaluate(ctx, EvaluateRequest)` → `POST /v1-evaluate`.
- `Client.VerifyPermit(ctx, VerifyPermitRequest)` → `POST /v1-verify-permit`.
- Typed `*Error` returned on every failure path, with a stable `Code`
  drawn from `ErrInvalidAPIKey`, `ErrForbidden`, `ErrRateLimited`,
  `ErrTimeout`, `ErrNetwork`, `ErrBadResponse`, `ErrBadRequest`,
  `ErrServerError`. Use `atlasent.IsCode(err, code)` to branch.
- `Retry-After` header parsing on 429 → `err.RetryAfter time.Duration`.
- `X-Request-ID` UUID header on every request, echoed in `err.RequestID`.
- `Accept: application/json` header on every request.
- Build-time version injection via `-ldflags "-X …/atlasent.Version=..."`.
- Response-shape adapter: the SDK accepts both the canonical shape
  (`permitted`, `decision_id`, `audit_hash`, `verified`) and the
  legacy native shape (`decision: "allow"`, `permit_token`,
  `audit_entry_hash`, `valid`). Canonical keys win on mixed shapes.
- GitHub Actions `ci.yml` (go vet / build / test-race across Go 1.22–1.24
  + `golangci-lint`) and `release.yml` (validates that the
  ldflags-injected Version matches the pushed git tag).

### Changed

- `Check` / `Guard` / `HTTPMiddleware` now call `Evaluate` under the
  hood. `Principal.ID` maps to the wire `actor_id`; `Resource` is
  embedded in the evaluation context under `"resource"`. Decisions
  from the PDP are returned via the existing `Decision` struct.
- `HTTPMiddleware` propagates `Retry-After` seconds on rate-limit-sourced
  503 responses so downstream proxies can back off.
- Endpoint fixed: was `/v1/authorize`, now `/v1-evaluate` +
  `/v1-verify-permit` to match the canonical contract.

### Fixed

- Missing `Accept` header (previously only `Content-Type`).
- Missing `X-Request-ID` — there was no correlation ID at all.
- Untyped error strings — callers could not programmatically branch on
  `invalid_api_key` / `rate_limited` / `server_error`.
- Version was a hard-coded `"atlasent-sdk-go/0.1"` string in User-Agent.
  Now sourced from `Version` (ldflags-settable).
