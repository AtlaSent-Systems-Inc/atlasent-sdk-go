// Package atlasentotel is an OpenTelemetry Observer for the AtlaSent SDK.
//
// Each Check call emits:
//   - a counter `atlasent.check.count` with attributes
//     {allowed, cache_hit, error_kind}
//   - a histogram `atlasent.check.duration_ms`
//   - a span named "atlasent.check" as a child of the caller's span,
//     with timestamps reconstructed from the recorded latency (so the
//     span wraps the real wall-clock call even though the observer runs
//     after it completes)
//
// Wire it on the Client:
//
//	obs, _ := atlasentotel.NewObserver(otel.Meter("my-app"), otel.Tracer("my-app"))
//	client, _ := atlasent.New(apiKey, atlasent.WithObserver(obs))
package atlasentotel

import (
	"context"
	"fmt"
	"time"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Observer implements atlasent.Observer by emitting OpenTelemetry metrics
// and spans.
type Observer struct {
	meter    metric.Meter
	tracer   trace.Tracer
	counter  metric.Int64Counter
	duration metric.Float64Histogram
}

// NewObserver constructs an Observer. Either meter or tracer may be nil to
// skip that signal.
func NewObserver(meter metric.Meter, tracer trace.Tracer) (*Observer, error) {
	o := &Observer{meter: meter, tracer: tracer}
	if meter != nil {
		c, err := meter.Int64Counter("atlasent.check.count",
			metric.WithDescription("Number of AtlaSent authorization checks"))
		if err != nil {
			return nil, fmt.Errorf("atlasentotel: counter: %w", err)
		}
		o.counter = c
		h, err := meter.Float64Histogram("atlasent.check.duration_ms",
			metric.WithDescription("Duration of AtlaSent checks"),
			metric.WithUnit("ms"))
		if err != nil {
			return nil, fmt.Errorf("atlasentotel: histogram: %w", err)
		}
		o.duration = h
	}
	return o, nil
}

// OnCheck implements atlasent.Observer.
func (o *Observer) OnCheck(ctx context.Context, ev atlasent.CheckEvent) {
	attrs := []attribute.KeyValue{
		attribute.Bool("allowed", ev.Decision.Allowed),
		attribute.Bool("cache_hit", ev.CacheHit),
		attribute.String("action", ev.Request.Action),
		attribute.String("resource_type", ev.Request.Resource.Type),
	}
	if ev.Err != nil {
		var apiErr *atlasent.APIError
		kind := "unknown"
		if ok := asAPIError(ev.Err, &apiErr); ok {
			kind = apiErr.Kind.String()
		}
		attrs = append(attrs, attribute.String("error_kind", kind))
	}

	if o.counter != nil {
		o.counter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	if o.duration != nil && !ev.CacheHit {
		o.duration.Record(ctx, float64(ev.Latency)/float64(time.Millisecond),
			metric.WithAttributes(attrs...))
	}
	if o.tracer != nil && !ev.CacheHit {
		end := time.Now()
		start := end.Add(-ev.Latency)
		_, span := o.tracer.Start(ctx, "atlasent.check",
			trace.WithTimestamp(start),
			trace.WithAttributes(attrs...))
		if ev.Err != nil {
			span.RecordError(ev.Err)
			span.SetStatus(codes.Error, ev.Err.Error())
		}
		span.End(trace.WithTimestamp(end))
	}
}

// asAPIError is a thin wrapper to avoid importing errors in this file's
// hot path; the compiler inlines errors.As anyway.
func asAPIError(err error, target **atlasent.APIError) bool {
	for err != nil {
		if ae, ok := err.(*atlasent.APIError); ok {
			*target = ae
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
