// grpc example demonstrates gating RPCs with AtlaSent. It wires the unary
// and stream server interceptors onto a bare gRPC server and shows the
// three pieces you have to provide yourself:
//
//  1. An upstream auth interceptor that extracts the Principal from
//     request metadata (JWT, mTLS, session) and stamps it on the context.
//  2. A ResourceResolver that maps the RPC's fullMethod + request message
//     to an (action, resource) pair.
//  3. A concrete service implementation (omitted — this example registers
//     no service; it just proves the interceptors compile and compose).
package main

import (
	"context"
	"log"
	"net"
	"os"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
	atlasentgrpc "github.com/atlasent-systems-inc/atlasent-sdk-go/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// principalAuth is a stand-in auth interceptor. In production, verify a JWT
// from incoming metadata and derive the Principal from its claims.
func principalAuth(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	userIDs := md.Get("x-user-id")
	if len(userIDs) == 0 {
		return nil, status.Error(codes.Unauthenticated, "no user id")
	}
	ctx = atlasent.WithPrincipal(ctx, atlasent.Principal{ID: userIDs[0], Type: "user"})
	return handler(ctx, req)
}

// resolve maps gRPC method names to PDP actions and resources. Extend the
// switch as you add services.
func resolve(_ context.Context, fullMethod string, _ any) (string, atlasent.Resource, map[string]any, error) {
	switch fullMethod {
	case "/billing.Invoices/Pay":
		return "invoice.pay", atlasent.Resource{Type: "invoice"}, nil, nil
	case "/billing.Invoices/Read":
		return "invoice.read", atlasent.Resource{Type: "invoice"}, nil, nil
	default:
		return "rpc.call", atlasent.Resource{ID: fullMethod, Type: "rpc"}, nil, nil
	}
}

func main() {
	apiKey := os.Getenv("ATLASENT_API_KEY")
	if apiKey == "" {
		log.Fatal("set ATLASENT_API_KEY")
	}
	opts := []atlasent.Option{}
	if base := os.Getenv("ATLASENT_BASE_URL"); base != "" {
		opts = append(opts, atlasent.WithBaseURL(base))
	}
	client, err := atlasent.New(apiKey, opts...)
	if err != nil {
		log.Fatalf("atlasent: %v", err)
	}

	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			principalAuth,
			atlasentgrpc.UnaryServerInterceptor(client, resolve),
		),
		grpc.StreamInterceptor(atlasentgrpc.StreamServerInterceptor(client, resolve)),
	)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":50051"
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	log.Printf("listening on %s (register your service and run)", addr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
