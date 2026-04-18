package atlasent_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
)

// allowingServer is a throwaway PDP used only to keep godoc examples
// self-contained and runnable.
func allowingServer(reason string, allow bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(atlasent.Decision{Allowed: allow, Reason: reason, PolicyID: "p"})
	}))
}

// Direct Check: ask the PDP whether a principal may perform an action.
func ExampleClient_Check() {
	srv := allowingServer("", true)
	defer srv.Close()

	client, _ := atlasent.New("test", atlasent.WithBaseURL(srv.URL))
	d, err := client.Check(context.Background(), atlasent.CheckRequest{
		Principal: atlasent.Principal{ID: "user_alice"},
		Action:    "invoice.read",
		Resource:  atlasent.Resource{ID: "inv_42", Type: "invoice"},
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("allowed=%v policy=%s", d.Allowed, d.PolicyID)
	// Output: allowed=true policy=p
}

// Guard wraps a sensitive function so it only runs on allow. A *DeniedError
// wraps the Decision so callers can inspect policy and reason.
func ExampleGuard() {
	srv := allowingServer("not owner", false)
	defer srv.Close()

	client, _ := atlasent.New("test", atlasent.WithBaseURL(srv.URL))
	_, err := atlasent.Guard(context.Background(), client, atlasent.CheckRequest{
		Principal: atlasent.Principal{ID: "user_alice"},
		Action:    "invoice.pay",
		Resource:  atlasent.Resource{ID: "inv_42", Type: "invoice"},
	}, func(ctx context.Context) (string, error) {
		return "paid", nil
	})

	var denied *atlasent.DeniedError
	if errors.As(err, &denied) {
		fmt.Printf("denied: %s", denied.Decision.Reason)
	}
	// Output: denied: not owner
}

// CheckMany amortizes list-endpoint authorization over one round trip.
func ExampleClient_CheckMany() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var br struct {
			Checks []atlasent.CheckRequest `json:"checks"`
		}
		_ = json.NewDecoder(r.Body).Decode(&br)
		decs := make([]atlasent.Decision, len(br.Checks))
		for i, c := range br.Checks {
			decs[i] = atlasent.Decision{Allowed: c.Resource.ID != "inv_secret"}
		}
		_ = json.NewEncoder(w).Encode(struct {
			Decisions []atlasent.Decision `json:"decisions"`
		}{decs})
	}))
	defer srv.Close()

	client, _ := atlasent.New("test", atlasent.WithBaseURL(srv.URL))
	reqs := []atlasent.CheckRequest{
		{Principal: atlasent.Principal{ID: "u"}, Action: "read", Resource: atlasent.Resource{ID: "inv_1", Type: "invoice"}},
		{Principal: atlasent.Principal{ID: "u"}, Action: "read", Resource: atlasent.Resource{ID: "inv_secret", Type: "invoice"}},
	}
	decs, _ := client.CheckMany(context.Background(), reqs)
	fmt.Printf("%v %v", decs[0].Allowed, decs[1].Allowed)
	// Output: true false
}

// Cache hot paths with an LRU that honors the PDP's TTL hint.
func ExampleWithCache() {
	srv := allowingServer("", true)
	defer srv.Close()

	client, _ := atlasent.New("test",
		atlasent.WithBaseURL(srv.URL),
		atlasent.WithCache(atlasent.NewMemoryCache(1024), 5*time.Second),
	)
	_, _ = client.Check(context.Background(), atlasent.CheckRequest{
		Principal: atlasent.Principal{ID: "u"},
		Action:    "read",
		Resource:  atlasent.Resource{ID: "r", Type: "doc"},
	})
	fmt.Println("cached")
	// Output: cached
}

// ObligationRegistry enforces side-effects the PDP demands.
func ExampleObligationRegistry() {
	reg := atlasent.NewObligationRegistry()
	var redacted bool
	reg.Register("redact:ssn", func(_ context.Context, _ string) error {
		redacted = true
		return nil
	})
	err := reg.Apply(context.Background(), atlasent.Decision{
		Allowed:     true,
		Obligations: []string{"redact:ssn"},
	})
	fmt.Printf("err=%v redacted=%v", err, redacted)
	// Output: err=<nil> redacted=true
}
