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
  containers_path TEXT NOT NULL DEFAULT 'user/bombvault/container',
  vms_path        TEXT NOT NULL DEFAULT 'user/bombvault/vms',
  flash_path      TEXT NOT NULL DEFAULT 'user/bombvault/flash',
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
	{
		// Relax the runs.target_id FK so VM targets can record runs in the same
		// table without a separate runs_vms table. SQLite cannot drop constraints
		// in place, so we recreate runs without the REFERENCES clause (data is
		// preserved via INSERT INTO ... SELECT). The idx_runs_target index is
		// recreated after the table swap.
		version: 5,
		name:    "runs_relax_fk",
		sql: `
PRAGMA foreign_keys=OFF;
CREATE TABLE runs_new (
  id          TEXT    PRIMARY KEY,
  target_id   TEXT    NOT NULL,
  kind        TEXT    NOT NULL,
  status      TEXT    NOT NULL,
  started_at  INTEGER NOT NULL,
  finished_at INTEGER,
  snapshot_id TEXT,
  bytes       INTEGER,
  error       TEXT
);
INSERT INTO runs_new SELECT id, target_id, kind, status, started_at, finished_at, snapshot_id, bytes, error FROM runs;
DROP TABLE runs;
ALTER TABLE runs_new RENAME TO runs;
CREATE INDEX IF NOT EXISTS idx_runs_target ON runs(target_id);
PRAGMA foreign_keys=ON;`,
	},
	{
		// Re-home the default backup paths under the user share:
		// host /mnt/user/bombvault/{container,vms,flash} (relative to the
		// /host/user mount). Only rows still holding the original v1 defaults are
		// updated, so any path a user already customised in Settings is preserved.
		version: 6,
		name:    "default_paths_user_share",
		sql: `
UPDATE settings SET containers_path = 'user/bombvault/container' WHERE containers_path = 'backups/bombvault/containers';
UPDATE settings SET vms_path        = 'user/bombvault/vms'       WHERE vms_path        = 'backups/bombvault/vms';
UPDATE settings SET flash_path      = 'user/bombvault/flash'     WHERE flash_path      = 'backups/bombvault/flash';`,
	},
	{
		// Retention keep-policy (all 0 = off) + the encrypted rclone config for
		// off-site repos.
		version: 7,
		name:    "retention_and_rclone",
		sql: `
ALTER TABLE settings ADD COLUMN retention_keep_last    INTEGER NOT NULL DEFAULT 0;
ALTER TABLE settings ADD COLUMN retention_keep_daily   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE settings ADD COLUMN retention_keep_weekly  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE settings ADD COLUMN retention_keep_monthly INTEGER NOT NULL DEFAULT 0;
ALTER TABLE settings ADD COLUMN rclone_conf            TEXT    NOT NULL DEFAULT '';`,
	},
	{
		// Per-container pre/post-backup hook commands.
		version: 8,
		name:    "target_hooks",
		sql: `
ALTER TABLE targets ADD COLUMN pre_hook  TEXT NOT NULL DEFAULT '';
ALTER TABLE targets ADD COLUMN post_hook TEXT NOT NULL DEFAULT '';`,
	},
	{
		// Per-container explicit backup-folder selection (container-translated
		// paths). Empty ('[]') means "use the automatic appdata detection".
		version: 9,
		name:    "target_selected_paths",
		sql:     "ALTER TABLE targets ADD COLUMN selected_paths TEXT NOT NULL DEFAULT '[]';",
	},
	{
		// Notification config (webhook / Matrix / Healthchecks), stored as an
		// AES-256-GCM-encrypted JSON blob (base64). Empty = notifications off.
		version: 10,
		name:    "settings_notify_conf",
		sql:     "ALTER TABLE settings ADD COLUMN notify_conf TEXT NOT NULL DEFAULT '';",
	},
	{
		// Other container names to stop for the duration of this container's backup
		// (e.g. a database), started again afterwards. JSON array; '[]' = none.
		version: 11,
		name:    "target_stop_containers",
		sql:     "ALTER TABLE targets ADD COLUMN stop_containers TEXT NOT NULL DEFAULT '[]';",
	},
	{
		// Cloud-backend credentials (S3 keys, restic-REST user/password) for
		// off-site repos, stored as an AES-256-GCM-encrypted JSON blob (base64),
		// like rclone_conf/notify_conf. Empty = none.
		version: 12,
		name:    "settings_cloud_conf",
		sql:     "ALTER TABLE settings ADD COLUMN cloud_conf TEXT NOT NULL DEFAULT '';",
	},
	{
		// Optional off-site repo per domain; a successful local backup is replicated
		// there with `restic copy`. Empty = no off-site copy. One column per domain
		// (SQLite ADD COLUMN is single-column), hence three migrations.
		version: 13,
		name:    "settings_containers_offsite",
		sql:     "ALTER TABLE settings ADD COLUMN containers_offsite TEXT NOT NULL DEFAULT '';",
	},
	{
		version: 14,
		name:    "settings_vms_offsite",
		sql:     "ALTER TABLE settings ADD COLUMN vms_offsite TEXT NOT NULL DEFAULT '';",
	},
	{
		version: 15,
		name:    "settings_flash_offsite",
		sql:     "ALTER TABLE settings ADD COLUMN flash_offsite TEXT NOT NULL DEFAULT '';",
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
