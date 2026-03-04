package db

import (
	"database/sql"
	"fmt"
	"log"
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
		profile_id INT NOT NULL DEFAULT 0,
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
	`CREATE TABLE IF NOT EXISTS model_profiles (
		id INT PRIMARY KEY AUTO_INCREMENT,
		name VARCHAR(255) NOT NULL,
		provider VARCHAR(255) NOT NULL,
		models TEXT NOT NULL,
		is_builtin BOOLEAN DEFAULT false NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS routing_rules (
		id INT PRIMARY KEY AUTO_INCREMENT,
		name VARCHAR(255) NOT NULL,
		priority INT DEFAULT 0 NOT NULL,
		enabled BOOLEAN DEFAULT true NOT NULL,
		conditions TEXT NOT NULL,
		action TEXT NOT NULL
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
		request_content MEDIUMTEXT NOT NULL,
		response_content MEDIUMTEXT NOT NULL,
		error TEXT NOT NULL,
		attempts MEDIUMTEXT NOT NULL,
		total_attempts INT DEFAULT 0 NOT NULL,
		upstream_content MEDIUMTEXT
	)`,
	`CREATE TABLE IF NOT EXISTS audit_logs (
		id INT PRIMARY KEY AUTO_INCREMENT,
		time BIGINT NOT NULL,
		user VARCHAR(255) NOT NULL,
		action VARCHAR(64) NOT NULL,
		target VARCHAR(255) NOT NULL,
		detail TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS mcp_logs (
		id INT PRIMARY KEY AUTO_INCREMENT,
		time BIGINT NOT NULL,
		client_id INT NOT NULL,
		client_name VARCHAR(255) NOT NULL,
		tool_name VARCHAR(255) NOT NULL,
		status VARCHAR(32) NOT NULL,
		duration INT DEFAULT 0 NOT NULL,
		error TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS model_limits (
		id INT PRIMARY KEY AUTO_INCREMENT,
		model VARCHAR(255) NOT NULL,
		rpm INT DEFAULT 0 NOT NULL,
		tpm INT DEFAULT 0 NOT NULL,
		daily_requests INT DEFAULT 0 NOT NULL,
		daily_tokens INT DEFAULT 0 NOT NULL,
		enabled BOOLEAN DEFAULT true NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS guardrail_rules (
		id INT PRIMARY KEY AUTO_INCREMENT,
		name VARCHAR(255) NOT NULL,
		type VARCHAR(32) NOT NULL,
		target VARCHAR(32) NOT NULL,
		action VARCHAR(32) NOT NULL,
		pattern TEXT NOT NULL,
		max_length INT DEFAULT 0 NOT NULL,
		enabled BOOLEAN DEFAULT true NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS tags (
		id INT PRIMARY KEY AUTO_INCREMENT,
		name VARCHAR(255) NOT NULL,
		color VARCHAR(32) NOT NULL DEFAULT 'blue',
		description TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS mcp_clients (
		id INT PRIMARY KEY AUTO_INCREMENT,
		name VARCHAR(255) NOT NULL,
		connection_type VARCHAR(32) NOT NULL DEFAULT 'sse',
		connection_string TEXT NOT NULL,
		stdio_config TEXT NOT NULL,
		auth_type VARCHAR(32) NOT NULL DEFAULT 'none',
		headers TEXT NOT NULL,
		oauth_config_id VARCHAR(255),
		tools_to_execute TEXT NOT NULL,
		tools_to_auto_exec TEXT NOT NULL,
		enabled BOOLEAN DEFAULT true NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS distributed_locks (
		lock_name VARCHAR(255) PRIMARY KEY,
		holder_id VARCHAR(255) NOT NULL,
		acquired_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		expires_at TIMESTAMP NOT NULL,
		INDEX idx_distributed_locks_expires_at (expires_at)
	)`,
	`CREATE TABLE IF NOT EXISTS virtual_keys (
		id INT PRIMARY KEY AUTO_INCREMENT,
		name VARCHAR(255) NOT NULL,
		` + "`key`" + ` VARCHAR(255) NOT NULL UNIQUE,
		description TEXT NOT NULL DEFAULT '',
		team_id INT,
		api_key_id INT NOT NULL,
		enabled BOOLEAN DEFAULT true NOT NULL,
		rate_limit_rpm INT DEFAULT 0 NOT NULL,
		rate_limit_tpm INT DEFAULT 0 NOT NULL,
		max_budget DOUBLE DEFAULT 0 NOT NULL,
		current_spend DOUBLE DEFAULT 0 NOT NULL,
		allowed_models TEXT NOT NULL DEFAULT '[]',
		expires_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
	)`,
}

