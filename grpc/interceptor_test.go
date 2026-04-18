package atlasentgrpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newClient(t *testing.T, allow bool, reason string) *atlasent.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(atlasent.Decision{Allowed: allow, Reason: reason, PolicyID: "p"})
	}))
	t.Cleanup(srv.Close)
	c, err := atlasent.New("k", atlasent.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func resolveDoc(ctx context.Context, m string, _ any) (string, atlasent.Resource, map[string]any, error) {
	return "read", atlasent.Resource{ID: "r", Type: "doc"}, nil, nil
}

func TestUnary_Unauthenticated(t *testing.T) {
	c := newClient(t, true, "")
	ic := UnaryServerInterceptor(c, resolveDoc)
	_, err := ic(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/x/Y"}, func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not run")
		return nil, nil
	})
	if code := status.Code(err); code != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", code)
	}
}

func TestUnary_PermissionDenied(t *testing.T) {
	c := newClient(t, false, "not owner")
	ic := UnaryServerInterceptor(c, resolveDoc)
	ctx := atlasent.WithPrincipal(context.Background(), atlasent.Principal{ID: "u"})
	_, err := ic(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/x/Y"}, func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not run")
		return nil, nil
	})
	st, _ := status.FromError(err)
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("want PermissionDenied, got %v", st.Code())
	}
	if st.Message() != "not owner" {
		t.Fatalf("want reason in message, got %q", st.Message())
	}
}

func TestUnary_Allow(t *testing.T) {
	c := newClient(t, true, "")
	ic := UnaryServerInterceptor(c, resolveDoc)
	ctx := atlasent.WithPrincipal(context.Background(), atlasent.Principal{ID: "u"})
	ran := false
	resp, err := ic(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/x/Y"}, func(ctx context.Context, req any) (any, error) {
		ran = true
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !ran || resp != "ok" {
		t.Fatalf("handler did not run: resp=%v ran=%v", resp, ran)
	}
}

func TestUnary_ResolveError(t *testing.T) {
	c := newClient(t, true, "")
	bad := func(ctx context.Context, m string, _ any) (string, atlasent.Resource, map[string]any, error) {
		return "", atlasent.Resource{}, nil, context.DeadlineExceeded
	}
	ic := UnaryServerInterceptor(c, bad)
	ctx := atlasent.WithPrincipal(context.Background(), atlasent.Principal{ID: "u"})
	_, err := ic(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/x/Y"}, func(ctx context.Context, req any) (any, error) {
		return nil, nil
	})
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", code)
	}
}

func TestUnary_PDPUnavailable(t *testing.T) {
	c, _ := atlasent.New("k", atlasent.WithBaseURL("http://127.0.0.1:1"))
	ic := UnaryServerInterceptor(c, resolveDoc)
	ctx := atlasent.WithPrincipal(context.Background(), atlasent.Principal{ID: "u"})
	_, err := ic(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/x/Y"}, func(ctx context.Context, req any) (any, error) {
		return nil, nil
	})
	if code := status.Code(err); code != codes.Unavailable {
		t.Fatalf("want Unavailable, got %v", code)
	}
}
