## ADDED Requirements

### Requirement: TiDB-compatible migration tracking table

The system SHALL create a `_drizzle_migrations` table using MySQL/TiDB DDL syntax with `AUTO_INCREMENT` and `UNIX_TIMESTAMP()`.

#### Scenario: First startup creates tracking table

- **WHEN** the application starts and `_drizzle_migrations` does not exist
- **THEN** the system SHALL create it with columns: `id INT PRIMARY KEY AUTO_INCREMENT`, `hash VARCHAR(255) NOT NULL UNIQUE`, `created_at BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP())`

### Requirement: TiDB-compatible relay_logs schema

The `MigrateLogDB` function SHALL create the `relay_logs` table using MySQL/TiDB DDL: `INT` instead of `integer`, `AUTO_INCREMENT` instead of `AUTOINCREMENT`, `DOUBLE` instead of `real`, `TEXT` with prefix length for indexes.

#### Scenario: relay_logs table creation

- **WHEN** the log database migration runs
- **THEN** the `relay_logs` table SHALL be created with `id INT PRIMARY KEY AUTO_INCREMENT`, `cost DOUBLE DEFAULT 0`, and all text columns as `TEXT`

#### Scenario: Error column index without partial filter

- **WHEN** indexes are created on `relay_logs`
- **THEN** the error index SHALL use `error(255)` prefix length instead of a `WHERE` clause partial index

### Requirement: Drizzle migration SQL compatibility

The migration runner SHALL execute TiDB-compatible SQL files. Existing SQLite-specific Drizzle files SHALL be replaced with a consolidated init schema.

#### Scenario: Fresh database initialization

- **WHEN** the application starts against an empty TiDB database
- **THEN** all tables (api_keys, channels, channel_keys, groups, group_items, users, settings, llm_prices) SHALL be created with MySQL/TiDB syntax
