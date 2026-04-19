# atlasent-sdk-go

[![Go Reference](https://pkg.go.dev/badge/github.com/atlasent-systems-inc/atlasent-sdk-go.svg)](https://pkg.go.dev/github.com/atlasent-systems-inc/atlasent-sdk-go)
[![Test](https://github.com/AtlaSent-Systems-Inc/atlasent-sdk-go/actions/workflows/test.yml/badge.svg)](https://github.com/AtlaSent-Systems-Inc/atlasent-sdk-go/actions/workflows/test.yml)
[![Contract](https://github.com/AtlaSent-Systems-Inc/atlasent-sdk-go/actions/workflows/contract.yml/badge.svg)](https://github.com/AtlaSent-Systems-Inc/atlasent-sdk-go/actions/workflows/contract.yml)

AtlaSent is a centralized policy decision point (PDP). Your code asks *may
principal X do action Y on resource Z?*, the PDP answers, your code
enforces. Policy lives server-side; this SDK is the thin HTTP client plus
ergonomics for the two hot paths: the authorization call-site and the
middleware / interceptor layer.

Wire parity with the Python and TypeScript SDKs is enforced by
[`atlasent-sdk/contract`][contract] vectors replayed in CI.

## Install

```bash
go get github.com/atlasent-systems-inc/atlasent-sdk-go
```

Requires Go 1.22 or later. Tested on 1.22, 1.23, 1.24.

## Quick start

```go
client, err := atlasent.New(os.Getenv("ATLASENT_API_KEY"),
    atlasent.WithCache(atlasent.NewMemoryCache(1024), 30*time.Second),
    atlasent.WithRetry(atlasent.DefaultRetryPolicy),
)
if err != nil { log.Fatal(err) }

decision, err := client.Check(ctx, atlasent.CheckRequest{
    Principal: atlasent.Principal{ID: "user_alice", Type: "user"},
    Action:    "invoice.pay",
    Resource:  atlasent.Resource{ID: "inv_42", Type: "invoice"},
    Context:   map[string]any{"amount_cents": 1200},
})
if err != nil {
    // transport failure: decision still honors FailClosed (default deny)
}
if !decision.Allowed {
    http.Error(w, decision.Reason, http.StatusForbidden)
    return
}
```

## Guard: colocate the PDP call with the side-effect

`Guard` runs `fn` only when the PDP allows. On deny it returns `*DeniedError`
wrapping the full `Decision`; on transport failure + fail-closed it returns
the transport error without running `fn`. Use it at the call-site so
authorization cannot drift from what it protects.

```go
invoice, err := atlasent.Guard(ctx, client, req, func(ctx context.Context) (*Invoice, error) {
    return db.PayInvoice(ctx, "inv_42")
})
var denied *atlasent.DeniedError
if errors.As(err, &denied) {
    log.Warn("payment denied", "reason", denied.Decision.Reason, "policy", denied.Decision.PolicyID)
    return
}
```

## net/http middleware

```go
mw := client.HTTPMiddleware(func(r *http.Request) (string, atlasent.Resource, map[string]any, error) {
    return "report.read",
        atlasent.Resource{ID: mux.Vars(r)["id"], Type: "report"},
        map[string]any{"ip": r.RemoteAddr},
        nil
})

http.Handle("/reports/", mw(reportHandler))
```

The upstream auth layer must set the principal on the request context with
`atlasent.WithPrincipal(ctx, atlasent.Principal{...})`.

## gRPC interceptors

The `grpc` subpackage lives in its own Go module so non-gRPC users don't pay
for gRPC dependencies:

```bash
go get github.com/atlasent-systems-inc/atlasent-sdk-go/grpc
```

```go
import atlasentgrpc "github.com/atlasent-systems-inc/atlasent-sdk-go/grpc"

s := grpc.NewServer(
    grpc.UnaryInterceptor(atlasentgrpc.UnaryServerInterceptor(client, resolve)),
    grpc.StreamInterceptor(atlasentgrpc.StreamServerInterceptor(client, resolve)),
)
```

## Batch authorization

```go
reqs := []atlasent.CheckRequest{
    {Principal: p, Action: "report.read",  Resource: atlasent.Resource{ID: "r1", Type: "report"}},
    {Principal: p, Action: "report.read",  Resource: atlasent.Resource{ID: "r2", Type: "report"}},
    {Principal: p, Action: "report.share", Resource: atlasent.Resource{ID: "r2", Type: "report"}},
}
decisions, err := client.CheckMany(ctx, reqs) // one round trip, results in order
```

## Permits, audit, approvals, overrides

All OpenAPI v2 endpoints are exposed as methods on `*Client`:

| Area       | Methods                                                                       |
|------------|-------------------------------------------------------------------------------|
| Evaluation | `Evaluate`, `EvaluateStream`, `Authorize`, `Check`, `CheckMany`, `Guard`      |
| Permits    | `VerifyPermit`, `ConsumePermit`, `RevokePermit`                               |
| Audit      | `ListAuditEvents`, `CreateAuditExport`, `VerifyBundle`                        |
| Approvals  | `ListApprovals`, `CreateApproval`, `ResolveApproval`                          |
| Overrides  | `RequestOverride`, `ResolveOverride`                                          |
| Session    | `GetSession`                                                                  |

## Testing

`*MockClient` is a drop-in test double with no network I/O:

```go
mock := atlasent.NewMock().AllowAll()
mock.SetDecision(atlasent.MockRule{ActionType: "data:delete", Outcome: atlasent.OutcomeDeny})

result, _ := mock.Evaluate(ctx, payload)
calls := mock.Calls() // inspect what was evaluated
```

## Cross-SDK wire contract

The cross-SDK v1 wire contract lives at [`atlasent-sdk/contract`][contract].
Use `LegacyClient` to talk to a v1-compatible endpoint with the exact wire
shape the Python / TypeScript SDKs use:

```go
legacy := atlasent.NewLegacy(client)
out, err := legacy.Evaluate(ctx, "clinical-data-agent", "read_patient_record", nil)
```

The contract vectors are vendored into `atlasent/testdata/contract/vectors/`
and replayed by `TestContract*` in every CI run. Refresh them with
`./scripts/update-contract-vectors.sh`.

## Examples

| Example                           | What it shows                                             |
|-----------------------------------|-----------------------------------------------------------|
| [`examples/quickstart`][ex-qs]    | Minimal authorize-and-execute                             |
| [`examples/deploy-gate`][ex-dg]   | Gating a deploy pipeline on PDP decisions                 |
| [`examples/lims-write`][ex-lims]  | GxP-style clinical data write with obligations            |
| [`examples/batch-release`][ex-br] | Batch release-train sign-off via `CheckMany`              |
| [`examples/dbguard`][ex-db]       | Gating database operations behind Guard                   |
| [`examples/worker`][ex-worker]    | Worker queue authorization + `CheckMany`                  |
| [`examples/grpc`][ex-grpc]        | gRPC service with unary + stream interceptors             |

## Development

```bash
go test -race ./...               # unit + contract tests
go generate ./internal/...        # regenerate oapi-codegen output
./scripts/update-contract-vectors.sh  # refresh vendored contract vectors
```

Releases are tagged as `vX.Y.Z`; the `Release` workflow runs goreleaser to
publish signed (sigstore cosign, keyless) source archives and a SHA-256
checksums file.

## License

MIT. See [LICENSE](./LICENSE).

[contract]: https://github.com/AtlaSent-Systems-Inc/atlasent-sdk/tree/main/contract
[ex-qs]: ./examples/quickstart
[ex-dg]: ./examples/deploy-gate
[ex-lims]: ./examples/lims-write
[ex-br]: ./examples/batch-release
[ex-db]: ./examples/dbguard
[ex-worker]: ./examples/worker
[ex-grpc]: ./examples/grpc
