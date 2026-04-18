package atlasent

import (
	"context"
	"encoding/json"
	"net/http"
)

// principalCtxKey is the type used to stash a Principal on a request context.
type principalCtxKey struct{}

// WithPrincipal returns a child context carrying p. Your auth layer (JWT
// parser, session lookup, mTLS handler) should set this before requests reach
// HTTPMiddleware.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalCtxKey{}, p)
}

// PrincipalFrom extracts a Principal previously stored with WithPrincipal.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalCtxKey{}).(Principal)
	return p, ok
}

// ResourceResolver maps an incoming HTTP request to the (action, resource)
// pair the PDP needs. Returning an error short-circuits with a 400.
type ResourceResolver func(*http.Request) (action string, resource Resource, reqCtx map[string]any, err error)

// HTTPMiddleware gates an http.Handler with an AtlaSent Check. It pulls the
// Principal from the request context, asks resolve for the action and
// resource, and forwards to next only if the Decision allows it.
//
// Responses:
//   - 401 if no Principal is attached.
//   - 400 if resolve returns an error.
//   - 403 with a JSON body {"reason": "...", "policy_id": "..."} when denied.
//   - 503 when fail-closed and the PDP is unreachable.
func (c *Client) HTTPMiddleware(resolve ResourceResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFrom(r.Context())
			if !ok {
				http.Error(w, "unauthenticated", http.StatusUnauthorized)
				return
			}
			action, resource, reqCtx, err := resolve(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			decision, err := c.Check(r.Context(), CheckRequest{
				Principal: principal,
				Action:    action,
				Resource:  resource,
				Context:   reqCtx,
			})
			if err != nil && c.FailClosed {
				http.Error(w, "authorization service unavailable", http.StatusServiceUnavailable)
				return
			}
			if !decision.Allowed {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"reason":    decision.Reason,
					"policy_id": decision.PolicyID,
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
