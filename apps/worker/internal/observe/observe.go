package observe

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Observer holds OpenTelemetry providers and metric instruments.
// A nil *Observer is safe to use — all recording methods are no-ops.
type Observer struct {
	meterProvider  *sdkmetric.MeterProvider
	tracerProvider *sdktrace.TracerProvider
	promHandler    http.Handler
	tracer         trace.Tracer

	// Metric instruments
	requestsTotal      otelmetric.Int64Counter
	errorsTotal        otelmetric.Int64Counter
	retriesTotal       otelmetric.Int64Counter
	tokensTotal        otelmetric.Int64Counter
	costTotal          otelmetric.Float64Counter
	durationSeconds    otelmetric.Float64Histogram
	ttfbSeconds        otelmetric.Float64Histogram
	circuitBreakerState otelmetric.Int64UpDownCounter
	activeStreams      otelmetric.Int64UpDownCounter
	logDropsTotal     otelmetric.Int64Counter
}

// New creates an Observer based on config flags.
// Returns (nil, nil) when both metrics and tracing are disabled.
func New(metricsEnabled, otelEnabled bool, otelEndpoint, serviceName string) (*Observer, error) {
	if !metricsEnabled && !otelEnabled {
		return nil, nil
	}

	obs := &Observer{}
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(semconv.ServiceName(serviceName)),
		resource.WithFromEnv(),
		resource.WithHost(),
	)
	if err != nil {
		return nil, err
	}

	// Metrics via Prometheus exporter as OTel MeterProvider reader
	if metricsEnabled {
		exporter, err := promexporter.New()
		if err != nil {
			return nil, err
		}
		obs.meterProvider = sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
			sdkmetric.WithReader(exporter),
		)
		obs.promHandler = promhttp.Handler()
	}

	// Tracing via OTLP gRPC
	if otelEnabled {
		traceExporter, err := otlptracegrpc.New(
			context.Background(),
			otlptracegrpc.WithEndpoint(otelEndpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return nil, err
		}
		obs.tracerProvider = sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithBatcher(traceExporter),
		)
		obs.tracer = obs.tracerProvider.Tracer("wheel-gateway")
	}

	// Register metric instruments
	if obs.meterProvider != nil {
		if err := obs.registerMetrics(); err != nil {
			return nil, err
		}
	}

	return obs, nil
}

func (o *Observer) registerMetrics() error {
	meter := o.meterProvider.Meter("wheel-gateway")
	var err error

	o.requestsTotal, err = meter.Int64Counter("wheel_requests_total",
		otelmetric.WithDescription("Total relay requests"))
	if err != nil {
		return err
	}

	o.errorsTotal, err = meter.Int64Counter("wheel_errors_total",
		otelmetric.WithDescription("Total relay errors"))
	if err != nil {
		return err
	}

	o.retriesTotal, err = meter.Int64Counter("wheel_retries_total",
		otelmetric.WithDescription("Total relay retries"))
	if err != nil {
		return err
	}

	o.tokensTotal, err = meter.Int64Counter("wheel_tokens_total",
		otelmetric.WithDescription("Total tokens processed"))
	if err != nil {
		return err
	}

	o.costTotal, err = meter.Float64Counter("wheel_cost_dollars_total",
		otelmetric.WithDescription("Total cost in dollars"))
	if err != nil {
		return err
	}

	o.durationSeconds, err = meter.Float64Histogram("wheel_request_duration_seconds",
		otelmetric.WithDescription("Request duration in seconds"))
	if err != nil {
		return err
	}

	o.ttfbSeconds, err = meter.Float64Histogram("wheel_ttfb_seconds",
		otelmetric.WithDescription("Time to first byte in seconds"))
	if err != nil {
		return err
	}

	o.circuitBreakerState, err = meter.Int64UpDownCounter("wheel_circuit_breaker_state",
		otelmetric.WithDescription("Circuit breaker state (1=open, 0=closed)"))
	if err != nil {
		return err
	}

	o.activeStreams, err = meter.Int64UpDownCounter("wheel_active_streams",
		otelmetric.WithDescription("Currently active streaming connections"))
	if err != nil {
		return err
	}

	o.logDropsTotal, err = meter.Int64Counter("wheel_log_drops_total",
		otelmetric.WithDescription("Total log entries dropped due to full buffer"))
	if err != nil {
		return err
	}

	return nil
}

// MetricsHandler returns the Prometheus HTTP handler, or nil if metrics are disabled.
func (o *Observer) MetricsHandler() http.Handler {
	if o == nil {
		return nil
	}
	return o.promHandler
}

// Shutdown gracefully shuts down providers.
func (o *Observer) Shutdown(ctx context.Context) {
	if o == nil {
		return
	}
	if o.meterProvider != nil {
		_ = o.meterProvider.Shutdown(ctx)
	}
	if o.tracerProvider != nil {
		_ = o.tracerProvider.Shutdown(ctx)
	}
}
