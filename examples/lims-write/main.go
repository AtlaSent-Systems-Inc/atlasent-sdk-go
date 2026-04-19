// lims-write shows a GxP-regulated LIMS write gated by an AtlaSent
// decision that carries obligations. The obligation contract is:
//
//	redact:phi   — scrub PHI from any logs the write emits
//	log:gxp      — emit a structured audit entry alongside the write
//	cosign:batch — require a second-person sign-off before commit
//
// The SDK does not know what these mean; the application MUST honor them.
// Unknown obligations fail-closed to be safe.
//
// Run:
//
//	ATLASENT_API_KEY=... go run ./examples/lims-write
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
	client, err := atlasent.New(apiKey, atlasent.WithRetry(atlasent.DefaultRetryPolicy))
	if err != nil {
		log.Fatalf("init: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req := atlasent.CheckRequest{
		Principal: atlasent.Principal{
			ID:    "user_smith",
			Type:  "user",
			Tags:  map[string]string{"gxp_role": "operator"},
		},
		Action: "lims.record.modify",
		Resource: atlasent.Resource{
			ID:   "PT-001",
			Type: "clinical_record",
			Attributes: map[string]any{
				"study":        "STUDY-42",
				"record_state": "locked",
			},
		},
		Context: map[string]any{
			"change_reason":     "Correct date-of-birth typo",
			"requires_cosign":   true,
			"site_code":         "SITE-EU-1",
			"regulatory_bucket": "21-CFR-11",
		},
	}

	// Known obligation set: everything else blocks the write.
	known := map[string]bool{"redact:phi": true, "log:gxp": true, "cosign:batch": true}

	_, err = atlasent.Guard(ctx, client, req, func(ctx context.Context) (struct{}, error) {
		dec, _ := client.Check(ctx, req) // cache hit — Guard already fetched

		for _, o := range dec.Obligations {
			if !known[o] {
				return struct{}{}, fmt.Errorf("unknown obligation %q; refusing to proceed", o)
			}
		}
		if dec.HasObligation("cosign:batch") {
			if err := requestCosign(ctx); err != nil {
				return struct{}{}, fmt.Errorf("cosign: %w", err)
			}
		}
		if dec.HasObligation("log:gxp") {
			fmt.Printf("[GXP-AUDIT] user=%s action=%s record=%s policy=%s\n",
				req.Principal.ID, req.Action, req.Resource.ID, dec.PolicyID)
		}
		payload := applyRedactions(req.Context, dec.HasObligation("redact:phi"))
		fmt.Printf("writing record %s → %+v\n", req.Resource.ID, payload)
		return struct{}{}, nil
	})

	var denied *atlasent.DeniedError
	if errors.As(err, &denied) {
		fmt.Printf("WRITE BLOCKED — %s (policy=%s)\n", denied.Decision.Reason, denied.Decision.PolicyID)
		os.Exit(1)
	}
	if err != nil {
		log.Fatalf("write: %v", err)
	}
	fmt.Println("WRITE COMMITTED")
}

// requestCosign stands in for a real second-person sign-off. In production
// this would suspend the write and hand off to an approval workflow.
func requestCosign(_ context.Context) error {
	fmt.Println("[COSIGN] awaiting second-person sign-off (simulated)")
	return nil
}

func applyRedactions(in map[string]any, redactPHI bool) map[string]any {
	if !redactPHI {
		return in
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if k == "patient_name" || k == "dob" || k == "ssn" {
			out[k] = "<redacted>"
			continue
		}
		out[k] = v
	}
	return out
}
