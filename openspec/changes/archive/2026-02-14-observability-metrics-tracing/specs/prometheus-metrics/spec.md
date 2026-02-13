## ADDED Requirements

### Requirement: Prometheus metrics endpoint

The system SHALL expose a `/metrics` HTTP endpoint that returns metrics in Prometheus text exposition format when `METRICS_ENABLED=true`.

#### Scenario: Metrics endpoint enabled

- **WHEN** `METRICS_ENABLED=true` and a GET request is made to `/metrics`
- **THEN** the system returns HTTP 200 with `text/plain` content containing all `wheel_*` metrics in Prometheus format

#### Scenario: Metrics endpoint disabled by default

- **WHEN** `METRICS_ENABLED` is not set or set to any value other than `true`
- **THEN** the `/metrics` endpoint SHALL NOT be registered and returns HTTP 404

### Requirement: Request counter metric

The system SHALL maintain a `wheel_requests_total` Int64Counter with labels `channel`, `model`, `api_key`, `status_code` that increments on every completed relay request (success or exhaustion).

#### Scenario: Successful request increments counter

- **WHEN** a relay request completes successfully through a channel
- **THEN** `wheel_requests_total` increments by 1 with the corresponding channel name, model, api key, and status code 200

#### Scenario: Exhausted request increments counter

- **WHEN** all channels are exhausted for a relay request
- **THEN** `wheel_requests_total` increments by 1 with the last attempted channel name, model, empty api key, and the exhaustion status code

### Requirement: Error counter metric

The system SHALL maintain a `wheel_errors_total` Int64Counter with labels `channel`, `model`, `error_type` that increments on relay errors.

#### Scenario: Exhaustion error recorded

- **WHEN** all channels are exhausted
- **THEN** `wheel_errors_total` increments by 1 with `error_type` set to `exhausted` or `rate_limited`

### Requirement: Retry counter metric

The system SHALL maintain a `wheel_retries_total` Int64Counter with labels `channel`, `model` that increments on each failed attempt that will be retried.

#### Scenario: Failed attempt triggers retry counter

- **WHEN** an upstream attempt fails and the retry loop continues
- **THEN** `wheel_retries_total` increments by 1 with the channel name and model

### Requirement: Token usage metric

The system SHALL maintain a `wheel_tokens_total` Int64Counter with labels `channel`, `model`, `direction` that records token consumption.

#### Scenario: Streaming request records tokens

- **WHEN** a streaming relay request completes successfully
- **THEN** `wheel_tokens_total` increments by input token count with `direction=input` and by output token count with `direction=output`

#### Scenario: Non-streaming request records tokens

- **WHEN** a non-streaming relay request completes successfully
- **THEN** `wheel_tokens_total` increments by input and output token counts with respective direction labels

### Requirement: Cost metric

The system SHALL maintain a `wheel_cost_dollars_total` Float64Counter with labels `channel`, `model` that records dollar cost.

#### Scenario: Request with positive cost

- **WHEN** a relay request completes and calculated cost is greater than 0
- **THEN** `wheel_cost_dollars_total` increments by the cost value

### Requirement: Duration histogram metric

The system SHALL maintain a `wheel_request_duration_seconds` Float64Histogram with labels `channel`, `model` that records total request duration.

#### Scenario: Request duration recorded

- **WHEN** a relay request completes (success or exhaustion)
- **THEN** `wheel_request_duration_seconds` records the elapsed time in seconds

### Requirement: TTFB histogram metric

The system SHALL maintain a `wheel_ttfb_seconds` Float64Histogram with labels `channel`, `model` that records time-to-first-byte for streaming requests.

#### Scenario: Streaming TTFB recorded

- **WHEN** a streaming relay request completes with a positive first-token time
- **THEN** `wheel_ttfb_seconds` records the TTFB converted from milliseconds to seconds

### Requirement: Circuit breaker state metric

The system SHALL maintain a `wheel_circuit_breaker_state` Int64UpDownCounter with label `channel` that tracks circuit breaker state changes.

#### Scenario: Circuit breaker opens

- **WHEN** a circuit breaker transitions to open state
- **THEN** `wheel_circuit_breaker_state` increments by 1 for that channel

#### Scenario: Circuit breaker closes

- **WHEN** a circuit breaker transitions from open/half-open to closed state
- **THEN** `wheel_circuit_breaker_state` decrements by 1 for that channel

### Requirement: Active streams metric

The system SHALL maintain a `wheel_active_streams` Int64UpDownCounter that tracks currently active streaming connections.

#### Scenario: Stream starts and ends

- **WHEN** a streaming relay request begins proxying
- **THEN** `wheel_active_streams` increments by 1, and decrements by 1 when the stream ends (success or failure)

### Requirement: Zero overhead when disabled

The system SHALL impose zero runtime overhead when metrics are disabled. All metric recording methods SHALL be no-ops when the Observer is nil.

#### Scenario: Nil observer no-op

- **WHEN** `METRICS_ENABLED` is not `true` and a relay request is processed
- **THEN** no metric instruments are created, no allocations occur for metric recording, and no `/metrics` endpoint is registered
