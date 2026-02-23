## Why

Wheel currently uses SQLite as its storage engine, which limits deployment to a single instance (`SetMaxOpenConns(1)`) and creates write contention between config operations and high-frequency log writes (requiring a separate `relay_logs.db`). Migrating to TiDB enables multi-instance horizontal scaling, eliminates the single-writer bottleneck, and provides HTAP capabilities for real-time analytics on the dashboard without impacting API relay performance.

## What Changes

- **BREAKING**: Replace SQLite driver (`modernc.org/sqlite`) and Bun dialect (`sqlitedialect`) with MySQL driver (`go-sql-driver/mysql`) and `mysqldialect`
- **BREAKING**: Database connection changes from file path (`DATA_PATH`) to DSN-based configuration (`DB_DSN`)
- Merge `Open()` and `OpenLogDB()` into a single connection — no longer need separate databases
- Remove all SQLite PRAGMA statements (WAL, busy_timeout, synchronous, cache_size)
- Rewrite SQLite-specific SQL: `strftime()` → `DATE_FORMAT(FROM_UNIXTIME())`, `datetime('now')` → `NOW()`, `CAST AS REAL` → `CAST AS DOUBLE`
- Replace SQLite upsert syntax (`ON CONFLICT ... EXCLUDED`) with MySQL syntax (`ON DUPLICATE KEY UPDATE ... VALUES()`)
- Rewrite migration system DDL from SQLite to MySQL/TiDB compatible syntax
- Remove partial index (`WHERE error != ''`) — not supported in TiDB
- Increase connection pool from 1 to configurable pool size

## Capabilities

### New Capabilities

- `tidb-connection`: Database connection layer supporting TiDB/MySQL with DSN configuration and connection pooling
- `tidb-migration`: Migration system with TiDB-compatible DDL and schema initialization
- `tidb-stats-queries`: Rewritten statistics queries using MySQL/TiDB time functions

### Modified Capabilities

<!-- No existing specs have requirement-level changes -->

## Impact

- **Backend code**: `internal/db/db.go`, `internal/db/migrate.go`, `internal/db/dal/stats.go`, `internal/db/dal/models.go`, `internal/db/dal/settings.go`
- **Dependencies**: `go.mod` — swap `modernc.org/sqlite` + `sqlitedialect` for `go-sql-driver/mysql` + `mysqldialect`
- **Configuration**: New `DB_DSN` environment variable replaces `DATA_PATH` for database connection
- **Deployment**: Requires a running TiDB cluster (or MySQL-compatible database) instead of local file storage
- **Migration files**: All 11 `drizzle/*.sql` files need TiDB-compatible equivalents