// initIndexes contains indexes created after tables exist.
var initIndexes = []string{
	`CREATE INDEX IF NOT EXISTS idx_relay_logs_time ON relay_logs(time)`,
	`CREATE INDEX IF NOT EXISTS idx_relay_logs_channel_id ON relay_logs(channel_id)`,
	`CREATE INDEX IF NOT EXISTS idx_relay_logs_error ON relay_logs(error(255))`,
	"CREATE INDEX IF NOT EXISTS idx_groups_profile_id ON `groups`(profile_id)",
	`CREATE INDEX IF NOT EXISTS idx_routing_rules_priority ON routing_rules(priority)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_logs_time ON audit_logs(time)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_logs_user ON audit_logs(user)`,
	`CREATE INDEX IF NOT EXISTS idx_mcp_logs_time ON mcp_logs(time)`,
	`CREATE INDEX IF NOT EXISTS idx_mcp_logs_client_id ON mcp_logs(client_id)`,
	`CREATE INDEX IF NOT EXISTS idx_model_limits_model ON model_limits(model)`,
	`CREATE INDEX IF NOT EXISTS idx_virtual_keys_api_key_id ON virtual_keys(api_key_id)`,
}

// initAlters upgrades existing columns (idempotent).
var initAlters = []string{
	`ALTER TABLE relay_logs MODIFY COLUMN request_content MEDIUMTEXT NOT NULL`,
	`ALTER TABLE relay_logs MODIFY COLUMN response_content MEDIUMTEXT NOT NULL`,
	`ALTER TABLE relay_logs MODIFY COLUMN attempts MEDIUMTEXT NOT NULL`,
	`ALTER TABLE relay_logs MODIFY COLUMN upstream_content MEDIUMTEXT`,
	`ALTER TABLE model_profiles ADD COLUMN IF NOT EXISTS provider VARCHAR(255) NOT NULL DEFAULT ''`,
	`ALTER TABLE model_profiles ADD COLUMN IF NOT EXISTS models TEXT`,
	`UPDATE model_profiles SET models = '[]' WHERE models IS NULL`,
	"ALTER TABLE `groups` ADD COLUMN IF NOT EXISTS profile_id INT NOT NULL DEFAULT 0",
	`ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS rpm_limit INT NOT NULL DEFAULT 0`,
	`ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS tpm_limit INT NOT NULL DEFAULT 0`,
	`ALTER TABLE mcp_clients ADD COLUMN IF NOT EXISTS oauth_config TEXT`,
	`ALTER TABLE routing_rules ADD COLUMN IF NOT EXISTS cel_expression TEXT DEFAULT '' AFTER enabled`,
	`ALTER TABLE routing_rules ADD COLUMN IF NOT EXISTS created_at DATETIME DEFAULT CURRENT_TIMESTAMP`,
	`ALTER TABLE routing_rules ADD COLUMN IF NOT EXISTS updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP`,
}

// Migrate ensures all tables exist, then applies any pending Drizzle migration files.
func Migrate(db *sql.DB) error {
	// 1. Create all tables
	for _, ddl := range initSchema {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("init schema: %w\nDDL: %s", err, ddl)
		}
	}
	for _, ddl := range initAlters {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("alter schema: %w\nDDL: %s", err, ddl)
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
