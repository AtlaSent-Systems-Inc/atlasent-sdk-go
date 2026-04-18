# atlasent-sdk-go

Go SDK for **AtlaSent** execution-time authorization.

Ask the AtlaSent Policy Decision Point (PDP) — at the exact call-site of a
sensitive action — whether a principal may perform an action on a resource.

## Install

```sh
go get github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent
```

Requires Go 1.21+ (the `Guard` helper uses generics).

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

## Fail-closed by default

If the PDP is unreachable, `Check` returns a deny decision *and* a non-nil
transport error. Opt out with `atlasent.WithFailOpen()` only when availability
outranks correctness.

## Run the example

```sh
ATLASENT_API_KEY=sk_live_... go run ./examples/quickstart
```

Point at a non-production PDP with `ATLASENT_BASE_URL`.

## Layout

```
atlasent/          # SDK package (client, types, Guard, middleware)
examples/quickstart/   # end-to-end QuickStart
```
