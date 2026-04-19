// dbguard shows how to colocate an authorization decision with a database
// write. The pattern: open a transaction, ask AtlaSent whether the write is
// allowed, then either commit or roll back based on the decision.
//
// The same shape works for any side-effect that must be undone on denial —
// uploading to S3, enqueuing a job, calling a payment provider.
//
// Run with a real PDP:
//
//	ATLASENT_API_KEY=... go run ./examples/dbguard
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
)

// tx stands in for *sql.Tx / *pgx.Tx. Replace with your driver's type.
type tx struct {
	committed  bool
	rolledBack bool
}

func (t *tx) Exec(_ context.Context, _ string, _ ...any) error { return nil }
func (t *tx) Commit() error                                    { t.committed = true; return nil }
func (t *tx) Rollback() error                                  { t.rolledBack = true; return nil }

func beginTx(_ context.Context) (*tx, error) { return &tx{}, nil }

func main() {
	apiKey := os.Getenv("ATLASENT_API_KEY")
	if apiKey == "" {
		log.Fatal("set ATLASENT_API_KEY")
	}
	opts := []atlasent.Option{
		atlasent.WithCache(atlasent.NewMemoryCache(4096), 5*time.Second),
		atlasent.WithRetry(atlasent.DefaultRetryPolicy),
	}
	if base := os.Getenv("ATLASENT_BASE_URL"); base != "" {
		opts = append(opts, atlasent.WithBaseURL(base))
	}
	client, err := atlasent.New(apiKey, opts...)
	if err != nil {
		log.Fatalf("init: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	principal := atlasent.Principal{ID: "user_alice", Type: "user"}
	invoice := atlasent.Resource{ID: "inv_42", Type: "invoice"}

	receipt, err := updateInvoiceGuarded(ctx, client, principal, invoice, "paid")
	var denied *atlasent.DeniedError
	switch {
	case errors.As(err, &denied):
		fmt.Printf("denied: %s (policy %s)\n", denied.Decision.Reason, denied.Decision.PolicyID)
	case atlasent.IsTransport(err):
		fmt.Printf("skipped: pdp unavailable (%v)\n", err)
	case err != nil:
		log.Fatalf("update: %v", err)
	default:
		fmt.Printf("updated: %s\n", receipt)
	}
}

// updateInvoiceGuarded demonstrates the begin → check → commit/rollback
// pattern. The transaction is opened first so the same view of the world the
// write uses is the view the PDP reasons about; it is rolled back if the PDP
// denies or the write fails.
func updateInvoiceGuarded(
	ctx context.Context,
	client *atlasent.Client,
	principal atlasent.Principal,
	invoice atlasent.Resource,
	newStatus string,
) (string, error) {
	t, err := beginTx(ctx)
	if err != nil {
		return "", fmt.Errorf("begin: %w", err)
	}
	// On any early return below, Rollback is the safe default; the defer is
	// cheap since Commit()+Rollback() on the same tx is a driver no-op (or we
	// track the flag ourselves, as here).
	defer func() {
		if !t.committed {
			_ = t.Rollback()
		}
	}()

	receipt, err := atlasent.Guard(ctx, client, atlasent.CheckRequest{
		Principal: principal,
		Action:    "invoice.update_status",
		Resource:  invoice,
		Context:   map[string]any{"new_status": newStatus},
	}, func(ctx context.Context) (string, error) {
		if err := t.Exec(ctx, "UPDATE invoices SET status=$1 WHERE id=$2", newStatus, invoice.ID); err != nil {
			return "", err
		}
		return "receipt-for-" + invoice.ID, nil
	})
	if err != nil {
		return "", err
	}

	if err := t.Commit(); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}
	return receipt, nil
}
