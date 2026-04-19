// Package bundle is hybrid-mode authorization for the AtlaSent Go SDK:
// pull signed policy snapshots ("bundles") from the PDP on a schedule,
// evaluate CheckRequests locally for single-digit-microsecond decisions,
// and fall through to the remote PDP when the bundle has no opinion.
//
// # Shape
//
// The core interface is [Evaluator]:
//
//	Evaluate(ctx, CheckRequest) (Decision, bool, error)
//
// Returning ok=false means "no opinion — ask the remote PDP". ok=true
// short-circuits the Check. This keeps the bundle authoritative where
// it has an answer and lets the PDP decide the long tail without
// duplicating policy across the wire.
//
// This package ships [Manager], which composes a [Syncer] (bundle
// fetch + signature verify + rotation) with a pluggable
// [PolicyEngine] (the actual decision logic over a bundle payload).
// Callers plug in a Cedar/Rego/CEL engine of their choice; the bundle
// package never evaluates policy itself.
//
// # Wiring
//
//	eng := myCedarEngine                    // implements PolicyEngine
//	sync := bundle.NewHTTPSyncer(bundle.HTTPSyncerConfig{
//	    URL:       "https://api.atlasent.io/v1/bundles/prod",
//	    APIKey:    os.Getenv("ATLASENT_API_KEY"),
//	    PublicKey: verifyKey,              // Ed25519
//	    Interval:  30 * time.Second,
//	})
//	mgr, _ := bundle.NewManager(sync, eng)
//	defer mgr.Close()
//
//	client, _ := atlasent.New(apiKey, atlasent.WithLocalEvaluator(mgr))
//
// # Integrity
//
// [HTTPSyncer] refuses any bundle whose Ed25519 signature doesn't
// verify against the configured public key. There is no "skip signature
// in dev" switch by design — bundles are policy; policy bypass is the
// whole bug class we're avoiding.
package bundle
