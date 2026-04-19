package atlasentotel_test

import (
	"context"
	"testing"

	atlasentotel "github.com/atlasent-systems-inc/atlasent-sdk-go/otel"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestTraceIDEnricherNoSpan(t *testing.T) {
	// Bare context has no span; enricher should return nil.
	if got := atlasentotel.TraceIDEnricher()(context.Background()); got != nil {
		t.Fatalf("want nil, got %v", got)
	}
}

func TestTraceIDEnricherWithSpan(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	ctx, span := tracer.Start(context.Background(), "op")
	defer span.End()
	// noop spans have invalid contexts, so this should still be nil —
	// asserts that we don't emit garbage from a no-op tracer.
	if got := atlasentotel.TraceIDEnricher()(ctx); got != nil {
		t.Fatalf("noop tracer span should not emit IDs, got %v", got)
	}
}

func TestTraceIDEnricherValidSpanContext(t *testing.T) {
	// Synthesize a valid SpanContext and attach it to ctx directly.
	traceID, _ := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	spanID, _ := trace.SpanIDFromHex("0123456789abcdef")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	got := atlasentotel.TraceIDEnricher()(ctx)
	if got["trace_id"] != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("wrong trace_id: %v", got)
	}
	if got["span_id"] != "0123456789abcdef" {
		t.Fatalf("wrong span_id: %v", got)
	}
}
