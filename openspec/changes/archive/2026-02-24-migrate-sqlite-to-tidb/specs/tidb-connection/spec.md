## ADDED Requirements

### Requirement: DSN-based database connection

The system SHALL connect to TiDB/MySQL using a DSN string from the `DB_DSN` environment variable, replacing the SQLite file path configuration.

#### Scenario: Successful connection with DSN

- **WHEN** `DB_DSN` is set to a valid MySQL DSN (e.g., `user:pass@tcp(host:4000)/wheel?parseTime=true`)
- **THEN** the system connects and returns a configured `*bun.DB` instance with `mysqldialect`

#### Scenario: Missing DSN

- **WHEN** `DB_DSN` is not set
- **THEN** the system SHALL return an error at startup

### Requirement: Connection pooling

The system SHALL configure a connection pool suitable for concurrent access, replacing SQLite's `MaxOpenConns(1)`.

#### Scenario: Default pool settings

- **WHEN** no pool-specific environment variables are set
- **THEN** the connection pool SHALL use `MaxOpenConns=25`, `MaxIdleConns=10`, `ConnMaxLifetime=5m`

### Requirement: Unified database connection

The system SHALL use a single database connection for both config tables and relay log tables, eliminating the separate `OpenLogDB` function.

#### Scenario: Single connection serves all tables

- **WHEN** the application starts
- **THEN** both config queries (channels, groups, api_keys) and log queries (relay_logs) SHALL use the same `*bun.DB` instance

### Requirement: MySQL upsert syntax

The system SHALL use `ON DUPLICATE KEY UPDATE` with `VALUES()` for upsert operations, replacing SQLite's `ON CONFLICT ... EXCLUDED` syntax.

#### Scenario: Settings upsert

- **WHEN** a setting with an existing key is inserted
- **THEN** the system SHALL update the value using `ON DUPLICATE KEY UPDATE` syntax

#### Scenario: Price sync timestamp upsert

- **WHEN** `SetLastPriceSyncTime` is called
- **THEN** the system SHALL upsert using `ON DUPLICATE KEY UPDATE` syntax

### Requirement: MySQL datetime functions

The system SHALL use `NOW()` instead of SQLite's `datetime('now')` for timestamp updates.

#### Scenario: LLM price update

- **WHEN** an LLM price record is updated
- **THEN** the `updated_at` column SHALL be set using `NOW()`
