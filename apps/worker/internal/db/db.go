package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	_ "modernc.org/sqlite"
)

// Open creates a SQLite connection with WAL mode and foreign keys enabled,
// wrapped in a Bun DB instance.
func Open(dbPath string) (*bun.DB, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	sqldb, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite concurrency safety
	sqldb.SetMaxOpenConns(1)

	// Enable WAL mode and foreign keys
	if _, err := sqldb.Exec("PRAGMA journal_mode = WAL"); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := sqldb.Exec("PRAGMA foreign_keys = ON"); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("enable foreign_keys: %w", err)
	}

	bunDB := bun.NewDB(sqldb, sqlitedialect.New())
	return bunDB, nil
}

// OpenLogDB creates a SQLite connection optimized for high-frequency log writes.
// It uses a separate database file to avoid contention with the main config database.
func OpenLogDB(dbPath string) (*bun.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	sqldb, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sqldb.SetMaxOpenConns(1)

	pragmas := []struct {
		sql string
		msg string
	}{
		{"PRAGMA journal_mode = WAL", "set WAL mode"},
		{"PRAGMA busy_timeout = 5000", "set busy_timeout"},
		{"PRAGMA synchronous = NORMAL", "set synchronous"},
		{"PRAGMA cache_size = -64000", "set cache_size"},
	}
	for _, p := range pragmas {
		if _, err := sqldb.Exec(p.sql); err != nil {
			sqldb.Close()
			return nil, fmt.Errorf("%s: %w", p.msg, err)
		}
	}

	bunDB := bun.NewDB(sqldb, sqlitedialect.New())
	return bunDB, nil
}
