## Context

Wheel is an LLM API Gateway built with Go (Gin) + React. It currently uses SQLite via `modernc.org/sqlite` driver and Bun ORM (`sqlitedialect`). The database layer is split into two files — `wheel.db` for config and `relay_logs.db` for high-frequency log writes — to work around SQLite's single-writer limitation (`SetMaxOpenConns(1)`).

Key SQLite-specific code is concentrated in:

- `internal/db/db.go` — driver, dialect, PRAGMAs
- `internal/db/dal/stats.go` — `strftime()`, `unixepoch()` for time-based aggregation
- `internal/db/dal/models.go` + `settings.go` — `ON CONFLICT ... EXCLUDED` upsert syntax
- `internal/db/migrate.go` — DDL with `AUTOINCREMENT`, partial indexes

The Bun ORM model definitions (`types/models.go`) and most DAL query logic are database-agnostic and require no changes.

## Goals / Non-Goals

**Goals:**

- Replace SQLite with TiDB/MySQL-compatible database backend
- Unify config and log databases into a single connection
- Enable multi-instance deployment with shared database
- Maintain all existing functionality (API, dashboard, stats)

**Non-Goals:**

- TiFlash (columnar) setup — can be added later without code changes
- Data migration tooling from existing SQLite to TiDB (out of scope, users start fresh or use external tools)
- Frontend changes — the API contract remains identical
- Kubernetes/TiDB Operator deployment automation

## Decisions

### 1. MySQL driver: `go-sql-driver/mysql`

**Choice**: Use the standard `go-sql-driver/mysql` driver.
**Rationale**: Most widely used Go MySQL driver, fully compatible with TiDB, well-tested with Bun ORM. Alternative `github.com/pingcap/tidb/client` is TiDB-specific and less portable.

### 2. Connection pooling strategy

**Choice**: Default `MaxOpenConns=25`, `MaxIdleConns=10`, `ConnMaxLifetime=5m`.
**Rationale**: Replaces SQLite's `MaxOpenConns(1)`. These defaults suit a gateway handling moderate concurrent requests. Configurable via environment variables for production tuning.

### 3. Time function translation approach

**Choice**: Replace SQLite `strftime()`/`unixepoch()` with MySQL `FROM_UNIXTIME()`/`DATE_FORMAT()` directly in SQL.
**Alternative considered**: Move all time computation to Go application layer. Rejected because GROUP BY date/hour aggregation is more efficient in the database, and TiDB's time functions are stable and well-documented.

### 4. Upsert syntax

**Choice**: Use Bun's `On("DUPLICATE KEY UPDATE")` with `VALUES()` syntax.
**Rationale**: Standard MySQL upsert pattern. TiDB fully supports it. Only 2 call sites need updating (`settings.go:41`, `models.go:122`).

### 5. Migration strategy

**Choice**: Create a single consolidated TiDB-compatible init schema in `migrate.go`, keep Drizzle SQL files as historical reference only.
**Alternative considered**: Convert all 11 Drizzle files to MySQL syntax. Rejected because the incremental SQLite migrations use SQLite-specific patterns (table rebuild for column removal) that don't translate cleanly. A fresh init schema is simpler and less error-prone.

### 6. `CAST AS REAL` → `CAST AS DOUBLE`

**Choice**: Global replace in `stats.go`.
**Rationale**: SQLite's `REAL` is 8-byte float. MySQL/TiDB equivalent is `DOUBLE`. Semantically identical, mechanical replacement.

## Risks / Trade-offs

- **[Deployment complexity]** → TiDB requires PD + TiKV + TiDB server vs SQLite's zero-config. Mitigation: document Docker Compose setup with TiDB; users can also point at any MySQL 8.0 instance.
- **[Latency increase]** → Network round-trip vs local file I/O. Mitigation: connection pooling and TiDB's query cache. Acceptable for a gateway that already makes external HTTP calls per request.
- **[Partial index loss]** → SQLite's `WHERE error != ''` partial index on `relay_logs` cannot be replicated in TiDB. Mitigation: use prefix index `error(255)` — slightly less efficient but functional.
- **[Breaking change]** → Existing SQLite users must re-deploy with a TiDB instance. Mitigation: clear documentation and Docker Compose example.
