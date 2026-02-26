package observe

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
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
	requestsTotal       otelmetric.Int64Counter
	errorsTotal         otelmetric.Int64Counter
	retriesTotal        otelmetric.Int64Counter
	tokensTotal         otelmetric.Int64Counter
	costTotal           otelmetric.Float64Counter
	durationSeconds     otelmetric.Float64Histogram
	ttfbSeconds         otelmetric.Float64Histogram
	circuitBreakerState otelmetric.Int64UpDownCounter
	activeStreams       otelmetric.Int64UpDownCounter
	logDropsTotal       otelmetric.Int64Counter

	// New: plugin & multimodal metrics
	cacheHitsTotal     otelmetric.Int64Counter
	cacheMissesTotal   otelmetric.Int64Counter
	contentFilterTotal otelmetric.Int64Counter
	rateLimitHitsTotal otelmetric.Int64Counter
	multimodalTotal    otelmetric.Int64Counter
	pluginDuration     otelmetric.Float64Histogram
}

// OtelConfig holds all observability configuration.
type OtelConfig struct {
	MetricsEnabled      bool
	OtelEnabled         bool
	OtelEndpoint        string
	ServiceName         string
	OtelMetricsPush     bool
	OtelMetricsEndpoint string
}

// New creates an Observer based on config flags.
// Returns (nil, nil) when both metrics and tracing are disabled.
func New(metricsEnabled, otelEnabled bool, otelEndpoint, serviceName string) (*Observer, error) {
	return NewWithConfig(OtelConfig{
		MetricsEnabled: metricsEnabled,
		OtelEnabled:    otelEnabled,
		OtelEndpoint:   otelEndpoint,
		ServiceName:    serviceName,
	})
}

// NewWithConfig creates an Observer with full configuration including OTLP metrics push.
func NewWithConfig(cfg OtelConfig) (*Observer, error) {
	if !cfg.MetricsEnabled && !cfg.OtelEnabled {
		return nil, nil
	}

	obs := &Observer{}
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(semconv.ServiceName(cfg.ServiceName)),
		resource.WithFromEnv(),
		resource.WithHost(),
	)
	if err != nil {
		return nil, err
	}

	// Metrics: collect readers (Prometheus pull + optional OTLP push)
	var meterOpts []sdkmetric.Option
	meterOpts = append(meterOpts, sdkmetric.WithResource(res))

	if cfg.MetricsEnabled {
		promExp, err := promexporter.New()
		if err != nil {
			return nil, err
		}
		meterOpts = append(meterOpts, sdkmetric.WithReader(promExp))
		obs.promHandler = promhttp.Handler()
	}

	if cfg.OtelMetricsPush {
		endpoint := cfg.OtelMetricsEndpoint
		if endpoint == "" {
			endpoint = cfg.OtelEndpoint
		}
		metricExp, err := otlpmetricgrpc.New(
			context.Background(),
			otlpmetricgrpc.WithEndpoint(endpoint),
			otlpmetricgrpc.WithInsecure(),
		)
		if err != nil {
			return nil, err
		}
		meterOpts = append(meterOpts, sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(15*time.Second)),
		))
	}

	if len(meterOpts) > 1 { // more than just WithResource
		obs.meterProvider = sdkmetric.NewMeterProvider(meterOpts...)
	}

	// Tracing via OTLP gRPC
	if cfg.OtelEnabled {
		traceExporter, err := otlptracegrpc.New(
			context.Background(),
			otlptracegrpc.WithEndpoint(cfg.OtelEndpoint),
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

	// Plugin & multimodal metrics
	o.cacheHitsTotal, err = meter.Int64Counter("wheel_cache_hits_total",
		otelmetric.WithDescription("Total semantic cache hits"))
	if err != nil {
		return err
	}

	o.cacheMissesTotal, err = meter.Int64Counter("wheel_cache_misses_total",
		otelmetric.WithDescription("Total semantic cache misses"))
	if err != nil {
		return err
	}

	o.contentFilterTotal, err = meter.Int64Counter("wheel_content_filter_total",
		otelmetric.WithDescription("Total requests blocked by content filter"))
	if err != nil {
		return err
	}

	o.rateLimitHitsTotal, err = meter.Int64Counter("wheel_rate_limit_hits_total",
		otelmetric.WithDescription("Total requests blocked by rate limiter"))
	if err != nil {
		return err
	}

	o.multimodalTotal, err = meter.Int64Counter("wheel_multimodal_requests_total",
		otelmetric.WithDescription("Total multimodal API requests (images, audio)"))
	if err != nil {
		return err
	}

	o.pluginDuration, err = meter.Float64Histogram("wheel_plugin_duration_seconds",
		otelmetric.WithDescription("Plugin execution duration in seconds"))
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
