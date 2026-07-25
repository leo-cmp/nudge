package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Init(dbPath string) (*DB, error) {
	// Ensure directory exists if path contains directories
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory: %w", err)
		}
	}

	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	// SQLite PRAGMAs for performance and WAL mode
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;"); err != nil {
		return nil, fmt.Errorf("failed to set sqlite pragmas: %w", err)
	}

	database := &DB{DB: sqlDB}
	if err := database.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run db migrations: %w", err)
	}

	return database, nil
}

func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS reminders (
		id TEXT PRIMARY KEY,
		message TEXT NOT NULL,
		type TEXT CHECK(type IN ('instant', 'scheduled', 'recurring')) NOT NULL,
		scheduled_at DATETIME NULL,
		cron_pattern TEXT NULL,
		status TEXT CHECK(status IN ('pending', 'sent', 'cancelled')) NOT NULL DEFAULT 'pending',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_reminders_status_scheduled ON reminders(status, scheduled_at);
	`

	_, err := d.Exec(schema)
	return err
}
