# Changelog

All notable changes to the AtlaSent Go SDK are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] — 2026-04-19

Substantial expansion from the 1.0 baseline. The one breaking change
— a `context.Context` parameter on the `Cache` interface — is called
out explicitly below. If you have not implemented `atlasent.Cache`
yourself, upgrading is a drop-in.

### Added — core SDK

- **Combinators.** `CheckAny` (first-allowed) and `CheckAll`
  (all-or-deny), each in one round trip via the existing batch endpoint.
- **Obligation registry.** `ObligationRegistry` with
  unknown-fails-by-default so a PDP-issued obligation like `redact:ssn`
  can never be silently dropped.
- **Typed errors.** `APIError` + `ErrorKind` (`Transport`, `Unauthorized`,
  `Forbidden`, `RateLimit`, `Invalid`, `Server`, `Validation`) with
  `IsTransport` / `IsUnauthorized` / `IsRateLimit` / etc. helpers.
- **Validation.** `CheckRequest.Validate` fast-fails empty actions locally.
- **DryRun.** `CheckRequest.DryRun` asks the PDP to evaluate without
  emitting audit records (useful for "what-if" UI pre-checks).
- **Circuit breaker.** `WithCircuitBreaker` with a
  closed → open → half-open state machine. `BreakerConfig.OnStateChange`
  fires on every transition; `IsBreakerOpen(err)` distinguishes a
  breaker short-circuit from a live outage.
- **Async observer.** `NewAsyncObserver` hands events to a background
  goroutine with drop-newest back-pressure so slow metric exporters
  never block `Check`.
- **JWT helpers.** `PrincipalFromClaims` / `PrincipalFromJWT` with
  overridable claim names (Cognito, Auth0 namespaced claims, custom IDPs).
  Does NOT verify signatures — pair with your JWT library.
- **Resource struct tags.** `ResourceFrom(v, defaultType)` derives a
  `Resource` from `atlasent:"id|type|attr[,name=x]"` struct tags.
- **Context enrichers.** `ContextEnricher` + `WithContextEnricher`
  auto-merge cross-cutting values (request IDs, trace IDs, tenant IDs)
  into every `CheckRequest.Context`. Caller-supplied keys take
  precedence. Built-in: `RequestIDEnricher` + `WithRequestID`.
- **Local evaluator.** `LocalEvaluator` interface + `WithLocalEvaluator`
  option so hybrid-mode implementations (see new `bundle/` submodule)
  can short-circuit `Check` before the remote call.
  `CheckEvent.LocalHit` lets observers count it.
- **Package docs.** New `doc.go` with a getting-started overview,
  fail-closed contract, production knobs, and enforcement shapes.
  Runnable godoc `Example_*` functions for `Check`, `Guard`,
  `CheckMany`, `WithCache`, `ObligationRegistry`.
- **Benchmarks** for `Check`, cached check, `Guard`, `CheckMany`, and
  cache primitives.
- **`Version`** constant (= "1.1.0"); `User-Agent` derives from it.

### Added — submodules (each its own `go.mod`, deps stay off the core)

- `atlasenttest/` — httptest-backed fake PDP with fluent
  `On` / `OnResource` / `OnExact` rules for consumer tests.
- `connectrpc/` — interceptor for `connectrpc.com/connect` mirroring
  the existing gRPC adapter.
- `otel/` — OpenTelemetry Observer (counter, duration histogram,
  reconstructed spans) plus `TraceIDEnricher` that copies the active
  `trace_id` / `span_id` into every `CheckRequest.Context`.
- `cacheredis/` — Redis-backed `Cache` using go-redis; miniredis tests.
- `bundle/` — **hybrid-mode evaluator.** Signed-bundle `Syncer` (ETag,
  Ed25519 verification, no skip-verify switch), `Manager` with atomic
  engine-state swap + background refresh + `OnBundleChange` callback,
  pluggable `PolicyEngine` (Cedar/Rego/CEL not bundled — pick one).
- `middleware/gin`, `middleware/echo`, `middleware/fiber` — per-framework
  adapters mirroring the built-in `HTTPMiddleware`. chi needs no
  adapter — the built-in middleware is a drop-in `func(http.Handler)
  http.Handler`.

### Added — tooling

- `LICENSE` (Apache-2.0).
- Full CI matrix (`.github/workflows/ci.yml`): `go test -race`,
  `staticcheck`, `govulncheck` across every module.
- `.goreleaser.yaml` + `.github/workflows/release.yml`: source archive,
  SBOMs, checksums, drafted GitHub Release on `vX.Y.Z` tag push.
- `RELEASING.md`: multi-module tag dance, submodule require-update
  procedure, module-proxy-aware rollback guidance.
- `go.work` for local workspace development.

### Changed

- **BREAKING**: `atlasent.Cache` interface methods now take
  `context.Context`. Networked implementations (Redis, Memcached) need
  the ctx to honor cancellation. If you implemented `Cache` yourself,
  add `ctx context.Context` as the first parameter to `Get` and `Set`.
  The built-in `NewMemoryCache` is unchanged from the consumer side.
- `User-Agent` is now `atlasent-sdk-go/1.1.0` (was `atlasent-sdk-go/1.0`).
- `Check` / `CheckMany` return typed `*APIError` on transport/protocol
  failures — `errors.As` / `Is*` helpers are the supported inspection
  path.

### Fixed

- `Client.retry` no longer races on a shared `*math/rand.Rand`. Jitter
  uses `math/rand/v2` package-level functions which are concurrent-safe.
- `MultiObserver` isolates per-observer panics so one bad observer
  no longer skips later ones.
- `examples/quickstart` and `examples/dbguard` no longer
  `log.Fatalf` on transport errors — they now demonstrate the
  fail-closed flow.

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

[1.1.0]: https://github.com/AtlaSent-Systems-Inc/atlasent-sdk-go/releases/tag/v1.1.0
[1.0.0]: https://github.com/AtlaSent-Systems-Inc/atlasent-sdk-go/releases/tag/v1.0.0
