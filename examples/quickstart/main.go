// QuickStart demonstrates AtlaSent execution-time authorization in three
// common shapes:
//
//  1. A direct Check call — inspect a Decision yourself.
//  2. A Guard wrapper around a sensitive function — authorization colocated
//     with the side-effect it gates.
//  3. HTTP middleware that protects a handler.
//
// Run against the live API:
//
//	ATLASENT_API_KEY=sk_live_... go run ./examples/quickstart
//
// Or point at a local / staging PDP:
//
//	ATLASENT_API_KEY=... ATLASENT_BASE_URL=http://localhost:8080 go run ./examples/quickstart
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
)

func main() {
	apiKey := os.Getenv("ATLASENT_API_KEY")
	if apiKey == "" {
		log.Fatal("set ATLASENT_API_KEY to run the quickstart")
	}

	opts := []atlasent.Option{}
	if base := os.Getenv("ATLASENT_BASE_URL"); base != "" {
		opts = append(opts, atlasent.WithBaseURL(base))
	}
	client, err := atlasent.New(apiKey, opts...)
	if err != nil {
		log.Fatalf("init client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	alice := atlasent.Principal{
		ID:     "user_alice",
		Type:   "user",
		Groups: []string{"finance"},
	}
	invoice := atlasent.Resource{
		ID:   "inv_42",
		Type: "invoice",
		Attributes: map[string]any{
			"amount_cents": 12_500,
			"currency":     "USD",
			"owner":        "user_alice",
		},
	}

	// 1. Direct Check. Even on transport error the SDK returns a Decision
	//    (fail-closed deny by default) — log the error and proceed.
	decision, err := client.Check(ctx, atlasent.CheckRequest{
		Principal: alice,
		Action:    "invoice.read",
		Resource:  invoice,
	})
	if err != nil {
		log.Printf("check transport error: %v", err)
	}
	fmt.Printf("read allowed=%v reason=%q policy=%s\n",
		decision.Allowed, decision.Reason, decision.PolicyID)

	// 2. Guard a sensitive function. payInvoice only runs if the PDP allows it.
	receipt, err := atlasent.Guard(ctx, client, atlasent.CheckRequest{
		Principal: alice,
		Action:    "invoice.pay",
		Resource:  invoice,
		Context: map[string]any{
			"ip":         "203.0.113.7",
			"mfa_age_ms": 30_000,
		},
	}, func(ctx context.Context) (string, error) {
		return payInvoice(ctx, invoice.ID)
	})
	var denied *atlasent.DeniedError
	switch {
	case errors.As(err, &denied):
		fmt.Printf("pay denied: %s (policy %s)\n", denied.Decision.Reason, denied.Decision.PolicyID)
	case atlasent.IsTransport(err):
		fmt.Printf("pay skipped: pdp unavailable (%v)\n", err)
	case err != nil:
		fmt.Printf("pay error: %v\n", err)
	default:
		fmt.Printf("pay ok: %s\n", receipt)
	}

	// 3. HTTP middleware example (demonstration only — not started).
	mux := http.NewServeMux()
	mux.HandleFunc("/invoices/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("invoice payload"))
	})
	resolve := func(r *http.Request) (string, atlasent.Resource, map[string]any, error) {
		id := r.URL.Path[len("/invoices/"):]
		return "invoice.read", atlasent.Resource{ID: id, Type: "invoice"}, nil, nil
	}
	_ = client.HTTPMiddleware(resolve)(mux)
	fmt.Println("middleware wired; mount it on your router and set a Principal upstream with atlasent.WithPrincipal")
}

// payInvoice stands in for the real side-effect the authorization is gating.
func payInvoice(_ context.Context, id string) (string, error) {
	return "receipt-for-" + id, nil
}
