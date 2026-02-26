package observe

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// StartRelaySpan starts a root span for a relay request.
func (o *Observer) StartRelaySpan(ctx context.Context, model, apiKey string, groupID int) (context.Context, trace.Span) {
	if o == nil || o.tracer == nil {
		return ctx, noopSpan()
	}
	return o.tracer.Start(ctx, "relay",
		trace.WithAttributes(
			attribute.String("wheel.model", model),
			attribute.String("wheel.api_key", apiKey),
			attribute.Int("wheel.group_id", groupID),
		),
	)
}

// StartAttemptSpan starts a child span for a single relay attempt.
func (o *Observer) StartAttemptSpan(ctx context.Context, attemptNum int, channelName string, channelID int) (context.Context, trace.Span) {
	if o == nil || o.tracer == nil {
		return ctx, noopSpan()
	}
	return o.tracer.Start(ctx, fmt.Sprintf("attempt-%d", attemptNum),
		trace.WithAttributes(
			attribute.Int("wheel.attempt", attemptNum),
			attribute.String("wheel.channel_name", channelName),
			attribute.Int("wheel.channel_id", channelID),
		),
	)
}

// EndAttemptSpan ends an attempt span with status and optional error.
func (o *Observer) EndAttemptSpan(span trace.Span, statusCode int, durationMs int, err error) {
	if o == nil {
		return
	}
	span.SetAttributes(
		attribute.Int("http.status_code", statusCode),
		attribute.Int("wheel.duration_ms", durationMs),
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

// StartPluginSpan starts a child span for plugin execution.
func (o *Observer) StartPluginSpan(ctx context.Context, pluginName, phase string) (context.Context, trace.Span) {
	if o == nil || o.tracer == nil {
		return ctx, noopSpan()
	}
	return o.tracer.Start(ctx, fmt.Sprintf("plugin/%s/%s", pluginName, phase),
		trace.WithAttributes(
			attribute.String("wheel.plugin", pluginName),
			attribute.String("wheel.plugin_phase", phase),
		),
	)
}

// StartMultimodalSpan starts a child span for a multimodal API request.
func (o *Observer) StartMultimodalSpan(ctx context.Context, requestType, model string) (context.Context, trace.Span) {
	if o == nil || o.tracer == nil {
		return ctx, noopSpan()
	}
	return o.tracer.Start(ctx, fmt.Sprintf("multimodal/%s", requestType),
		trace.WithAttributes(
			attribute.String("wheel.request_type", requestType),
			attribute.String("wheel.model", model),
		),
	)
}

// StartUpstreamSpan starts a child span for the upstream HTTP call.
func (o *Observer) StartUpstreamSpan(ctx context.Context, url, method string) (context.Context, trace.Span) {
	if o == nil || o.tracer == nil {
		return ctx, noopSpan()
	}
	return o.tracer.Start(ctx, "upstream",
		trace.WithAttributes(
			attribute.String("http.url", url),
			attribute.String("http.method", method),
		),
	)
}

// EndSpan ends a span with optional error.
func (o *Observer) EndSpan(span trace.Span, err error) {
	if o == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

// AddCircuitBreakerEvent adds a span event when a circuit breaker trips.
func (o *Observer) AddCircuitBreakerEvent(ctx context.Context, channelName string, channelID int) {
	if o == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	span.AddEvent("circuit_breaker_tripped", trace.WithAttributes(
		attribute.String("wheel.channel_name", channelName),
		attribute.Int("wheel.channel_id", channelID),
	))
}

// AddCacheEvent adds a span event for cache hit/miss.
func (o *Observer) AddCacheEvent(ctx context.Context, hit bool, model string) {
	if o == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	eventName := "cache_miss"
	if hit {
		eventName = "cache_hit"
	}
	span.AddEvent(eventName, trace.WithAttributes(
		attribute.String("wheel.model", model),
	))
}

// noopSpan returns a non-recording span for disabled tracing.
func noopSpan() trace.Span {
	return trace.SpanFromContext(context.Background())
}
