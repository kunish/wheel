## ADDED Requirements

### Requirement: Periodic background log cleanup

The system SHALL run a background goroutine that periodically deletes expired logs based on the configured retention period.

#### Scenario: Cleanup executes on schedule

- **WHEN** the cleanup timer fires (every 1 hour)
- **THEN** the system SHALL read the `log_retention_days` setting from the database
- **AND** delete all `relay_logs` rows where `time` is older than the retention period

#### Scenario: No retention setting configured

- **WHEN** the `log_retention_days` setting does not exist in the database
- **THEN** the system SHALL use a default retention of 30 days

#### Scenario: No expired logs

- **WHEN** the cleanup runs but no logs exceed the retention period
- **THEN** no DELETE operation SHALL be executed

### Requirement: Remove inline cleanup from request path

The system SHALL NOT perform log cleanup within the relay request handling goroutines.

#### Scenario: Relay request completes

- **WHEN** a relay request finishes and triggers async logging
- **THEN** the `maybeCleanupLogs()` call SHALL NOT be invoked

### Requirement: Cleanup runs at application startup

The system SHALL start the background cleanup goroutine when the application initializes.

#### Scenario: Application starts

- **WHEN** the application starts and the database is ready
- **THEN** the background cleanup goroutine SHALL be started
- **AND** an initial cleanup SHALL run immediately before entering the periodic loop
