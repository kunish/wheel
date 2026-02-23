## 1. Dependencies & Driver

- [x] 1.1 Replace `modernc.org/sqlite` and `sqlitedialect` with `go-sql-driver/mysql` and `mysqldialect` in `go.mod`
- [x] 1.2 Run `go mod tidy` to clean up transitive dependencies

## 2. Connection Layer

- [x] 2.1 Rewrite `internal/db/db.go`: replace `Open()`/`OpenLogDB()` with a single `Open(dsn string)` that uses MySQL driver, `mysqldialect`, and configurable connection pool (`MaxOpenConns=25`, `MaxIdleConns=10`, `ConnMaxLifetime=5m`)
- [x] 2.2 Update all call sites that pass file paths to `Open()`/`OpenLogDB()` to pass `DB_DSN` environment variable instead
- [x] 2.3 Remove separate logDB references — unify to single `*bun.DB` instance throughout the application

## 3. Migration System

- [x] 3.1 Rewrite `_drizzle_migrations` table DDL in `internal/db/migrate.go` to use `INT AUTO_INCREMENT`, `VARCHAR(255)`, `UNIX_TIMESTAMP()`
- [x] 3.2 Rewrite `MigrateLogDB()` relay_logs DDL: `INT AUTO_INCREMENT`, `DOUBLE`, prefix index `error(255)` without `WHERE` clause
- [x] 3.3 Create consolidated TiDB-compatible init schema SQL for all config tables (api_keys, channels, channel_keys, groups, group_items, users, settings, llm_prices)

## 4. Upsert Syntax

- [x] 4.1 Update `internal/db/dal/settings.go:41` — replace `On("CONFLICT(key) DO UPDATE")` + `EXCLUDED.value` with `On("DUPLICATE KEY UPDATE")` + `VALUES(value)`
- [x] 4.2 Update `internal/db/dal/models.go:122` — same upsert syntax replacement

## 5. DateTime Functions

- [x] 5.1 Replace `datetime('now')` with `NOW()` in `internal/db/dal/models.go` (lines 65, 71, 94)

## 6. Stats Queries

- [x] 6.1 Refactor `tzModifier()` in `internal/db/dal/stats.go` to return `int` (seconds offset) instead of SQLite modifier string
- [x] 6.2 Replace `strftime('%%Y%%m%%d', time, 'unixepoch', modifier)` with `DATE_FORMAT(DATE_ADD(FROM_UNIXTIME(time), INTERVAL ? SECOND), '%Y%m%d')` in `GetTodayStats`, `GetDailyStats`, `GetHourlyStats`
- [x] 6.3 Replace `CAST(strftime('%%H', ...) AS INTEGER)` with `HOUR(DATE_ADD(FROM_UNIXTIME(time), INTERVAL ? SECOND))` in `GetHourlyStats`
- [x] 6.4 Replace `unixepoch('now', '-365 days')` with `UNIX_TIMESTAMP(DATE_SUB(NOW(), INTERVAL 365 DAY))` in `GetDailyStats`
- [x] 6.5 Replace all `CAST(... AS REAL)` with `CAST(... AS DOUBLE)` in `metricsColumns()` and other aggregation expressions (~15 occurrences)

## 7. Verification

- [x] 7.1 Verify the application compiles with `go build ./...`
- [x] 7.2 Verify connection to a TiDB/MySQL instance and successful schema creation
