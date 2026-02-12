package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Migrate reads SQL files from migrationsDir and applies them in order,
// tracking applied migrations in the _drizzle_migrations table.
// Compatible with the Drizzle ORM migration format.
func Migrate(db *sql.DB, migrationsDir string) error {
	// Create tracking table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS _drizzle_migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hash TEXT NOT NULL UNIQUE,
			created_at INTEGER NOT NULL DEFAULT (unixepoch())
		)
	`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Collect already-applied migrations
	rows, err := db.Query("SELECT hash FROM _drizzle_migrations")
	if err != nil {
		return fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return fmt.Errorf("scan migration hash: %w", err)
		}
		applied[hash] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate migration rows: %w", err)
	}

	// Read and sort SQL files
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("[migration] No migrations directory found, skipping")
			return nil
		}
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var sqlFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			sqlFiles = append(sqlFiles, e.Name())
		}
	}
	sort.Strings(sqlFiles)

	// Apply each unapplied migration in a transaction
	for _, file := range sqlFiles {
		if applied[file] {
			continue
		}

		content, err := os.ReadFile(filepath.Join(migrationsDir, file))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}

		// Split by Drizzle's statement breakpoint delimiter
		// Note: two spaces before "statement-breakpoint"
		parts := strings.Split(string(content), "-->  statement-breakpoint")
		var statements []string
		for _, s := range parts {
			s = strings.TrimSpace(s)
			if s != "" {
				statements = append(statements, s)
			}
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", file, err)
		}

		for _, stmt := range statements {
			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("exec migration %s: %w\nStatement: %s", file, err, stmt)
			}
		}

		if _, err := tx.Exec("INSERT INTO _drizzle_migrations (hash) VALUES (?)", file); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", file, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", file, err)
		}

		log.Printf("[migration] Applied: %s", file)
	}

	return nil
}

// MigrateLogDB creates the relay_logs table and indexes in the separate log database.
func MigrateLogDB(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS relay_logs (
			id integer PRIMARY KEY AUTOINCREMENT NOT NULL,
			time integer NOT NULL,
			request_model_name text DEFAULT '' NOT NULL,
			channel_id integer DEFAULT 0 NOT NULL,
			channel_name text DEFAULT '' NOT NULL,
			actual_model_name text DEFAULT '' NOT NULL,
			input_tokens integer DEFAULT 0 NOT NULL,
			output_tokens integer DEFAULT 0 NOT NULL,
			ftut integer DEFAULT 0 NOT NULL,
			use_time integer DEFAULT 0 NOT NULL,
			cost real DEFAULT 0 NOT NULL,
			request_content text DEFAULT '' NOT NULL,
			response_content text DEFAULT '' NOT NULL,
			error text DEFAULT '' NOT NULL,
			attempts text DEFAULT '[]' NOT NULL,
			total_attempts integer DEFAULT 0 NOT NULL,
			upstream_content text
		)`,
		`CREATE INDEX IF NOT EXISTS idx_relay_logs_time ON relay_logs(time)`,
		`CREATE INDEX IF NOT EXISTS idx_relay_logs_channel_id ON relay_logs(channel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_relay_logs_error ON relay_logs(error) WHERE error != ''`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate log db: %w\nStatement: %s", err, stmt)
		}
	}

	log.Println("[migration] Log database schema ready")
	return nil
}
