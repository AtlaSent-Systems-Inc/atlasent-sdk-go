// Package atlasent is the AtlaSent Go SDK.
//
// AtlaSent is a centralized policy decision point (PDP). Your code asks
// "may Principal X do Action Y on Resource Z?", the PDP answers, and your
// code enforces. Policy lives server-side; the SDK is only a thin HTTP
// client with ergonomics for the two hot code paths: the authorization
// call-site and the middleware/interceptor layer.
//
// # Quick start
//
//	client, err := atlasent.New(os.Getenv("ATLASENT_API_KEY"),
//	    atlasent.WithCache(atlasent.NewMemoryCache(1024), 30*time.Second),
//	    atlasent.WithRetry(atlasent.DefaultRetryPolicy),
//	)
//	if err != nil { log.Fatal(err) }
//
//	decision, err := client.Check(ctx, atlasent.CheckRequest{
//	    Principal: atlasent.Principal{ID: "user_alice", Type: "user"},
//	    Action:    "invoice.pay",
//	    Resource:  atlasent.Resource{ID: "inv_42", Type: "invoice"},
//	    Context:   map[string]any{"amount_cents": 12_00},
//	})
//	if err != nil { /* transport failure: decision honors FailClosed */ }
//	if !decision.Allowed { /* surface decision.Reason to the user */ }
//
// Prefer Guard over hand-rolled if-blocks; it colocates the PDP call and
// the side-effect so they cannot drift:
//
//	invoice, err := atlasent.Guard(ctx, client, req, func(ctx context.Context) (*Invoice, error) {
//	    return payInvoice(ctx, "inv_42")
//	})
//
// # Defaults
//
// Transport failures fail-closed: the returned Decision denies. Flip with
// WithFailOpen only where availability outranks correctness. Obligations
// on the Decision ("log:high-risk", "redact:ssn", ...) are contract —
// ignore them at your own risk.
//
// # Cross-SDK contract
//
// The Python and TypeScript SDKs speak a v1 wire format documented in
// atlasent-sdk/contract/. Use LegacyClient (wrapped around an ordinary
// Client) when talking to a v1-compatible endpoint. Client.Evaluate
// targets the v2 OpenAPI surface. Tests in contract_test.go replay the
// cross-SDK vectors so drift is caught in CI.
//
// # Related packages
//
//   - grpc      — unary and stream server interceptors
//   - examples/ — deploy-gate, LIMS write, batch release, quickstart
//
// # Stability
//
// v0.x is a 0.x release: minor versions may break. Pin exact versions in
// go.mod until the v1 stability announcement.
package atlasent
