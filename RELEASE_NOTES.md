# Release Notes — v1.0.0

**Release date:** 2026-04-17

## AtlaSent Go SDK v1.0.0

First stable release. Module path: `github.com/atlasent-systems-inc/atlasent-sdk-go`

```bash
go get github.com/atlasent-systems-inc/atlasent-sdk-go@v1.0.0
```

### Public API

```go
import atlasent "github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"

client := atlasent.New(atlasent.Config{
    APIKey:  os.Getenv("ATLASENT_API_KEY"),
    BaseURL: "https://ihghhasvxtltlbizvkqy.supabase.co/functions/v1",
})

result, err := client.Authorize(ctx, atlasent.Request{
    Agent:  "my-agent",
    Action: "deployment.production",
    Context: map[string]any{"actor": "user:123"},
})
if err != nil || !result.Permitted {
    return fmt.Errorf("not authorized: %v", result.Decision)
}
// safe to proceed
```

### net/http middleware

```go
http.Handle("/deploy", atlasent.AuthorizeMiddleware(client, "deployment.production", handler))
```

### gRPC interceptor

```go
grpc.NewServer(
    grpc.UnaryInterceptor(atlasent.UnaryServerInterceptor(client)),
)
```

### Stability guarantees

All exported types and functions in the `atlasent` package are stable as of v1.0.0. The `grpc` submodule is separately versioned.
