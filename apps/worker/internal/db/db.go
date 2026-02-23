package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
)

// Open creates a MySQL/TiDB connection, auto-creating the database if needed.
func Open(dsn string) (*bun.DB, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	if dbName := cfg.DBName; dbName != "" {
		cfg.DBName = ""
		bootstrap, err := sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			return nil, fmt.Errorf("open bootstrap connection: %w", err)
		}
		_, err = bootstrap.Exec("CREATE DATABASE IF NOT EXISTS `" + dbName + "`")
		bootstrap.Close()
		if err != nil {
			return nil, fmt.Errorf("create database: %w", err)
		}
	}

	sqldb, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqldb.SetMaxOpenConns(25)
	sqldb.SetMaxIdleConns(10)
	sqldb.SetConnMaxLifetime(5 * time.Minute)

	if err := sqldb.Ping(); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return bun.NewDB(sqldb, mysqldialect.New()), nil
}
