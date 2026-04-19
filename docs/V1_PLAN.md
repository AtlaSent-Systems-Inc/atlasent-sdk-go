# atlasent-sdk-go — V1 Plan

**Role:** Go SDK for AtlaSent. Same wire contract as Python + TS SDKs
in `atlasent-sdk`. Go ships in its own repo because `pkg.go.dev`
expects a single-module repo layout.

**ICP this round:** platform engineers at biotechs running Go services
(LIMS sidecars, deploy gates, data-plane adapters) who need to wire
AtlaSent into an existing Go codebase.

---

## Context

- Default branch is `claude/quickstart-execution-auth-aLRNy` (should
  be renamed to `main` as part of V1).
- Wire contract lives in `atlasent-sdk/contract/` — this SDK must
  consume the same vectors to prove parity.
- Target module path: `github.com/atlasent-systems-inc/atlasent-sdk-go`
  published via `pkg.go.dev` on git-tag.

---

## V1 gates

### Surface parity

- [ ] `Evaluate`, `VerifyPermit`, `Session`, `AuditEvents`,
      `AuditExports`, `AuditVerify`, `Approvals`, `Overrides`,
      `ConsumePermit`, `RevokePermit`.
- [ ] Streaming `EvaluateStream` exposed as `<-chan Decision` or via
      `iter.Seq2` (Go 1.23+).
- [ ] Offline audit verifier: `VerifyBundle(path string) error` that
      validates an Ed25519-signed export without a network call.
- [ ] Contract vectors from `atlasent-sdk/contract/` pass against
      this SDK.

### Types + generation

- [ ] Types generated from `atlasent-api/openapi.yaml` via
      `oapi-codegen`. Hand-written types only for constructors and
      errors.
- [ ] No drift between generated Go structs and the TS/Python
      counterparts — enforced by a contract-vectors CI job.

### Publish story

- [ ] Rename default branch to `main`.
- [ ] `v0.1.0` tag publishes to `pkg.go.dev` (automatic on tag push).
- [ ] goreleaser cuts signed release artefacts + checksums.
- [ ] GitHub release notes auto-populated from CHANGELOG.md.

### Testing

- [ ] `go test ./...` green on Go 1.22, 1.23, 1.24.
- [ ] Contract vectors executed in CI; fail the build on wire drift.
- [ ] Race detector enabled (`-race`) in CI.
- [ ] Integration suite against staging `atlasent-api`.

### Docs + DX

- [ ] README quickstart: `go get` + hello-world that compiles.
- [ ] `godoc` examples (`func ExampleClient_Evaluate()`) for the top
      5 surface functions.
- [ ] `examples/` directory: deploy-gate, LIMS write, batch release.
- [ ] Errors wrap `request_id` for support escalation.

---

## Sequencing

1. Rename default branch to `main`. Tag current state as `v0.0.0`.
2. Wire `oapi-codegen` against `atlasent-api/openapi.yaml`.
3. Fill endpoint parity gaps.
4. Add contract vectors from `atlasent-sdk/contract/`.
5. Cut `v0.1.0`. Verify `pkg.go.dev` picks it up.
6. Write README + godoc examples.

---

## Out of scope for V1

- gRPC transport (REST + JSON only).
- Server-side Go helpers (webhook verifiers, etc.) — push to a
  separate `atlasent-go-server` repo if needed.
- Go 1.21 or earlier support.

---

## Risks

- **Branch hygiene.** Default branch is still a claude/ branch.
  Rename before first publish so `pkg.go.dev` doesn't cache a
  claude-branch module.
- **oapi-codegen config drift.** Generator options get ossified;
  document them in a `codegen.yaml` and commit that.
- **Go version floor.** `iter.Seq2` (1.23+) would simplify streaming.
  Decide whether to require 1.23 or to backport to channels on 1.22.
