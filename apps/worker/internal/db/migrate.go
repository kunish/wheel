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

// initSchema contains TiDB/MySQL-compatible DDL for all tables.
var initSchema = []string{
	`CREATE TABLE IF NOT EXISTS api_keys (
		id INT PRIMARY KEY AUTO_INCREMENT,
		name TEXT NOT NULL,
		api_key TEXT NOT NULL,
		enabled BOOLEAN DEFAULT true NOT NULL,
		expire_at BIGINT DEFAULT 0 NOT NULL,
		max_cost DOUBLE DEFAULT 0 NOT NULL,
		supported_models TEXT NOT NULL,
		total_cost DOUBLE DEFAULT 0 NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS channel_keys (
		id INT PRIMARY KEY AUTO_INCREMENT,
		channel_id INT NOT NULL,
		enabled BOOLEAN DEFAULT true NOT NULL,
		channel_key TEXT NOT NULL,
		status_code INT DEFAULT 0 NOT NULL,
		last_use_timestamp BIGINT DEFAULT 0 NOT NULL,
		total_cost DOUBLE DEFAULT 0 NOT NULL,
		remark TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS channels (
		id INT PRIMARY KEY AUTO_INCREMENT,
		name TEXT NOT NULL,
		type INT DEFAULT 0 NOT NULL,
		enabled BOOLEAN DEFAULT true NOT NULL,
		base_urls TEXT NOT NULL,
		model TEXT NOT NULL,
		fetched_model TEXT NOT NULL,
		custom_model TEXT NOT NULL,
		proxy BOOLEAN DEFAULT false NOT NULL,
		auto_sync BOOLEAN DEFAULT false NOT NULL,
		auto_group INT DEFAULT 0 NOT NULL,
		custom_header TEXT NOT NULL,
		param_override TEXT,
		channel_proxy TEXT,
		` + "`order`" + ` INT DEFAULT 0 NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS group_items (
		id INT PRIMARY KEY AUTO_INCREMENT,
		group_id INT NOT NULL,
		channel_id INT NOT NULL,
		model_name TEXT NOT NULL,
		priority INT DEFAULT 0 NOT NULL,
		weight INT DEFAULT 0 NOT NULL,
		enabled BOOLEAN DEFAULT true NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS ` + "`groups`" + ` (
		id INT PRIMARY KEY AUTO_INCREMENT,
		name TEXT NOT NULL,
		mode INT DEFAULT 0 NOT NULL,
		first_token_time_out INT DEFAULT 0 NOT NULL,
		session_keep_time INT DEFAULT 0 NOT NULL,
		` + "`order`" + ` INT DEFAULT 0 NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS users (
		id INT PRIMARY KEY AUTO_INCREMENT,
		username TEXT NOT NULL,
		password TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS settings (
		` + "`key`" + ` VARCHAR(255) PRIMARY KEY,
		value TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS llm_prices (
		id INT PRIMARY KEY AUTO_INCREMENT,
		name VARCHAR(255) NOT NULL,
		input_price DOUBLE DEFAULT 0 NOT NULL,
		output_price DOUBLE DEFAULT 0 NOT NULL,
		cache_read_price DOUBLE DEFAULT 0 NOT NULL,
		cache_write_price DOUBLE DEFAULT 0 NOT NULL,
		source VARCHAR(255) DEFAULT '' NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS relay_logs (
		id INT PRIMARY KEY AUTO_INCREMENT,
		time BIGINT NOT NULL,
		request_model_name TEXT NOT NULL,
		channel_id INT DEFAULT 0 NOT NULL,
		channel_name TEXT NOT NULL,
		actual_model_name TEXT NOT NULL,
		input_tokens INT DEFAULT 0 NOT NULL,
		output_tokens INT DEFAULT 0 NOT NULL,
		ftut INT DEFAULT 0 NOT NULL,
		use_time INT DEFAULT 0 NOT NULL,
		cost DOUBLE DEFAULT 0 NOT NULL,
		request_content TEXT NOT NULL,
		response_content TEXT NOT NULL,
		error TEXT NOT NULL,
		attempts TEXT NOT NULL,
		total_attempts INT DEFAULT 0 NOT NULL,
		upstream_content TEXT
	)`,
}

// initIndexes contains indexes created after tables exist.
var initIndexes = []string{
	`CREATE INDEX IF NOT EXISTS idx_relay_logs_time ON relay_logs(time)`,
	`CREATE INDEX IF NOT EXISTS idx_relay_logs_channel_id ON relay_logs(channel_id)`,
	`CREATE INDEX IF NOT EXISTS idx_relay_logs_error ON relay_logs(error(255))`,
}

// Migrate ensures all tables exist, then applies any pending Drizzle migration files.
func Migrate(db *sql.DB) error {
	// 1. Create all tables
	for _, ddl := range initSchema {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("init schema: %w\nDDL: %s", err, ddl)
		}
	}
	for _, ddl := range initIndexes {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("init index: %w\nDDL: %s", err, ddl)
		}
	}

	log.Println("[migration] Schema ready")
	return nil
}

// MigrateWithDrizzle applies Drizzle-format SQL migration files on top of the base schema.
// This is kept for forward compatibility if new migrations are added.
func MigrateWithDrizzle(db *sql.DB, migrationsDir string) error {
	// Create tracking table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS _drizzle_migrations (
			id INT PRIMARY KEY AUTO_INCREMENT,
			hash VARCHAR(255) NOT NULL UNIQUE,
			created_at BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP())
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

	for _, file := range sqlFiles {
		if applied[file] {
			continue
		}

		content, err := os.ReadFile(filepath.Join(migrationsDir, file))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}

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
