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
	{
		// Optional off-site replication schedule per domain. Empty = replicate
		// after every local backup; set = replicate only on this cadence.
		version: 16,
		name:    "settings_containers_offsite_schedule",
		sql:     "ALTER TABLE settings ADD COLUMN containers_offsite_schedule TEXT NOT NULL DEFAULT '';",
	},
	{
		version: 17,
		name:    "settings_vms_offsite_schedule",
		sql:     "ALTER TABLE settings ADD COLUMN vms_offsite_schedule TEXT NOT NULL DEFAULT '';",
	},
	{
		version: 18,
		name:    "settings_flash_offsite_schedule",
		sql:     "ALTER TABLE settings ADD COLUMN flash_offsite_schedule TEXT NOT NULL DEFAULT '';",
	},
	{
		version: 19,
		name:    "settings_offsite_retention_keep_last",
		sql:     "ALTER TABLE settings ADD COLUMN offsite_retention_keep_last INTEGER NOT NULL DEFAULT 0;",
	},
	{
		version: 20,
		name:    "settings_offsite_retention_keep_daily",
		sql:     "ALTER TABLE settings ADD COLUMN offsite_retention_keep_daily INTEGER NOT NULL DEFAULT 0;",
	},
	{
		version: 21,
		name:    "settings_offsite_retention_keep_weekly",
		sql:     "ALTER TABLE settings ADD COLUMN offsite_retention_keep_weekly INTEGER NOT NULL DEFAULT 0;",
	},
	{
		version: 22,
		name:    "settings_offsite_retention_keep_monthly",
		sql:     "ALTER TABLE settings ADD COLUMN offsite_retention_keep_monthly INTEGER NOT NULL DEFAULT 0;",
	},
	{
		// Repository-size history (per domain + source), sampled after a successful
		// backup. Drives the dashboard's size/dedup trend. raw_size = physical
		// (deduplicated + compressed) repo size; restore_size = logical size;
		// snapshots = snapshot count at sample time.
		version: 23,
		name:    "repo_stats",
		sql: `CREATE TABLE repo_stats (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  domain       TEXT    NOT NULL,
  source       TEXT    NOT NULL,
  at           INTEGER NOT NULL,
  raw_size     INTEGER NOT NULL,
  restore_size INTEGER NOT NULL,
  snapshots    INTEGER NOT NULL
);`,
	},
	{
		// Off-site transfer bandwidth caps (KiB/s) for restic's global
		// --limit-upload / --limit-download. 0 (the default) = unlimited, so the
		// WAN is never throttled until the user sets a cap.
		version: 24,
		name:    "settings_offsite_limit_upload",
		sql:     "ALTER TABLE settings ADD COLUMN offsite_limit_upload INTEGER NOT NULL DEFAULT 0;",
	},
	{
		version: 25,
		name:    "settings_offsite_limit_download",
		sql:     "ALTER TABLE settings ADD COLUMN offsite_limit_download INTEGER NOT NULL DEFAULT 0;",
	},
	{
		// Opt-in Prometheus /metrics endpoint for Grafana / Uptime Kuma scraping.
		// Default 0 (off): when disabled the endpoint returns 404 and is not served.
		version: 26,
		name:    "settings_metrics_enabled",
		sql:     "ALTER TABLE settings ADD COLUMN metrics_enabled INTEGER NOT NULL DEFAULT 0;",
	},
	{
		// Optional bearer token for /metrics. Empty (the default) = open (LAN trust
		// model, like /api/health); set = require Authorization: Bearer <token>.
		version: 27,
		name:    "settings_metrics_token",
		sql:     "ALTER TABLE settings ADD COLUMN metrics_token TEXT NOT NULL DEFAULT '';",
	},
	{
		// Restore-verification "drills": each row records one `restic check
		// --read-data-subset` run for a domain + source, proving the backup is
		// actually restorable. ok = 1 on success, 0 on failure; detail = a short
		// scrubbed reason (empty on success). Powers the "last verified restorable"
		// badge.
		version: 28,
		name:    "restore_drills",
		sql: `CREATE TABLE restore_drills (
  id     INTEGER PRIMARY KEY AUTOINCREMENT,
  domain TEXT    NOT NULL,
  source TEXT    NOT NULL,
  at     INTEGER NOT NULL,
  ok     INTEGER NOT NULL,
  detail TEXT    NOT NULL DEFAULT ''
);`,
	},
	{
		// Scheduled restore drills: enable flag, cadence (same grammar as the backup
		// schedules; 'off' = no scheduled drills), and the data subset percent each
		// drill reads back. Off by default (drills are expensive: they read real pack
		// data), so existing setups are unchanged until the user opts in.
		version: 29,
		name:    "settings_drills_enabled",
		sql:     "ALTER TABLE settings ADD COLUMN drills_enabled INTEGER NOT NULL DEFAULT 0;",
	},
	{
		version: 30,
		name:    "settings_drills_schedule",
		sql:     "ALTER TABLE settings ADD COLUMN drills_schedule TEXT NOT NULL DEFAULT 'off';",
	},
	{
		version: 31,
		name:    "settings_drills_subset_pct",
		sql:     "ALTER TABLE settings ADD COLUMN drills_subset_pct INTEGER NOT NULL DEFAULT 5;",
	},
	{
		// Acknowledgement that the user has downloaded + safely stored the
		// encryption-key recovery kit, so the dashboard nag can be dismissed.
		// Default 0 (the nag shows while encryption is on and this is unset).
		version: 32,
		name:    "settings_recovery_kit_ack",
		sql:     "ALTER TABLE settings ADD COLUMN recovery_kit_ack INTEGER NOT NULL DEFAULT 0;",
	},
	{
		// Default folder for "restore to a folder": a relative subpath under the
		// host mount that pre-fills the restore-to-folder picker (same style as the
		// backup-path settings). Extracts land under here unless the user picks
		// elsewhere.
		version: 33,
		name:    "settings_restore_folder",
		sql:     "ALTER TABLE settings ADD COLUMN restore_folder TEXT NOT NULL DEFAULT 'user/bombvault/restore';",
	},
	{
		// Per-domain "off-site repo is append-only (immutable)" flag: the far side
		// (e.g. rest-server --append-only) enforces it; BombVault then skips its own
		// off-site prune and refuses off-site deletes. One column per domain (SQLite
		// ADD COLUMN is single-column), hence three migrations.
		version: 34,
		name:    "settings_containers_offsite_immutable",
		sql:     "ALTER TABLE settings ADD COLUMN containers_offsite_immutable INTEGER NOT NULL DEFAULT 0;",
	},
	{
		version: 35,
		name:    "settings_vms_offsite_immutable",
		sql:     "ALTER TABLE settings ADD COLUMN vms_offsite_immutable INTEGER NOT NULL DEFAULT 0;",
	},
	{
		version: 36,
		name:    "settings_flash_offsite_immutable",
		sql:     "ALTER TABLE settings ADD COLUMN flash_offsite_immutable INTEGER NOT NULL DEFAULT 0;",
	},
	{
		// Off-site growth budget (GB): an append-only repo only ever grows, so this
		// caps how large it may get before a notification fires (detection, not
		// prevention). 0 = budget alarm off (the default).
		version: 37,
		name:    "settings_offsite_growth_budget_gb",
		sql:     "ALTER TABLE settings ADD COLUMN offsite_growth_budget_gb INTEGER NOT NULL DEFAULT 0;",
	},
	{
		// Cadence for the scheduled off-site tamper test (same grammar as the backup
		// schedules). Weekly by default so the "append-only is still enforced"
		// verdict never grows stale unnoticed.
		version: 38,
		name:    "settings_tamper_test_schedule",
		sql:     "ALTER TABLE settings ADD COLUMN tamper_test_schedule TEXT NOT NULL DEFAULT 'weekly Sun 04:30';",
	},
	{
		// Container the real-restore DR drill restores by default. Empty (the
		// default) = auto: the most recently successfully backed-up container.
		version: 39,
		name:    "settings_dr_drill_target",
		sql:     "ALTER TABLE settings ADD COLUMN dr_drill_target TEXT NOT NULL DEFAULT '';",
	},
	{
		// Off-site tamper-test history: each row is one active probe of the far
		// side's delete path. protected = 1 means the delete was refused (append-only
		// is actually enforced); detail carries the scrubbed status/error.
		version: 40,
		name:    "tamper_tests",
		sql: `CREATE TABLE IF NOT EXISTS tamper_tests (
  domain TEXT NOT NULL, at INTEGER NOT NULL,
  protected INTEGER NOT NULL,          -- 1 = delete was refused
  detail TEXT NOT NULL DEFAULT ''      -- scrubbed status/error
);`,
	},
	{
		// Off-site replication history: one row per `restic copy` run (begin/end,
		// outcome, scrubbed error). finished_at NULL = still running.
		version: 41,
		name:    "offsite_runs",
		sql: `CREATE TABLE IF NOT EXISTS offsite_runs (
  domain TEXT NOT NULL, started_at INTEGER NOT NULL, finished_at INTEGER,
  ok INTEGER NOT NULL DEFAULT 0, error TEXT NOT NULL DEFAULT ''
);`,
	},
	{
		// Drill kind: 'subset' = the existing `restic check --read-data-subset`
		// verification; 'dr' = a real sandbox restore from the off-site repo.
		version: 42,
		name:    "restore_drills_kind",
		sql:     "ALTER TABLE restore_drills ADD COLUMN kind TEXT NOT NULL DEFAULT 'subset';",
	},
	{
		// Covering indexes for the history tables' hot query shape: "latest row for
		// this domain" (tamper_tests, offsite_runs) and "latest drill for this
		// domain+source+kind" (restore_drills). Each lookup filters by domain (+source
		// +kind) and orders by the timestamp DESC, so these indexes let SQLite skip a
		// full-table scan as the history grows. IF NOT EXISTS keeps it idempotent.
		version: 43,
		name:    "history_indexes",
		sql: `CREATE INDEX IF NOT EXISTS idx_tamper_tests_domain_at ON tamper_tests(domain, at);
CREATE INDEX IF NOT EXISTS idx_offsite_runs_domain_started ON offsite_runs(domain, started_at);
CREATE INDEX IF NOT EXISTS idx_restore_drills_domain_source_kind_at ON restore_drills(domain, source, kind, at);`,
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
