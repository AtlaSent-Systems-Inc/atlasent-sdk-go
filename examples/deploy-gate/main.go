// deploy-gate demonstrates how to gate a deploy pipeline on an AtlaSent
// decision. A CI runner asks "may deploy_bot push service X to env Y?" and
// honors the PDP's answer. High-risk environments get an extra permit
// lifecycle: evaluate → verify → consume.
//
// Run:
//
//	ATLASENT_API_KEY=... \
//	DEPLOY_SERVICE=payments \
//	DEPLOY_ENV=production \
//	go run ./examples/deploy-gate
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

func main() {
	apiKey := os.Getenv("ATLASENT_API_KEY")
	if apiKey == "" {
		log.Fatal("set ATLASENT_API_KEY")
	}
	service := envOr("DEPLOY_SERVICE", "payments")
	env := envOr("DEPLOY_ENV", "staging")

	opts := []atlasent.Option{
		atlasent.WithRetry(atlasent.DefaultRetryPolicy),
	}
	if base := os.Getenv("ATLASENT_BASE_URL"); base != "" {
		opts = append(opts, atlasent.WithBaseURL(base))
	}
	client, err := atlasent.New(apiKey, opts...)
	if err != nil {
		log.Fatalf("init: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req := atlasent.CheckRequest{
		Principal: atlasent.Principal{ID: "deploy_bot", Type: "service"},
		Action:    "service.deploy",
		Resource:  atlasent.Resource{ID: service, Type: "service", Attributes: map[string]any{"env": env}},
		Context: map[string]any{
			"commit_sha":    envOr("GITHUB_SHA", "dev"),
			"actor":         envOr("GITHUB_ACTOR", "local"),
			"run_id":        envOr("GITHUB_RUN_ID", ""),
			"change_window": "weekday-business-hours",
		},
	}

	// Production deploys require a full permit lifecycle so we have a
	// single-use artefact to attach to the audit record.
	if env == "production" {
		result, err := client.Evaluate(ctx, atlasent.EvaluationPayload{
			Actor:   atlasent.Actor{ID: "deploy_bot", Type: "service"},
			Action:  atlasent.Action{Type: "service.deploy"},
			Target:  atlasent.Target{ID: service, Type: "service", Environment: env},
			Context: req.Context,
		})
		if err != nil {
			log.Fatalf("evaluate: %v", err)
		}
		switch result.Outcome {
		case atlasent.OutcomeAllow:
			if result.PermitID == "" {
				log.Fatal("prod deploy allowed but no permit issued")
			}
			if _, err := client.ConsumePermit(ctx, result.PermitID); err != nil {
				log.Fatalf("consume permit: %v", err)
			}
			fmt.Printf("DEPLOY APPROVED — permit %s consumed\n", result.PermitID)
		case atlasent.OutcomeRequireApproval:
			fmt.Printf("DEPLOY PAUSED — human approval required\n")
			os.Exit(2)
		default:
			fmt.Printf("DEPLOY BLOCKED — risk=%s\n", result.Risk.Level)
			os.Exit(1)
		}
		return
	}

	// Non-prod: single Guard call runs the deploy step inline.
	_, err = atlasent.Guard(ctx, client, req, func(ctx context.Context) (struct{}, error) {
		fmt.Printf("deploying %s to %s...\n", service, env)
		return struct{}{}, nil
	})
	var denied *atlasent.DeniedError
	if errors.As(err, &denied) {
		fmt.Printf("DEPLOY BLOCKED — %s\n", denied.Decision.Reason)
		os.Exit(1)
	}
	if err != nil {
		log.Fatalf("guard: %v", err)
	}
	fmt.Println("DEPLOY APPROVED")
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
