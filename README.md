# atlasent-sdk-go

AtlaSent Go SDK — policy evaluation, permit management, and governance.

## Installation

```bash
go get github.com/atlasent-systems-inc/atlasent-sdk-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
)

func main() {
    client := atlasent.New(atlasent.ClientOptions{
        APIURL: "https://your-project.supabase.co/functions/v1",
        APIKey: "your-api-key",
    })

    ok, result, err := client.Authorize(context.Background(),
        atlasent.Actor{ID: "user-123", Type: "user"},
        "data:export",
        atlasent.Target{ID: "report-456", Type: "report", Environment: "production"},
    )
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Decision: %s (risk: %s)\n", result.Decision, result.Risk.Level)
    if ok {
        fmt.Printf("Permit: %s\n", result.PermitID)
    }
}
```

## Batch Authorization

```go
results := client.AuthorizeMany(ctx, []atlasent.EvaluationPayload{
    {Actor: actor, Action: atlasent.Action{ID: uuid.NewString(), Type: "data:read"}, Target: target1},
    {Actor: actor, Action: atlasent.Action{ID: uuid.NewString(), Type: "data:write"}, Target: target2},
})
for _, r := range results {
    if r.Err == nil {
        fmt.Printf("%s → %s\n", r.Payload.Action.Type, r.Result.Decision)
    }
}
```

## Testing

```go
mock := atlasent.NewMock().AllowAll()
mock.SetDecision(atlasent.MockRule{ActionType: "data:delete", Decision: atlasent.DecisionDeny})

result, _ := mock.Evaluate(ctx, payload)
calls := mock.Calls() // inspect what was evaluated
```

## Types

All types align with `@atlasent/types` v2.0.0: `Actor`, `Action`, `Target`, `RiskAssessment`, `EvaluationResult`, `Permit`, `Session`.
