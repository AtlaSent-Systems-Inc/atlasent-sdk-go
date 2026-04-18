// Package grpc provides gRPC server interceptors that gate RPCs with an
// AtlaSent authorization check.
//
// Wire them like any other interceptor:
//
//	s := grpc.NewServer(
//	    grpc.UnaryInterceptor(atlasentgrpc.UnaryServerInterceptor(client, resolve)),
//	    grpc.StreamInterceptor(atlasentgrpc.StreamServerInterceptor(client, resolve)),
//	)
//
// The Principal must be set on the incoming context by an upstream interceptor
// (JWT/mTLS/header auth) via atlasent.WithPrincipal.
package atlasentgrpc

import (
	"context"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ResourceResolver maps an incoming RPC to the (action, resource, context)
// triple needed by the PDP. fullMethod is of the form "/pkg.Service/Method".
// req is the unmarshaled request message (nil for streams).
type ResourceResolver func(
	ctx context.Context,
	fullMethod string,
	req any,
) (action string, resource atlasent.Resource, reqCtx map[string]any, err error)

// UnaryServerInterceptor gates unary RPCs with an AtlaSent Check. It returns
// Unauthenticated when no Principal is attached, InvalidArgument when the
// resolver errors, PermissionDenied when the PDP denies, and Unavailable
// when the PDP is unreachable and the client is fail-closed.
func UnaryServerInterceptor(c *atlasent.Client, resolve ResourceResolver) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := authorize(ctx, c, resolve, info.FullMethod, req); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamServerInterceptor gates streaming RPCs. The check fires once at
// stream open using a nil request message.
func StreamServerInterceptor(c *atlasent.Client, resolve ResourceResolver) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := authorize(ss.Context(), c, resolve, info.FullMethod, nil); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func authorize(
	ctx context.Context,
	c *atlasent.Client,
	resolve ResourceResolver,
	fullMethod string,
	req any,
) error {
	principal, ok := atlasent.PrincipalFrom(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing principal")
	}
	action, resource, reqCtx, err := resolve(ctx, fullMethod, req)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "resolve: %v", err)
	}
	decision, checkErr := c.Check(ctx, atlasent.CheckRequest{
		Principal: principal,
		Action:    action,
		Resource:  resource,
		Context:   reqCtx,
	})
	if checkErr != nil && c.FailClosed {
		return status.Error(codes.Unavailable, "authorization service unavailable")
	}
	if !decision.Allowed {
		reason := decision.Reason
		if reason == "" {
			reason = "denied"
		}
		return status.Error(codes.PermissionDenied, reason)
	}
	return nil
}

