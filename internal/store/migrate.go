package store

import (
	"database/sql"
	"fmt"
	"time"
)

type migration struct {
	version int
	name    string
	sql     string
}

var migrations = []migration{
	{
		version: 1,
		name:    "initial_schema",
		sql: `
CREATE TABLE settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  encryption_enabled INTEGER NOT NULL DEFAULT 1,
  containers_enabled INTEGER NOT NULL DEFAULT 1,
  vms_enabled        INTEGER NOT NULL DEFAULT 0,
  flash_enabled      INTEGER NOT NULL DEFAULT 0,
  containers_path TEXT NOT NULL DEFAULT 'backups/bombvault/containers',
  vms_path        TEXT NOT NULL DEFAULT 'backups/bombvault/vms',
  flash_path      TEXT NOT NULL DEFAULT 'backups/bombvault/flash',
  containers_schedule TEXT NOT NULL DEFAULT 'off',
  vms_schedule        TEXT NOT NULL DEFAULT 'off',
  flash_schedule      TEXT NOT NULL DEFAULT 'off',
  default_language TEXT NOT NULL DEFAULT ''
);
INSERT INTO settings (id) VALUES (1);
CREATE TABLE targets (
  id TEXT PRIMARY KEY,
  container_name TEXT NOT NULL UNIQUE,
  appdata_paths TEXT NOT NULL,
  include_in_schedule INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE TABLE runs (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id),
  kind TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  finished_at INTEGER,
  snapshot_id TEXT,
  bytes INTEGER,
  error TEXT
);
CREATE INDEX idx_runs_target ON runs(target_id);
`,
	},
	{
		version: 2,
		name:    "target_definition",
		sql:     "ALTER TABLE targets ADD COLUMN definition TEXT NOT NULL DEFAULT '';",
	},
	{
		version: 3,
		name:    "auth_password",
		sql:     "ALTER TABLE settings ADD COLUMN auth_password_hash TEXT NOT NULL DEFAULT '';",
	},
	{
		version: 4,
		name:    "vms_table",
		sql: `CREATE TABLE vms (
  id                  TEXT    PRIMARY KEY,
  name                TEXT    NOT NULL UNIQUE,
  method              TEXT    NOT NULL DEFAULT 'graceful',
  include_in_schedule INTEGER NOT NULL DEFAULT 0,
  definition          TEXT    NOT NULL DEFAULT '',
  created_at          INTEGER NOT NULL
);`,
	},
}

// Migrate applies any pending forward-only migrations to db.
// It is idempotent: already-applied migrations are skipped.
func Migrate(db *sql.DB) error {
	// Ensure the tracking table exists.
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		name       TEXT NOT NULL,
		applied_at INTEGER NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("migrate: create schema_migrations: %w", err)
	}

	for _, m := range migrations {
		var count int
		row := db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version = ?`, m.version)
		if err := row.Scan(&count); err != nil {
			return fmt.Errorf("migrate: check v%d: %w", m.version, err)
		}
		if count > 0 {
			continue // already applied
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("migrate: begin v%d: %w", m.version, err)
		}
		if _, err := tx.Exec(m.sql); err != nil {
			tx.Rollback() //nolint:errcheck,gosec // best-effort rollback; original error takes priority
			return fmt.Errorf("migrate: apply v%d (%s): %w", m.version, m.name, err)
		}
		_, err = tx.Exec(
			`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			m.version, m.name, time.Now().Unix(),
		)
		if err != nil {
			tx.Rollback() //nolint:errcheck,gosec // best-effort rollback; original error takes priority
			return fmt.Errorf("migrate: record v%d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migrate: commit v%d: %w", m.version, err)
		}
	}
	return nil
}
