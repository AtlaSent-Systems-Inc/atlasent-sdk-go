// worker shows how to apply AtlaSent authorization in an async consumer:
// each dequeued job names a principal and a resource, and the worker asks
// the PDP before executing. Jobs are batched into a single CheckMany round
// trip, then fanned out to per-job Guards.
//
// The same shape works for any queue (SQS, Kafka, NATS, Pub/Sub) — only the
// dequeue function changes.
//
// Run with a real PDP:
//
//	ATLASENT_API_KEY=... go run ./examples/worker
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
)

// Job is one dequeued unit of work. In a real system it would carry the
// principal via a signed token, not a plain ID.
type Job struct {
	ID          string
	PrincipalID string
	InvoiceID   string
	Amount      int
}

func dequeueBatch(_ context.Context) []Job {
	return []Job{
		{ID: "j1", PrincipalID: "user_alice", InvoiceID: "inv_42", Amount: 1200},
		{ID: "j2", PrincipalID: "user_bob", InvoiceID: "inv_43", Amount: 800},
		{ID: "j3", PrincipalID: "user_alice", InvoiceID: "inv_44", Amount: 5000},
	}
}

func main() {
	apiKey := os.Getenv("ATLASENT_API_KEY")
	if apiKey == "" {
		log.Fatal("set ATLASENT_API_KEY")
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	opts := []atlasent.Option{
		atlasent.WithCache(atlasent.NewMemoryCache(4096), 30*time.Second),
		atlasent.WithRetry(atlasent.DefaultRetryPolicy),
		atlasent.WithObserver(atlasent.MultiObserver(
			atlasent.SlogObserver(logger),
			&counters,
		)),
	}
	if base := os.Getenv("ATLASENT_BASE_URL"); base != "" {
		opts = append(opts, atlasent.WithBaseURL(base))
	}
	client, err := atlasent.New(apiKey, opts...)
	if err != nil {
		log.Fatalf("init: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	jobs := dequeueBatch(ctx)

	// Batch-authorize in a single round trip.
	reqs := make([]atlasent.CheckRequest, len(jobs))
	for i, j := range jobs {
		reqs[i] = atlasent.CheckRequest{
			Principal: atlasent.Principal{ID: j.PrincipalID, Type: "user"},
			Action:    "invoice.pay",
			Resource:  atlasent.Resource{ID: j.InvoiceID, Type: "invoice"},
			Context:   map[string]any{"amount_cents": j.Amount, "channel": "worker"},
		}
	}
	decisions, err := client.CheckMany(ctx, reqs)
	if err != nil {
		// Fail-closed: decisions already denies, so we still iterate and
		// record each failure rather than giving up on the whole batch.
		logger.Warn("pdp unavailable, falling through with fail-closed denies", "err", err)
	}

	for i, j := range jobs {
		if !decisions[i].Allowed {
			logger.Warn("skipping denied job",
				"job", j.ID,
				"reason", decisions[i].Reason,
				"policy", decisions[i].PolicyID,
			)
			continue
		}
		// Honor obligations before running the effect.
		if decisions[i].HasObligation("log:high-risk") {
			logger.Warn("high-risk job", "job", j.ID, "amount_cents", j.Amount)
		}
		// Re-Guard at the call-site so later decisions (e.g. policy reload)
		// are re-checked. The cache keeps this cheap.
		_, err := atlasent.Guard(ctx, client, reqs[i], func(ctx context.Context) (struct{}, error) {
			fmt.Printf("processing %s: charge invoice %s $%d\n", j.ID, j.InvoiceID, j.Amount)
			return struct{}{}, nil
		})
		var denied *atlasent.DeniedError
		if errors.As(err, &denied) {
			logger.Warn("race: re-check denied", "job", j.ID, "reason", denied.Decision.Reason)
		}
	}

	fmt.Printf("summary: allow=%d deny=%d errors=%d cache_hits=%d\n",
		counters.Allow.Load(), counters.Deny.Load(), counters.Errors.Load(), counters.CacheHits.Load())
}

var counters atlasent.Counters
