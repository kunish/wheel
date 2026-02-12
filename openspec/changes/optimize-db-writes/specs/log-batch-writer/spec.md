## ADDED Requirements

### Requirement: Log batch writer accepts logs via channel

The system SHALL provide a `LogWriter` service that accepts `RelayLog` entries through a Go channel for asynchronous batched persistence.

#### Scenario: Single log submission

- **WHEN** a relay handler calls `LogWriter.Submit(log)` with a RelayLog entry
- **THEN** the log entry SHALL be buffered in the internal channel without blocking the caller

#### Scenario: Channel backpressure

- **WHEN** the internal buffer channel is full (capacity 1000)
- **THEN** the Submit call SHALL block until space is available, preventing unbounded memory growth

### Requirement: Batch flush by count threshold

The system SHALL flush buffered logs to the database when the buffer reaches 50 entries.

#### Scenario: Count threshold reached

- **WHEN** 50 log entries have accumulated in the buffer
- **THEN** the system SHALL INSERT all 50 entries in a single database transaction
- **AND** each inserted log SHALL trigger a WebSocket `log-created` broadcast

### Requirement: Batch flush by time threshold

The system SHALL flush buffered logs to the database after 2 seconds of inactivity, regardless of buffer count.

#### Scenario: Time threshold reached with partial buffer

- **WHEN** fewer than 50 entries are buffered AND 2 seconds have elapsed since the last flush
- **THEN** the system SHALL INSERT all buffered entries in a single database transaction

#### Scenario: Empty buffer at timer tick

- **WHEN** zero entries are buffered AND the 2-second timer fires
- **THEN** the system SHALL NOT execute any database operation

### Requirement: Cost updates in same transaction

The system SHALL include cost increment operations (api_key and channel_key) within the same transaction as the log batch INSERT.

#### Scenario: Successful request with cost

- **WHEN** a batch flush occurs containing logs with cost > 0
- **THEN** `IncrementApiKeyCost` and `IncrementChannelKeyCost` SHALL execute within the same database transaction as the log INSERT
- **AND** either all operations succeed or all roll back

#### Scenario: Log without cost

- **WHEN** a log entry has cost = 0 (e.g., error logs)
- **THEN** no cost increment operations SHALL be included in the transaction

### Requirement: Graceful shutdown flushes remaining logs

The system SHALL flush all remaining buffered logs on application shutdown.

#### Scenario: Process receives shutdown signal

- **WHEN** the application receives SIGTERM or SIGINT
- **THEN** the LogWriter SHALL flush all remaining buffered logs to the database before exiting
- **AND** the channel SHALL be closed to prevent new submissions

### Requirement: SQLite PRAGMA optimization

The system SHALL configure SQLite with optimized PRAGMAs at connection open time.

#### Scenario: Database connection opened

- **WHEN** `db.Open()` is called
- **THEN** the following PRAGMAs SHALL be set: `busy_timeout = 5000`, `synchronous = NORMAL`, `cache_size = -64000`

### Requirement: Relay logs table indexes

The system SHALL add indexes to the `relay_logs` table for commonly queried columns.

#### Scenario: Migration applied

- **WHEN** the database migration runs
- **THEN** indexes SHALL exist on `relay_logs(time)`, `relay_logs(channel_id)`, and a partial index on `relay_logs(error)` WHERE `error != ''`
