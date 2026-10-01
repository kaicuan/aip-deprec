package db

import (
	"database/sql"
	"fmt"
	"sync"

	_ "modernc.org/sqlite"
)

type DB struct {
	db *sql.DB
	mu sync.RWMutex
}

func Open(dbPath string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	d := &DB{db: sqlDB}
	if err := d.initSchema(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return d, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS proxies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			raw_input TEXT NOT NULL,
			url TEXT UNIQUE NOT NULL,
			protocol TEXT NOT NULL,
			host TEXT NOT NULL,
			port INTEGER NOT NULL,
			username TEXT NOT NULL DEFAULT '',
			password TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'online',
			latency_ms INTEGER NOT NULL DEFAULT 0,
			egress_ip TEXT NOT NULL DEFAULT '',
			cooldown_until INTEGER NOT NULL DEFAULT 0,
			last_checked_at INTEGER NOT NULL DEFAULT 0,
			fail_count INTEGER NOT NULL DEFAULT 0,
			success_count INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS models (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			"limit" TEXT NOT NULL DEFAULT '{}',
			modalities TEXT NOT NULL DEFAULT '',
			reasoning BOOLEAN NOT NULL DEFAULT 0,
			tool_call BOOLEAN NOT NULL DEFAULT 0,
			temperature BOOLEAN NOT NULL DEFAULT 0,
			attachment BOOLEAN NOT NULL DEFAULT 0,
			reasoning_options TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			lab TEXT NOT NULL DEFAULT '',
			commercial_rate TEXT NOT NULL DEFAULT '{}',
			fetched_at INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS request_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp INTEGER NOT NULL,
			model TEXT NOT NULL,
			proxy_url TEXT NOT NULL,
			status_code INTEGER NOT NULL,
			duration_ms INTEGER NOT NULL,
			prompt_tokens INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			error_message TEXT NOT NULL DEFAULT '',
			is_stream INTEGER NOT NULL DEFAULT 0,
			session_id TEXT NOT NULL DEFAULT '',
			request_body TEXT NOT NULL DEFAULT '',
			response_body TEXT NOT NULL DEFAULT '',
			estimated_savings REAL NOT NULL DEFAULT 0.0
		);`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_timestamp ON request_logs(timestamp);`,
	}

	for _, q := range queries {
		if _, err := d.db.Exec(q); err != nil {
			return err
		}
	}
	_ = d.addColumnIfNotExists("request_logs", "cache_read_tokens", "INTEGER NOT NULL DEFAULT 0")
	_ = d.addColumnIfNotExists("request_logs", "session_id", "TEXT NOT NULL DEFAULT ''")
	_ = d.addColumnIfNotExists("request_logs", "request_body", "TEXT NOT NULL DEFAULT ''")
	_ = d.addColumnIfNotExists("request_logs", "response_body", "TEXT NOT NULL DEFAULT ''")
	_ = d.addColumnIfNotExists("request_logs", "estimated_savings", "REAL NOT NULL DEFAULT 0.0")
	// Clean up any direct proxy entries so local IP is never part of the pool
	_, _ = d.db.Exec(`DELETE FROM proxies WHERE protocol = 'direct'`)

	return nil
}

func (d *DB) addColumnIfNotExists(table, column, colType string) error {
	query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colType)
	_, err := d.db.Exec(query)
	return err
}
