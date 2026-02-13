package observe

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

// RecordRequest records a completed relay request.
func (o *Observer) RecordRequest(ctx context.Context, channel, model, apiKey string, statusCode int) {
	if o == nil {
		return
	}
	o.requestsTotal.Add(ctx, 1, otelmetric.WithAttributes(
		attribute.String("channel", channel),
		attribute.String("model", model),
		attribute.String("api_key", apiKey),
		attribute.Int("status_code", statusCode),
	))
}

// RecordError records a relay error.
func (o *Observer) RecordError(ctx context.Context, channel, model, errorType string) {
	if o == nil {
		return
	}
	o.errorsTotal.Add(ctx, 1, otelmetric.WithAttributes(
		attribute.String("channel", channel),
		attribute.String("model", model),
		attribute.String("error_type", errorType),
	))
}

// RecordDuration records the total request duration.
func (o *Observer) RecordDuration(ctx context.Context, channel, model string, d time.Duration) {
	if o == nil {
		return
	}
	o.durationSeconds.Record(ctx, d.Seconds(), otelmetric.WithAttributes(
		attribute.String("channel", channel),
		attribute.String("model", model),
	))
}

// RecordTTFB records time-to-first-byte for streaming requests.
func (o *Observer) RecordTTFB(ctx context.Context, channel, model string, ms int) {
	if o == nil {
		return
	}
	o.ttfbSeconds.Record(ctx, float64(ms)/1000.0, otelmetric.WithAttributes(
		attribute.String("channel", channel),
		attribute.String("model", model),
	))
}

// RecordRetry records a retry attempt.
func (o *Observer) RecordRetry(ctx context.Context, channel, model string) {
	if o == nil {
		return
	}
	o.retriesTotal.Add(ctx, 1, otelmetric.WithAttributes(
		attribute.String("channel", channel),
		attribute.String("model", model),
	))
}

// RecordTokens records token usage.
func (o *Observer) RecordTokens(ctx context.Context, channel, model, direction string, count int) {
	if o == nil {
		return
	}
	o.tokensTotal.Add(ctx, int64(count), otelmetric.WithAttributes(
		attribute.String("channel", channel),
		attribute.String("model", model),
		attribute.String("direction", direction),
	))
}

// RecordCost records the dollar cost of a request.
func (o *Observer) RecordCost(ctx context.Context, channel, model string, cost float64) {
	if o == nil {
		return
	}
	o.costTotal.Add(ctx, cost, otelmetric.WithAttributes(
		attribute.String("channel", channel),
		attribute.String("model", model),
	))
}

// SetCircuitBreakerState sets the circuit breaker gauge (1=open, 0=closed).
func (o *Observer) SetCircuitBreakerState(ctx context.Context, channel string, delta int64) {
	if o == nil {
		return
	}
	o.circuitBreakerState.Add(ctx, delta, otelmetric.WithAttributes(
		attribute.String("channel", channel),
	))
}

// StreamStarted increments the active streams gauge.
func (o *Observer) StreamStarted(ctx context.Context) {
	if o == nil {
		return
	}
	o.activeStreams.Add(ctx, 1)
}

// StreamEnded decrements the active streams gauge.
func (o *Observer) StreamEnded(ctx context.Context) {
	if o == nil {
		return
	}
	o.activeStreams.Add(ctx, -1)
}

// RecordLogDrop increments the log drops counter.
func (o *Observer) RecordLogDrop(ctx context.Context) {
	if o == nil {
		return
	}
	o.logDropsTotal.Add(ctx, 1)
}
