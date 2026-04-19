// batch-release gates a release-train with a single batched PDP round trip.
// Each candidate service gets its own (principal, action, resource) tuple;
// CheckMany fans them out concurrently on the server and returns decisions
// in input order.
//
// A release-train ships only if every candidate is allowed. Partial denies
// print a structured block report so the on-call engineer can route each to
// the right owner.
//
// Run:
//
//	ATLASENT_API_KEY=... go run ./examples/batch-release
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
)

type candidate struct {
	Service string
	Version string
	Env     string
	Owner   string
}

func main() {
	apiKey := os.Getenv("ATLASENT_API_KEY")
	if apiKey == "" {
		log.Fatal("set ATLASENT_API_KEY")
	}
	client, err := atlasent.New(apiKey,
		atlasent.WithCache(atlasent.NewMemoryCache(256), 10*time.Second),
		atlasent.WithRetry(atlasent.DefaultRetryPolicy),
	)
	if err != nil {
		log.Fatalf("init: %v", err)
	}

	candidates := []candidate{
		{Service: "payments", Version: "v2.17.0", Env: "production", Owner: "payments-oncall"},
		{Service: "checkout", Version: "v4.1.2", Env: "production", Owner: "checkout-oncall"},
		{Service: "ledger", Version: "v1.9.4", Env: "production", Owner: "ledger-oncall"},
	}

	reqs := make([]atlasent.CheckRequest, len(candidates))
	for i, c := range candidates {
		reqs[i] = atlasent.CheckRequest{
			Principal: atlasent.Principal{ID: "release_bot", Type: "service"},
			Action:    "service.deploy",
			Resource: atlasent.Resource{
				ID:   c.Service,
				Type: "service",
				Attributes: map[string]any{
					"version": c.Version,
					"env":     c.Env,
				},
			},
			Context: map[string]any{
				"train":    "release-2026-04-19",
				"owner":    c.Owner,
				"freeze":   isChangeFreeze(time.Now()),
				"attempt":  1,
			},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	decisions, err := client.CheckMany(ctx, reqs)
	if err != nil {
		// Fail-closed means every slot already carries a deny; keep going
		// so the on-call engineer gets a usable block report.
		log.Printf("pdp unavailable (fail-closed applies): %v", err)
	}

	var blocked []map[string]any
	for i, c := range candidates {
		dec := decisions[i]
		if dec.Allowed {
			fmt.Printf("  ✓ %-12s %-8s env=%s\n", c.Service, c.Version, c.Env)
			continue
		}
		blocked = append(blocked, map[string]any{
			"service":   c.Service,
			"version":   c.Version,
			"owner":     c.Owner,
			"reason":    dec.Reason,
			"policy_id": dec.PolicyID,
		})
	}

	if len(blocked) > 0 {
		out, _ := json.MarshalIndent(blocked, "", "  ")
		fmt.Fprintf(os.Stderr, "\nRELEASE TRAIN BLOCKED — %d of %d services denied:\n%s\n",
			len(blocked), len(candidates), out)
		os.Exit(1)
	}
	fmt.Println("\nRELEASE TRAIN DEPARTED — all services allowed")
}

// isChangeFreeze is a stand-in for a real change-calendar lookup. Returns
// true between Friday 18:00 UTC and Monday 06:00 UTC.
func isChangeFreeze(t time.Time) bool {
	t = t.UTC()
	switch t.Weekday() {
	case time.Saturday, time.Sunday:
		return true
	case time.Friday:
		return t.Hour() >= 18
	case time.Monday:
		return t.Hour() < 6
	}
	return false
}
