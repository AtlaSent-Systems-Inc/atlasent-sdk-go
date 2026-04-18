# atlasent-sdk-go

Go SDK for **AtlaSent** execution-time authorization — wire-compatible with the
TypeScript and Python SDKs.

Two public methods on a `Client`:

- `client.Evaluate(ctx, EvaluateRequest)` → `POST /v1-evaluate`
- `client.VerifyPermit(ctx, VerifyPermitRequest)` → `POST /v1-verify-permit`

Plus a higher-level `Check` / `Guard` / `HTTPMiddleware` surface for
Principal/Resource-shaped call sites (colocating the authorization decision
with the side-effect it gates).

## Install

```sh
go get github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent
```

Requires Go 1.22+.

## Quickstart — Evaluate

```go
client, _ := atlasent.New(os.Getenv("ATLASENT_API_KEY"))

resp, err := client.Evaluate(ctx, atlasent.EvaluateRequest{
    Agent:  "clinical-data-agent",
    Action: "modify_patient_record",
    Context: map[string]any{
        "user":        "dr_smith",
        "environment": "production",
    },
})
if err != nil {
    return err
}
if !resp.Permitted {
    log.Printf("blocked: %s", resp.Reason)
    return nil
}
// use resp.DecisionID to call VerifyPermit if you need a second-factor gate
```

A clean policy DENY is returned in `resp.Permitted == false` — it is not an
error. Network, 4xx, 5xx, and malformed-response failures are `*atlasent.Error`.

## Check / Guard (higher-level)

```go
decision, err := client.Check(ctx, atlasent.CheckRequest{
    Principal: atlasent.Principal{ID: "user_alice", Groups: []string{"finance"}},
    Action:    "invoice.pay",
    Resource:  atlasent.Resource{ID: "inv_42", Type: "invoice"},
    Context:   map[string]any{"ip": "203.0.113.7"},
})
```

`Check` maps `Principal.ID` → `agent`, embeds `Resource` into the evaluation
context under `"resource"`, and returns a `Decision` compatible with earlier
versions of this SDK.

### Guard a sensitive function

```go
receipt, err := atlasent.Guard(ctx, client, atlasent.CheckRequest{
    Principal: alice,
    Action:    "invoice.pay",
    Resource:  invoice,
}, func(ctx context.Context) (string, error) {
    return billing.Pay(ctx, invoice.ID)
})
```

## Typed errors

Every SDK failure returns an `*atlasent.Error` with a stable `Code`
aligned with the TypeScript and Python SDKs:

```go
resp, err := client.Evaluate(ctx, req)
switch {
case atlasent.IsCode(err, atlasent.ErrRateLimited):
    time.Sleep(atlasent.AsError(err).RetryAfter)
    // retry
case atlasent.IsCode(err, atlasent.ErrInvalidAPIKey):
    log.Fatalf("bad API key: %s", atlasent.AsError(err).RequestID)
case err != nil:
    return err
}
```

| `Code`             | When                                                 |
|--------------------|------------------------------------------------------|
| `invalid_api_key`  | HTTP 401                                             |
| `forbidden`        | HTTP 403                                             |
| `rate_limited`     | HTTP 429 — inspect `err.RetryAfter`                  |
| `bad_request`      | HTTP 4xx (other than 401/403/429)                    |
| `server_error`     | HTTP 5xx                                             |
| `timeout`          | context deadline exceeded                            |
| `network`          | DNS / connection failure, transport threw            |
| `bad_response`     | non-JSON body or missing required fields             |

Every `*Error` carries `err.RequestID` — the UUID the SDK sent as
`X-Request-ID`, correlatable in server logs.

## Fail-closed by default

If the PDP is unreachable, `Check` returns a deny `Decision` *and* a non-nil
`*Error`. Opt out with `atlasent.WithFailOpen()` only when availability
outranks correctness.

## Version

`atlasent.Version` is injected at build time by the release workflow:

```sh
go build -ldflags "-X github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent.Version=0.5.0"
```

Local builds show `Version = "dev"`.

## Run the example

```sh
ATLASENT_API_KEY=ask_live_... go run ./examples/quickstart
```

Point at a non-production PDP with `ATLASENT_BASE_URL`.

## Related

- **TypeScript SDK:** [`@atlasent/sdk`](https://github.com/AtlaSent-Systems-Inc/atlasent-sdk/tree/main/typescript) — wire-compatible.
- **Python SDK:** [`atlasent`](https://github.com/AtlaSent-Systems-Inc/atlasent-sdk/tree/main/python) — wire-compatible.

## License

Apache-2.0.
