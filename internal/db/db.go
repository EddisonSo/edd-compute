package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable foreign keys
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	db := &DB{sqlDB}
	if err := db.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func (db *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS api_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			key_hash TEXT NOT NULL,
			name TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_used DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS containers (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			namespace TEXT NOT NULL,
			status TEXT DEFAULT 'pending',
			external_ip TEXT,
			memory_mb INTEGER DEFAULT 512,
			storage_gb INTEGER DEFAULT 5,
			image TEXT DEFAULT 'eddisonso/edd-compute-base:latest',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			stopped_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS ssh_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			public_key TEXT NOT NULL,
			fingerprint TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_containers_user_id ON containers(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ssh_keys_user_id ON ssh_keys(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("execute migration: %w", err)
		}
	}

	return nil
}
