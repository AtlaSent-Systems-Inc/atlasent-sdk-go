package atlasentconnect_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
	atlasentconnect "github.com/atlasent-systems-inc/atlasent-sdk-go/connectrpc"
)

func newClient(t *testing.T, allow bool, reason string) *atlasent.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(atlasent.Decision{Allowed: allow, Reason: reason})
	}))
	t.Cleanup(srv.Close)
	c, _ := atlasent.New("k", atlasent.WithBaseURL(srv.URL))
	return c
}

type fakeReq struct {
	connect.Request[struct{}]
}

func (f *fakeReq) Spec() connect.Spec {
	return connect.Spec{Procedure: "/pkg.Svc/Method"}
}

func resolve(_ context.Context, procedure string, _ connect.AnyRequest) (string, atlasent.Resource, map[string]any, error) {
	return "read", atlasent.Resource{ID: "r", Type: "doc"}, nil, nil
}

// invokeUnary builds a UnaryFunc via the interceptor and runs it with ctx.
func invokeUnary(t *testing.T, c *atlasent.Client, ctx context.Context) error {
	t.Helper()
	ic := atlasentconnect.NewInterceptor(c, resolve)
	inner := connect.UnaryFunc(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, nil
	})
	wrapped := ic.WrapUnary(inner)
	_, err := wrapped(ctx, &fakeReq{})
	return err
}

func TestConnectUnauthenticated(t *testing.T) {
	c := newClient(t, true, "")
	err := invokeUnary(t, c, context.Background())
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

func TestConnectPermissionDenied(t *testing.T) {
	c := newClient(t, false, "not owner")
	ctx := atlasent.WithPrincipal(context.Background(), atlasent.Principal{ID: "u"})
	err := invokeUnary(t, c, ctx)
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("want PermissionDenied, got %v", err)
	}
}

func TestConnectAllow(t *testing.T) {
	c := newClient(t, true, "")
	ctx := atlasent.WithPrincipal(context.Background(), atlasent.Principal{ID: "u"})
	if err := invokeUnary(t, c, ctx); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestConnectPDPUnavailable(t *testing.T) {
	c, _ := atlasent.New("k", atlasent.WithBaseURL("http://127.0.0.1:1"))
	ctx := atlasent.WithPrincipal(context.Background(), atlasent.Principal{ID: "u"})
	err := invokeUnary(t, c, ctx)
	if connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("want Unavailable, got %v", err)
	}
}
