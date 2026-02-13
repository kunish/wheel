## ADDED Requirements

### Requirement: OTLP gRPC trace export

The system SHALL export distributed traces via OTLP gRPC protocol to a configurable endpoint when `OTEL_ENABLED=true`.

#### Scenario: Tracing enabled with default endpoint

- **WHEN** `OTEL_ENABLED=true` and `OTEL_EXPORTER_ENDPOINT` is not set
- **THEN** the system creates a TracerProvider that exports spans via OTLP gRPC to `localhost:4317` with service name `wheel-gateway`

#### Scenario: Tracing enabled with custom endpoint

- **WHEN** `OTEL_ENABLED=true` and `OTEL_EXPORTER_ENDPOINT=collector.example.com:4317`
- **THEN** the system exports spans to the specified endpoint

#### Scenario: Tracing disabled by default

- **WHEN** `OTEL_ENABLED` is not set or set to any value other than `true`
- **THEN** no TracerProvider is created and no gRPC connections are established

### Requirement: Relay root span

The system SHALL create a root span for each relay request when tracing is enabled.

#### Scenario: Relay span created

- **WHEN** a relay request enters `handleRelay` with tracing enabled
- **THEN** a span named `relay` is created and ended when the handler returns

### Requirement: Attempt child span

The system SHALL create a child span for each upstream attempt within a relay request.

#### Scenario: Successful attempt span

- **WHEN** an upstream attempt succeeds
- **THEN** the attempt span records the channel name, channel ID, attempt number, status code 200, and duration in milliseconds

#### Scenario: Failed attempt span

- **WHEN** an upstream attempt fails with an error
- **THEN** the attempt span records the error, status code (if available), and duration, and the span status is set to error

### Requirement: Circuit breaker span event

The system SHALL record a span event when a circuit breaker skip occurs during a relay request.

#### Scenario: Circuit breaker tripped during relay

- **WHEN** a channel is skipped due to a tripped circuit breaker during a relay request
- **THEN** a span event named `circuit_breaker_tripped` is added to the current span with the channel name and channel ID as attributes

### Requirement: Graceful shutdown

The system SHALL flush and shut down the TracerProvider on application exit.

#### Scenario: Application shutdown with tracing enabled

- **WHEN** the application receives a shutdown signal and tracing is enabled
- **THEN** the TracerProvider is shut down with a context, flushing any pending spans

### Requirement: Zero overhead when disabled

Tracing SHALL impose zero runtime overhead when disabled. All span creation methods SHALL return no-op spans when the Observer is nil.

#### Scenario: Nil observer returns no-op span

- **WHEN** `OTEL_ENABLED` is not `true` and `StartRelaySpan` is called
- **THEN** the method returns the original context and a no-op span that does nothing on `End()`
