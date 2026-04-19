package atlasentotel

import (
	"context"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
	"go.opentelemetry.io/otel/trace"
)

// TraceIDEnricher returns an atlasent.ContextEnricher that copies the
// current OpenTelemetry trace + span ID into CheckRequest.Context so
// authorization audit records join the same trace as the request that
// triggered them.
//
// Emitted keys:
//
//	trace_id  — 32-char hex, or absent if no active trace
//	span_id   — 16-char hex, or absent
//
// Wire it alongside the main SDK:
//
//	client, _ := atlasent.New(apiKey,
//	    atlasent.WithContextEnricher(atlasent.ChainEnrichers(
//	        atlasent.RequestIDEnricher(),
//	        atlasentotel.TraceIDEnricher(),
//	    )))
func TraceIDEnricher() atlasent.ContextEnricher {
	return func(ctx context.Context) map[string]any {
		span := trace.SpanFromContext(ctx)
		sc := span.SpanContext()
		if !sc.IsValid() {
			return nil
		}
		return map[string]any{
			"trace_id": sc.TraceID().String(),
			"span_id":  sc.SpanID().String(),
		}
	}
}
