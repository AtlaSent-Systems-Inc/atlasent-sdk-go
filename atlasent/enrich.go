package atlasent

import "context"

// ContextEnricher returns additional key/value pairs to merge into
// CheckRequest.Context for every Check. Use this to propagate request IDs,
// trace IDs, tenant IDs, or other cross-cutting attributes the PDP needs
// for audit or policy evaluation — without every call-site having to
// remember to wire them.
//
// The returned map may be nil; keys already present on the request are
// preserved (the caller's explicit value wins).
type ContextEnricher func(ctx context.Context) map[string]any

// WithContextEnricher installs an enricher on the Client. Multiple
// enrichers can be composed with ChainEnrichers.
func WithContextEnricher(e ContextEnricher) Option {
	return func(c *Client) { c.enricher = e }
}

// ChainEnrichers runs each enricher in order; later keys lose to earlier
// keys, matching the "caller wins" rule that the Client applies between
// enricher output and the explicit CheckRequest.Context.
func ChainEnrichers(enrichers ...ContextEnricher) ContextEnricher {
	return func(ctx context.Context) map[string]any {
		out := map[string]any{}
		for _, e := range enrichers {
			if e == nil {
				continue
			}
			for k, v := range e(ctx) {
				if _, exists := out[k]; !exists {
					out[k] = v
				}
			}
		}
		return out
	}
}

// requestIDKey is the type used to stash a request ID on a context.
type requestIDKey struct{}

// WithRequestID returns a child context carrying id. Pair with
// RequestIDEnricher to have it automatically merged into every
// CheckRequest.Context as "request_id".
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFrom extracts a request ID previously set with WithRequestID.
func RequestIDFrom(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey{}).(string)
	return id, ok && id != ""
}

// RequestIDEnricher is a ContextEnricher that copies any WithRequestID
// value into CheckRequest.Context as "request_id". Use it in tandem with
// your HTTP middleware so audit logs correlate without every call-site
// wiring the ID by hand.
func RequestIDEnricher() ContextEnricher {
	return func(ctx context.Context) map[string]any {
		if id, ok := RequestIDFrom(ctx); ok {
			return map[string]any{"request_id": id}
		}
		return nil
	}
}

// applyEnricher merges enricher output into req.Context. Caller-supplied
// Context keys take precedence — enrichers provide defaults, not overrides.
// The input request is copied so callers never observe mutation.
func (c *Client) applyEnricher(ctx context.Context, req CheckRequest) CheckRequest {
	if c.enricher == nil {
		return req
	}
	extras := c.enricher(ctx)
	if len(extras) == 0 {
		return req
	}
	merged := make(map[string]any, len(extras)+len(req.Context))
	for k, v := range extras {
		merged[k] = v
	}
	for k, v := range req.Context {
		merged[k] = v
	}
	req.Context = merged
	return req
}
