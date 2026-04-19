// Package atlasentconnect provides a connectrpc.com/connect interceptor
// that gates RPCs with an AtlaSent authorization check. It mirrors the
// gRPC interceptor in github.com/atlasent-systems-inc/atlasent-sdk-go/grpc.
//
//	interceptor := atlasentconnect.NewInterceptor(client, resolve)
//	path, handler := billingv1connect.NewInvoicesServiceHandler(
//	    svc,
//	    connect.WithInterceptors(interceptor),
//	)
//
// The Principal must be placed on the request context by an upstream
// interceptor (JWT, mTLS, etc.) via atlasent.WithPrincipal.
package atlasentconnect

import (
	"context"

	"connectrpc.com/connect"
	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
)

// ResourceResolver maps a Connect request to an (action, resource, reqCtx)
// triple. procedure is the fully qualified procedure, e.g.
// "/billing.v1.InvoicesService/Pay".
type ResourceResolver func(
	ctx context.Context,
	procedure string,
	req connect.AnyRequest,
) (action string, resource atlasent.Resource, reqCtx map[string]any, err error)

// NewInterceptor returns a connect.Interceptor that authorizes unary and
// streaming RPCs before calling the real handler.
func NewInterceptor(c *atlasent.Client, resolve ResourceResolver) connect.Interceptor {
	return &interceptor{client: c, resolve: resolve}
}

type interceptor struct {
	client  *atlasent.Client
	resolve ResourceResolver
}

// WrapUnary implements connect.Interceptor.
func (i *interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if err := i.authorize(ctx, req.Spec().Procedure, req); err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

// WrapStreamingClient is a no-op: client-side interceptors don't gate
// server-side authorization. Callers who want client-side hooks should
// wire their own interceptor.
func (i *interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// WrapStreamingHandler authorizes the stream once at open using a nil
// request message.
func (i *interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if err := i.authorize(ctx, conn.Spec().Procedure, nil); err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (i *interceptor) authorize(ctx context.Context, procedure string, req connect.AnyRequest) error {
	principal, ok := atlasent.PrincipalFrom(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, errMissingPrincipal)
	}
	action, resource, reqCtx, err := i.resolve(ctx, procedure, req)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	decision, checkErr := i.client.Check(ctx, atlasent.CheckRequest{
		Principal: principal,
		Action:    action,
		Resource:  resource,
		Context:   reqCtx,
	})
	if checkErr != nil && i.client.FailClosed {
		return connect.NewError(connect.CodeUnavailable, errPDPUnavailable)
	}
	if !decision.Allowed {
		if decision.Reason == "" {
			return connect.NewError(connect.CodePermissionDenied, errDenied)
		}
		return connect.NewError(connect.CodePermissionDenied, &denialErr{reason: decision.Reason})
	}
	return nil
}

type simpleErr string

func (e simpleErr) Error() string { return string(e) }

const (
	errMissingPrincipal = simpleErr("missing principal")
	errPDPUnavailable   = simpleErr("authorization service unavailable")
	errDenied           = simpleErr("denied")
)

type denialErr struct{ reason string }

func (e *denialErr) Error() string { return e.reason }
