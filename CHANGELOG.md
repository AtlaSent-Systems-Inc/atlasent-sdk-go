# Changelog

All notable changes to the AtlaSent Go SDK are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] — 2026-04-17

First stable release of the AtlaSent Go SDK.

### Added

- `client.go` — `AtlasentClient` with `Authorize()` and `VerifyPermit()` methods
- HTTP middleware: `AuthorizeMiddleware` for net/http handlers
- gRPC interceptors: unary and stream server interceptors
- Batch authorization: `AuthorizeBatch()` for multiple concurrent checks
- LRU permit cache with configurable TTL
- Retry with exponential backoff (configurable max attempts)
- Observer pattern: pluggable `DecisionObserver` interface for metrics/tracing
- `examples/quickstart/` — minimal authorize-and-execute pattern
- `examples/dbguard/` — database operation gating
- `examples/worker/` — background job authorization
- `examples/grpc/` — gRPC service with AtlaSent interceptors
- GitHub Actions workflow: `go test ./...` on push and PR

### Security

- All requests sent over HTTPS; TLS certificate validation enforced
- API keys passed via `Authorization: Bearer` header, never in URL
- Fail-closed: network errors and timeouts return `deny` decision

[1.0.0]: https://github.com/AtlaSent-Systems-Inc/atlasent-sdk-go/releases/tag/v1.0.0
