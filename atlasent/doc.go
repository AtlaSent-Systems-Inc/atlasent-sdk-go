/*
Package atlasent is the Go SDK for AtlaSent execution-time authorization.

AtlaSent is a Policy Decision Point (PDP): your application asks the
PDP whether a principal is allowed to perform an action on a resource,
right before the action is executed.

# Getting started

Create one Client per process and reuse it:

	client, _ := atlasent.New(os.Getenv("ATLASENT_API_KEY"))

Ask the PDP a single question:

	decision, err := client.Check(ctx, atlasent.CheckRequest{
	    Principal: atlasent.Principal{ID: "user_alice"},
	    Action:    "invoice.pay",
	    Resource:  atlasent.Resource{ID: "inv_42", Type: "invoice"},
	})

Or enforce at the call-site with [Guard], which only runs fn on allow:

	receipt, err := atlasent.Guard(ctx, client, req, func(ctx context.Context) (string, error) {
	    return billing.Pay(ctx, inv.ID)
	})

# Fail-closed by default

When the PDP is unreachable, [Client.Check] returns a deny Decision
alongside a non-nil transport error. Callers must treat err != nil and
!decision.Allowed as "deny"; only flip this with [WithFailOpen] when
availability outranks correctness for the call-site.

# Production knobs

The SDK composes a small set of orthogonal [Option]s:

  - [WithCache]           — local LRU of Decisions; PDP TTL hints honored.
  - [WithRetry]           — exponential backoff; respects Retry-After.
  - [WithObserver]        — metrics/logs/traces via [Observer].
  - [WithCircuitBreaker]  — fail-fast on a sustained-down PDP.
  - [WithContextEnricher] — auto-merge request/trace IDs into every Check.
  - [WithFailOpen]        — swap fail-closed for fail-open. Use with care.

# Enforcement shapes

Beyond [Client.Check] the SDK ships:

  - [Guard]                — generic wrapper for a sensitive function.
  - [Client.HTTPMiddleware] — gates net/http handlers (chi works as-is).
  - [Client.CheckMany]      — N-in-one-round-trip for list endpoints.
  - [Client.CheckAny]       — first allowed in N candidates.
  - [Client.CheckAll]       — all must allow, or [*DeniedError].

Per-framework middleware lives in separate submodules so their deps
stay off the core SDK's graph:

	github.com/atlasent-systems-inc/atlasent-sdk-go/grpc
	github.com/atlasent-systems-inc/atlasent-sdk-go/connectrpc
	github.com/atlasent-systems-inc/atlasent-sdk-go/middleware/gin
	github.com/atlasent-systems-inc/atlasent-sdk-go/middleware/echo
	github.com/atlasent-systems-inc/atlasent-sdk-go/middleware/fiber

# Obligations

A [Decision] can carry obligations the caller MUST honor
(e.g. "redact:ssn", "log:high-risk"). Register handlers at startup:

	reg := atlasent.NewObligationRegistry()
	reg.Register("redact:ssn", redactHandler)
	// ... then after every allow:
	if err := reg.Apply(ctx, decision); err != nil { return err }

Unknown obligations are errors by default — the PDP can't silently
demand a side-effect your code did not wire.

# Errors

Transport / protocol failures surface as [*APIError] with an
[ErrorKind]:

	switch {
	case atlasent.IsTransport(err):    // network
	case atlasent.IsUnauthorized(err): // bad API key
	case atlasent.IsRateLimit(err):    // back off
	case atlasent.IsValidation(err):   // malformed CheckRequest
	}

Policy denials come back as [*DeniedError] wrapping the [Decision].

# Testing consumers

The atlasenttest submodule spins up an httptest-backed fake PDP:

	fake := atlasenttest.NewServer(t)
	fake.On("invoice.pay").Allow()
	client, _ := atlasent.New("test", atlasent.WithBaseURL(fake.URL))

Your code runs against the real Client, so retry, cache, and observer
paths are exercised end-to-end.

# Version

The SDK release string is [Version] ("0.3.0"); the default HTTP
User-Agent is "atlasent-sdk-go/" + Version.
*/
package atlasent
