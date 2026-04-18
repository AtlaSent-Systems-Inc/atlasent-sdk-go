# atlasent-sdk-go

Go SDK for **AtlaSent** execution-time authorization.

Ask the AtlaSent Policy Decision Point (PDP) — at the exact call-site of a
sensitive action — whether a principal may perform an action on a resource.

## Install

```sh
go get github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent
```

Requires Go 1.21+ (the `Guard` helper uses generics).

Optional subpackage for gRPC servers:

```sh
go get github.com/atlasent-systems-inc/atlasent-sdk-go/grpc
```

## QuickStart

```go
client, _ := atlasent.New(os.Getenv("ATLASENT_API_KEY"))

decision, err := client.Check(ctx, atlasent.CheckRequest{
    Principal: atlasent.Principal{ID: "user_alice", Groups: []string{"finance"}},
    Action:    "invoice.pay",
    Resource:  atlasent.Resource{ID: "inv_42", Type: "invoice"},
    Context:   map[string]any{"ip": "203.0.113.7"},
})
if err != nil || !decision.Allowed {
    return fmt.Errorf("denied: %s", decision.Reason)
}
```

### Guard a sensitive function

Colocate the authorization decision with the side-effect it gates. `fn` only
runs when the PDP allows the request; otherwise you get a `*DeniedError`.

```go
receipt, err := atlasent.Guard(ctx, client, atlasent.CheckRequest{
    Principal: alice,
    Action:    "invoice.pay",
    Resource:  invoice,
}, func(ctx context.Context) (string, error) {
    return billing.Pay(ctx, invoice.ID)
})
```

### HTTP middleware

```go
mux.Handle("/invoices/", client.HTTPMiddleware(func(r *http.Request) (string, atlasent.Resource, map[string]any, error) {
    id := strings.TrimPrefix(r.URL.Path, "/invoices/")
    return "invoice.read", atlasent.Resource{ID: id, Type: "invoice"}, nil, nil
})(invoiceHandler))
```

Your auth layer must attach the Principal upstream:

```go
ctx = atlasent.WithPrincipal(r.Context(), atlasent.Principal{ID: claims.Sub})
```

### gRPC interceptors

```go
import atlasentgrpc "github.com/atlasent-systems-inc/atlasent-sdk-go/grpc"

s := grpc.NewServer(
    grpc.ChainUnaryInterceptor(jwtAuth, atlasentgrpc.UnaryServerInterceptor(client, resolve)),
    grpc.StreamInterceptor(atlasentgrpc.StreamServerInterceptor(client, resolve)),
)
```

Maps to gRPC status codes:
`Unauthenticated` (no Principal), `InvalidArgument` (resolver error),
`PermissionDenied` (PDP denied), `Unavailable` (fail-closed + PDP down).

### Batch checks

Ask N questions in one round trip — the right shape for list endpoints:

```go
decs, err := client.CheckMany(ctx, []atlasent.CheckRequest{
    {Principal: alice, Action: "invoice.read", Resource: inv1},
    {Principal: alice, Action: "invoice.read", Resource: inv2},
    {Principal: alice, Action: "invoice.read", Resource: inv3},
})
for i, d := range decs {
    if d.Allowed { /* include in response */ }
}
```

Cached entries are served locally; only uncached requests go on the wire.

## Caching

Hot paths should not round-trip the PDP on every call. Install a cache and
let the PDP hint TTLs per decision:

```go
client, _ := atlasent.New(apiKey,
    atlasent.WithCache(atlasent.NewMemoryCache(4096), 5*time.Second),
)
```

- `NewMemoryCache(n)` is a bounded LRU with per-entry TTLs. It's safe for
  concurrent use.
- If the PDP returns `ttl_ms` on a `Decision`, that wins; otherwise the SDK
  falls back to the default TTL you configured.
- Implement `atlasent.Cache` yourself to back it with Redis or Memcached.

## Retries

Transport errors, 429, and 5xx (except 501) are retried with exponential
backoff. `Retry-After` on 429/503 is honored.

```go
client, _ := atlasent.New(apiKey, atlasent.WithRetry(atlasent.DefaultRetryPolicy))
// or tune it:
client, _ = atlasent.New(apiKey, atlasent.WithRetry(atlasent.RetryPolicy{
    MaxAttempts:    4,
    InitialBackoff: 50 * time.Millisecond,
    MaxBackoff:     2 * time.Second,
    Multiplier:     2.0,
    Jitter:         true,
}))
```

## Observability

Plug an `Observer` to emit metrics, structured logs, or OpenTelemetry spans.
One callback per `Check`/`CheckMany` entry, including cache hits and
fail-open/fail-closed outcomes.

```go
client, _ := atlasent.New(apiKey,
    atlasent.WithObserver(atlasent.MultiObserver(
        atlasent.SlogObserver(slog.Default()),
        &myPrometheusObserver{},
    )),
)
```

Built-ins:

- `SlogObserver(logger)` — structured logs at info (allow) / warn (deny, error).
- `Counters` — atomic allow / deny / error / cache-hit counters.
- `MultiObserver(o1, o2, …)` — fan-out to multiple observers.

## Fail-closed by default

If the PDP is unreachable, `Check` returns a deny decision *and* a non-nil
transport error. Opt out with `atlasent.WithFailOpen()` only when availability
outranks correctness.

## Run the examples

```sh
ATLASENT_API_KEY=sk_live_... go run ./examples/quickstart
ATLASENT_API_KEY=sk_live_... go run ./examples/dbguard
ATLASENT_API_KEY=sk_live_... go run ./examples/worker
# gRPC example has its own go.mod (it pulls in google.golang.org/grpc):
cd examples/grpc && ATLASENT_API_KEY=sk_live_... go run .
```

Point at a non-production PDP with `ATLASENT_BASE_URL`.

## Layout

```
atlasent/                  # core SDK (Client, Guard, HTTPMiddleware, Cache, Retry, Observer, batch)
grpc/                      # gRPC server interceptors (separate go module)
examples/quickstart/       # minimal Check + Guard + middleware walkthrough
examples/dbguard/          # begin-tx → Guard → commit/rollback pattern
examples/worker/           # queue consumer using CheckMany + obligations
examples/grpc/             # gRPC server wiring (separate go module)
```
