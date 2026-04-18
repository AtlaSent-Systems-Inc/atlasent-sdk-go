package atlasenttest_test

import (
	"context"
	"testing"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasenttest"
)

func TestFakeAllowDeny(t *testing.T) {
	fake := atlasenttest.NewServer(t)
	fake.On("invoice.read").Allow()
	fake.OnResource("invoice", "secret").Deny("not owner")

	c, _ := atlasent.New("k", atlasent.WithBaseURL(fake.URL))

	got, err := c.Check(context.Background(), atlasent.CheckRequest{
		Principal: atlasent.Principal{ID: "u"},
		Action:    "invoice.read",
		Resource:  atlasent.Resource{ID: "inv_1", Type: "invoice"},
	})
	if err != nil || !got.Allowed {
		t.Fatalf("want allow, got %+v err=%v", got, err)
	}

	got, err = c.Check(context.Background(), atlasent.CheckRequest{
		Principal: atlasent.Principal{ID: "u"},
		Action:    "invoice.read",
		Resource:  atlasent.Resource{ID: "secret", Type: "invoice"},
	})
	if err != nil || got.Allowed {
		t.Fatalf("want deny for secret, got %+v", got)
	}
	if got.Reason != "not owner" {
		t.Fatalf("want reason 'not owner', got %q", got.Reason)
	}
}

func TestFakeRecordsCalls(t *testing.T) {
	fake := atlasenttest.NewServer(t)
	fake.On("x").Allow()
	c, _ := atlasent.New("k", atlasent.WithBaseURL(fake.URL))

	for i := 0; i < 3; i++ {
		_, _ = c.Check(context.Background(), atlasent.CheckRequest{
			Principal: atlasent.Principal{ID: "u"},
			Action:    "x",
			Resource:  atlasent.Resource{ID: "r", Type: "doc"},
		})
	}
	if n := len(fake.Calls()); n != 3 {
		t.Fatalf("want 3 recorded calls, got %d", n)
	}
}
