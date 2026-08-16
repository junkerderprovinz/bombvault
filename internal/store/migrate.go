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
	{
		// The `config` self-backup domain: BombVault backs up its own /config folder
		// (settings DB + rclone.conf + ssh keypair). Mirrors the flash domain's
		// settings columns. One ALTER per column (SQLite ADD COLUMN is single-column).
		version: 44, name: "settings_config_enabled",
		sql: "ALTER TABLE settings ADD COLUMN config_enabled INTEGER NOT NULL DEFAULT 0;",
	},
	{
		version: 45, name: "settings_config_path",
		sql: "ALTER TABLE settings ADD COLUMN config_path TEXT NOT NULL DEFAULT 'user/bombvault/config';",
	},
	{
		version: 46, name: "settings_config_schedule",
		sql: "ALTER TABLE settings ADD COLUMN config_schedule TEXT NOT NULL DEFAULT 'off';",
	},
	{
		version: 47, name: "settings_config_offsite",
		sql: "ALTER TABLE settings ADD COLUMN config_offsite TEXT NOT NULL DEFAULT '';",
	},
	{
		version: 48, name: "settings_config_offsite_schedule",
		sql: "ALTER TABLE settings ADD COLUMN config_offsite_schedule TEXT NOT NULL DEFAULT '';",
	},
	{
		version: 49, name: "settings_config_offsite_immutable",
		sql: "ALTER TABLE settings ADD COLUMN config_offsite_immutable INTEGER NOT NULL DEFAULT 0;",
	},
	{
		// Scheduled flash ZIP export (#28): after a successful flash backup, write
		// the snapshot out as a plain .zip to a user-chosen folder for off-server
		// sync. One ALTER per column (SQLite ADD COLUMN is single-column).
		version: 50, name: "settings_flash_zip_export_enabled",
		sql: "ALTER TABLE settings ADD COLUMN flash_zip_export_enabled INTEGER NOT NULL DEFAULT 0;",
	},
	{
		version: 51, name: "settings_flash_zip_export_path",
		sql: "ALTER TABLE settings ADD COLUMN flash_zip_export_path TEXT NOT NULL DEFAULT '';",
	},
	{
		version: 52, name: "settings_flash_zip_export_keep",
		sql: "ALTER TABLE settings ADD COLUMN flash_zip_export_keep INTEGER NOT NULL DEFAULT 0;",
	},
	{
		// Per-container restic --exclude patterns applied to this container's backup.
		// JSON array; '[]' = none. Owned by SetExcludes (never reset by Upsert).
		version: 53, name: "target_excludes",
		sql: "ALTER TABLE targets ADD COLUMN excludes TEXT NOT NULL DEFAULT '[]';",
	},
	{
		// Opt out of the scheduled off-site DR drill (#37). The DR drill re-downloads
		// the whole off-site snapshot each run (egress cost on metered clouds), so this
		// gates ONLY the scheduled off-site drill. DEFAULT 1 (ON) preserves current
		// behavior for upgraders + fresh installs.
		version: 54, name: "settings_offsite_drills_enabled",
		sql: "ALTER TABLE settings ADD COLUMN offsite_drills_enabled INTEGER NOT NULL DEFAULT 1;",
	},
	{
		// Per-container opt-in (#52): after a successful backup, pull the image and
		// recreate the container if a newer image exists. DEFAULT 0 (off) so
		// upgraders and fresh installs never auto-update unless the user enables it.
		version: 55, name: "target_update_after_backup",
		sql: "ALTER TABLE targets ADD COLUMN update_after_backup INTEGER NOT NULL DEFAULT 0;",
	},
	{
		// #55: remember which local repo destinations were successfully established,
		// so a repo that later fails to open (its backing store gone — e.g. a remote
		// share that mounts late at boot) is treated as "not mounted" instead of
		// re-initialised (which would write an empty repo shadowing the real one).
		// Lives in /config (always mounted), so it survives the unmounted window.
		version: 56, name: "established_repos",
		sql: "CREATE TABLE IF NOT EXISTS established_repos (repo TEXT PRIMARY KEY, created_at INTEGER NOT NULL DEFAULT 0);",
	},
	{
		// #56: after a post-backup container update, optionally remove the superseded
		// (old) image. DEFAULT 0 (off) — keeping the old image is what makes a
		// fresh-snapshot rollback cheap, so upgraders/fresh installs never prune unless
		// the user opts in.
		version: 57, name: "settings_prune_image_after_update",
		sql: "ALTER TABLE settings ADD COLUMN prune_image_after_update INTEGER NOT NULL DEFAULT 0;",
	},
	{
		// The `files` domain (#62): BombVault backs up arbitrary host folders as
		// named file sets. Mirrors the config domain's settings columns (v44–v49).
		// One ALTER per column (SQLite ADD COLUMN is single-column).
		version: 58, name: "settings_files_enabled",
		sql: "ALTER TABLE settings ADD COLUMN files_enabled INTEGER NOT NULL DEFAULT 0;",
	},
	{
		version: 59, name: "settings_files_path",
		sql: "ALTER TABLE settings ADD COLUMN files_path TEXT NOT NULL DEFAULT 'user/bombvault/files';",
	},
	{
		version: 60, name: "settings_files_schedule",
		sql: "ALTER TABLE settings ADD COLUMN files_schedule TEXT NOT NULL DEFAULT 'off';",
	},
	{
		version: 61, name: "settings_files_offsite",
		sql: "ALTER TABLE settings ADD COLUMN files_offsite TEXT NOT NULL DEFAULT '';",
	},
	{
		version: 62, name: "settings_files_offsite_schedule",
		sql: "ALTER TABLE settings ADD COLUMN files_offsite_schedule TEXT NOT NULL DEFAULT '';",
	},
	{
		version: 63, name: "settings_files_offsite_immutable",
		sql: "ALTER TABLE settings ADD COLUMN files_offsite_immutable INTEGER NOT NULL DEFAULT 0;",
	},
	{
		// The files domain's item table (#62): one row per named file set (a host
		// folder + optional restic --exclude patterns). `name` is the user-visible
		// label and the restic tag key (fileset:<Name>); `id` is stable so renames
		// never orphan run history (runs.target_id = file_sets.id).
		version: 64, name: "file_sets",
		sql: `CREATE TABLE file_sets (
  id         TEXT    PRIMARY KEY,
  name       TEXT    NOT NULL UNIQUE,
  path       TEXT    NOT NULL,
  excludes   TEXT    NOT NULL DEFAULT '[]',
  enabled    INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL
);`,
	},
	{
		// Session-revocation epoch mixed into every session token's HMAC. Rotating
		// it (POST /api/logout-all) invalidates all outstanding session cookies at
		// once. '' (the default) is a valid legacy epoch, so existing sessions
		// survive the upgrade until the first rotation.
		version: 65, name: "settings_session_epoch",
		sql: "ALTER TABLE settings ADD COLUMN session_epoch TEXT NOT NULL DEFAULT '';",
	},
	{
		// Size cap (MB) for restic's persistent cache under /config (v6.7.0 pinned
		// RESTIC_CACHE_DIR there so the cache survives restarts — but it now grows
		// unbounded). 0 = no limit. The DEFAULT 4096 backfills existing rows AND
		// seeds fresh installs (a new DB replays this migration after the v1 seed
		// row), so both get the 4 GB cap without a separate defaults path.
		version: 66, name: "settings_restic_cache_max_mb",
		sql: "ALTER TABLE settings ADD COLUMN restic_cache_max_mb INTEGER NOT NULL DEFAULT 4096;",
	},
	{
		// "Checked, up to date" update signal per container (#52 follow-up): the
		// post-backup update check deliberately records NO run when nothing newer
		// exists (44 noise rows a night), which made "checked and current"
		// indistinguishable from "never reached". last_update_check stamps when the
		// check last completed; last_update_result carries its outcome
		// ('' = never, 'up-to-date', 'updated', 'failed'). Owned by SetUpdateCheck
		// (never reset by Upsert).
		version: 67, name: "targets_last_update_check",
		sql: `
ALTER TABLE targets ADD COLUMN last_update_check INTEGER NOT NULL DEFAULT 0;
ALTER TABLE targets ADD COLUMN last_update_result TEXT NOT NULL DEFAULT '';`,
	},
	{
		// Weekly digest notification: one summary message per week through the
		// existing notify fan-out (counts, backup bytes, off-site currency, top
		// failures). Off by default so existing setups are unchanged until the
		// user opts in; the schedule uses the shared cadence grammar.
		version: 68, name: "settings_digest",
		sql: `
ALTER TABLE settings ADD COLUMN digest_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE settings ADD COLUMN digest_schedule TEXT NOT NULL DEFAULT 'weekly Mon 08:00';`,
	},
	{
		// #106: private container-registry credentials for the post-backup update
		// pull (e.g. a sponsor-gated ghcr.io image). Stored like notify_conf /
		// cloud_conf: an AES-256-GCM-encrypted JSON array (base64) of
		// {host, username, token} entries. Empty = anonymous pulls only.
		version: 69, name: "settings_registry_auths",
		sql: "ALTER TABLE settings ADD COLUMN registry_auths TEXT NOT NULL DEFAULT '';",
	},
	{
		// Embeddable dashboard-widget token: the shared secret that authorizes
		// the session-free GET /widget page + GET /api/widget/data feed (the
		// iframe-embeddable mini activity log). Empty (the default) = the widget
		// feature is OFF and both endpoints fail closed with 403.
		version: 70, name: "settings_widget_token",
		sql: "ALTER TABLE settings ADD COLUMN widget_token TEXT NOT NULL DEFAULT '';",
	},
	{
		// Anacron-style catch-up for missed scheduled backups: when the server was
		// off across a scheduled fire (home boxes sleep at night), the missed
		// domain backup runs shortly after the next app start instead of silently
		// waiting a whole cadence period (RPO slowly going red). DEFAULT 1 (ON):
		// catching up is the behaviour a backup tool should have out of the box;
		// the toggle exists for setups where a boot-time backup is unwanted.
		version: 71, name: "settings_catch_up_missed",
		sql: "ALTER TABLE settings ADD COLUMN catch_up_missed INTEGER NOT NULL DEFAULT 1;",
	},
	{
		// Overdue-backup watchdog: a daily scheduled check that actively NOTIFIES
		// (via the existing notify fan-out) when a domain's backups are overdue —
		// today that state is only visible on the dashboard. DEFAULT 1 (ON): the
		// check is silent unless something is actually overdue AND notifications
		// are configured, so it is safe to enable for everyone.
		version: 72, name: "settings_watchdog_enabled",
		sql: "ALTER TABLE settings ADD COLUMN watchdog_enabled INTEGER NOT NULL DEFAULT 1;",
	},
	{
		// Watchdog once-per-episode memory: one row per domain the watchdog has
		// notified for, keyed by the last-success timestamp the overdue verdict was
		// based on. While that timestamp is unchanged the episode is the same and
		// no further notification fires; a new success changes it (or clears the
		// row via the recovery path), re-arming the watchdog for the next episode.
		version: 73, name: "watchdog_state",
		sql: `CREATE TABLE IF NOT EXISTS watchdog_state (
  domain          TEXT    PRIMARY KEY,
  notified_at     INTEGER NOT NULL,
  last_success_at INTEGER NOT NULL
);`,
	},
	{
		// Optional age public-key encryption for the plain export paths (tool-free
		// tar.gz / xml / zip exports; the restic repo is already encrypted). When
		// enabled, exports are sealed to the age recipients below and written with a
		// .age suffix. Recipients are PUBLIC keys (age1... or SSH), so they are not a
		// secret and are stored/returned as plain columns. Off by default so existing
		// setups produce byte-identical plaintext exports until the user opts in.
		version: 74, name: "settings_export_encrypt",
		sql: `
ALTER TABLE settings ADD COLUMN export_encrypt_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE settings ADD COLUMN export_age_recipients  TEXT    NOT NULL DEFAULT '';`,
	},
	{
		// Multi-off-site, stage 1 (data model only, behavior-preserving). Introduce
		// the off-site DESTINATION table `offsite_targets` (the plural successor to
		// the single-repo-per-domain Settings.*Offsite* columns, which are kept
		// intact for backward compatibility), add a target dimension to the off-site
		// history tables, and backfill exactly one 'Primary' target per domain that
		// currently has an off-site repo configured — stamping the existing history
		// rows with the new target's id so nothing is orphaned. Nothing consumes the
		// table in the live path yet; the engine/UI rewire lands in later stages.
		//
		// NAMING TRAP: `targets` already means backup SOURCES (containers). This is a
		// separate concept — off-site destinations — hence `offsite_targets` /
		// `offsite_target_id`, never a reuse of `targets`.
		//
		// storage_class is deliberately NOT backfilled: it lives inside the encrypted
		// cloud_conf blob, which this pure-SQL migration cannot decode (that needs the
		// app secret key). The row is created with storage_class='' and stage 2 copies
		// it once ModeFor becomes target-aware. All other values ARE backfilled.
		//
		// The ALTER TABLE ADD COLUMN steps run exactly once because the version gate
		// (schema_migrations) skips already-applied migrations, so no per-column
		// existence guard is needed (same as every other ADD COLUMN migration here).
		version: 75, name: "offsite_targets",
		sql: `
CREATE TABLE IF NOT EXISTS offsite_targets (
  id                     TEXT    PRIMARY KEY,
  domain                 TEXT    NOT NULL,
  name                   TEXT    NOT NULL DEFAULT '',
  repo                   TEXT    NOT NULL DEFAULT '',
  creds_ref              TEXT    NOT NULL DEFAULT '',
  storage_class          TEXT    NOT NULL DEFAULT '',
  immutable              INTEGER NOT NULL DEFAULT 0,
  schedule               TEXT    NOT NULL DEFAULT '',
  retention_keep_last    INTEGER NOT NULL DEFAULT 0,
  retention_keep_daily   INTEGER NOT NULL DEFAULT 0,
  retention_keep_weekly  INTEGER NOT NULL DEFAULT 0,
  retention_keep_monthly INTEGER NOT NULL DEFAULT 0,
  limit_upload           INTEGER NOT NULL DEFAULT 0,
  limit_download         INTEGER NOT NULL DEFAULT 0,
  growth_budget_gb       INTEGER NOT NULL DEFAULT 0,
  enabled                INTEGER NOT NULL DEFAULT 1,
  created_at             INTEGER NOT NULL DEFAULT 0,
  sort_order             INTEGER NOT NULL DEFAULT 0
);

ALTER TABLE offsite_runs   ADD COLUMN offsite_target_id TEXT NOT NULL DEFAULT '';
ALTER TABLE tamper_tests   ADD COLUMN offsite_target_id TEXT NOT NULL DEFAULT '';
ALTER TABLE restore_drills ADD COLUMN offsite_target_id TEXT NOT NULL DEFAULT '';
ALTER TABLE repo_stats     ADD COLUMN offsite_target_id TEXT NOT NULL DEFAULT '';

-- Backfill one 'Primary' target per domain with a non-empty off-site repo. Each
-- INSERT gets its own fresh 32-hex id (lower(hex(randomblob(16))) mirrors the
-- Go newID() format) and copies that domain's per-domain off-site repo/schedule/
-- immutable plus the GLOBAL retention/limits/growth-budget from the settings row.
INSERT INTO offsite_targets (id, domain, name, repo, creds_ref, storage_class, immutable, schedule,
  retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly,
  limit_upload, limit_download, growth_budget_gb, enabled, created_at, sort_order)
SELECT lower(hex(randomblob(16))), 'containers', 'Primary', containers_offsite, '', '',
  containers_offsite_immutable, containers_offsite_schedule,
  offsite_retention_keep_last, offsite_retention_keep_daily, offsite_retention_keep_weekly, offsite_retention_keep_monthly,
  offsite_limit_upload, offsite_limit_download, offsite_growth_budget_gb, 1, strftime('%s','now'), 0
FROM settings WHERE id = 1 AND containers_offsite <> '';

INSERT INTO offsite_targets (id, domain, name, repo, creds_ref, storage_class, immutable, schedule,
  retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly,
  limit_upload, limit_download, growth_budget_gb, enabled, created_at, sort_order)
SELECT lower(hex(randomblob(16))), 'vms', 'Primary', vms_offsite, '', '',
  vms_offsite_immutable, vms_offsite_schedule,
  offsite_retention_keep_last, offsite_retention_keep_daily, offsite_retention_keep_weekly, offsite_retention_keep_monthly,
  offsite_limit_upload, offsite_limit_download, offsite_growth_budget_gb, 1, strftime('%s','now'), 0
FROM settings WHERE id = 1 AND vms_offsite <> '';

INSERT INTO offsite_targets (id, domain, name, repo, creds_ref, storage_class, immutable, schedule,
  retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly,
  limit_upload, limit_download, growth_budget_gb, enabled, created_at, sort_order)
SELECT lower(hex(randomblob(16))), 'flash', 'Primary', flash_offsite, '', '',
  flash_offsite_immutable, flash_offsite_schedule,
  offsite_retention_keep_last, offsite_retention_keep_daily, offsite_retention_keep_weekly, offsite_retention_keep_monthly,
  offsite_limit_upload, offsite_limit_download, offsite_growth_budget_gb, 1, strftime('%s','now'), 0
FROM settings WHERE id = 1 AND flash_offsite <> '';

INSERT INTO offsite_targets (id, domain, name, repo, creds_ref, storage_class, immutable, schedule,
  retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly,
  limit_upload, limit_download, growth_budget_gb, enabled, created_at, sort_order)
SELECT lower(hex(randomblob(16))), 'config', 'Primary', config_offsite, '', '',
  config_offsite_immutable, config_offsite_schedule,
  offsite_retention_keep_last, offsite_retention_keep_daily, offsite_retention_keep_weekly, offsite_retention_keep_monthly,
  offsite_limit_upload, offsite_limit_download, offsite_growth_budget_gb, 1, strftime('%s','now'), 0
FROM settings WHERE id = 1 AND config_offsite <> '';

INSERT INTO offsite_targets (id, domain, name, repo, creds_ref, storage_class, immutable, schedule,
  retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly,
  limit_upload, limit_download, growth_budget_gb, enabled, created_at, sort_order)
SELECT lower(hex(randomblob(16))), 'files', 'Primary', files_offsite, '', '',
  files_offsite_immutable, files_offsite_schedule,
  offsite_retention_keep_last, offsite_retention_keep_daily, offsite_retention_keep_weekly, offsite_retention_keep_monthly,
  offsite_limit_upload, offsite_limit_download, offsite_growth_budget_gb, 1, strftime('%s','now'), 0
FROM settings WHERE id = 1 AND files_offsite <> '';

-- Stamp the existing off-site history rows with their domain's new target id.
-- The EXISTS guard skips domains that got no target (empty repo), keeping the
-- NOT NULL column at its '' default there instead of writing a NULL.
UPDATE offsite_runs
   SET offsite_target_id = (SELECT ot.id FROM offsite_targets ot WHERE ot.domain = offsite_runs.domain)
 WHERE EXISTS (SELECT 1 FROM offsite_targets ot WHERE ot.domain = offsite_runs.domain);

UPDATE tamper_tests
   SET offsite_target_id = (SELECT ot.id FROM offsite_targets ot WHERE ot.domain = tamper_tests.domain)
 WHERE EXISTS (SELECT 1 FROM offsite_targets ot WHERE ot.domain = tamper_tests.domain);

UPDATE restore_drills
   SET offsite_target_id = (SELECT ot.id FROM offsite_targets ot WHERE ot.domain = restore_drills.domain)
 WHERE source = 'offsite'
   AND EXISTS (SELECT 1 FROM offsite_targets ot WHERE ot.domain = restore_drills.domain);

UPDATE repo_stats
   SET offsite_target_id = (SELECT ot.id FROM offsite_targets ot WHERE ot.domain = repo_stats.domain)
 WHERE source = 'offsite'
   AND EXISTS (SELECT 1 FROM offsite_targets ot WHERE ot.domain = repo_stats.domain);

-- Covering indexes on the new target dimension, parallel to the v43 domain
-- indexes (tamper_tests, offsite_runs, restore_drills), so target-scoped
-- "latest row" lookups skip a full scan once stage 2 queries per target.
CREATE INDEX IF NOT EXISTS idx_tamper_tests_target_at ON tamper_tests(offsite_target_id, at);
CREATE INDEX IF NOT EXISTS idx_offsite_runs_target_started ON offsite_runs(offsite_target_id, started_at);
CREATE INDEX IF NOT EXISTS idx_restore_drills_target_source_kind_at ON restore_drills(offsite_target_id, source, kind, at);`,
	},
	{
		// Receiver dashboard (read-only off-site RECEIVER monitoring). A box that
		// RECEIVES immutable off-site copies registers the received repo here and
		// monitors it READ-ONLY: snapshot inventory grouped by source, an independent
		// restic check on the receiving hardware, and dead-mans-switch alerting. The
		// SENDING instance's APP_KEY is stored ENCRYPTED at rest (internal/secret) in
		// app_key_enc and is only ever decrypted in-engine — never logged, never
		// returned in the clear. last_check_ok is nullable (NULL = never checked yet).
		//
		// receiver_enabled gates the whole feature (default 0 = off), exactly like the
		// files/vms domain toggles. It is added in the same migration so the settings
		// row and the table land together.
		version: 76, name: "received_repos",
		sql: `
CREATE TABLE IF NOT EXISTS received_repos (
  id                   TEXT    PRIMARY KEY,
  name                 TEXT    NOT NULL DEFAULT '',
  repo                 TEXT    NOT NULL DEFAULT '',
  app_key_enc          BLOB    NOT NULL DEFAULT x'',
  dead_man_hours       INTEGER NOT NULL DEFAULT 26,
  check_cadence        TEXT    NOT NULL DEFAULT 'off',
  read_data_percent    INTEGER NOT NULL DEFAULT 0,
  last_check_at        INTEGER NOT NULL DEFAULT 0,
  last_check_ok        INTEGER,                       -- nullable: NULL = never checked
  last_check_error     TEXT    NOT NULL DEFAULT '',
  last_check_read_data INTEGER NOT NULL DEFAULT 0,
  enabled              INTEGER NOT NULL DEFAULT 1,
  created_at           INTEGER NOT NULL DEFAULT 0,
  sort_order           INTEGER NOT NULL DEFAULT 0
);

ALTER TABLE settings ADD COLUMN receiver_enabled INTEGER NOT NULL DEFAULT 0;`,
	},
	{
		// Receiver dead-mans-switch episode memory (one row per stale SOURCE on a
		// received repo), so the scheduled receiver check alerts ONCE per stale
		// episode instead of every tick — the exact once-per-episode discipline
		// watchdog_state gives the overdue-backup watchdog. based_on is the newest
		// snapshot time (unix) the stale verdict was taken against: while it is
		// unchanged the source is still in the SAME episode and the receiver stays
		// quiet; a newer snapshot (the source recovered) changes it or clears the row,
		// re-arming the alert. Keyed by (received_repo_id, source). Integrity alerts
		// need no state here — they debounce off received_repos.last_check_ok.
		version: 77, name: "received_alert_state",
		sql: `
CREATE TABLE IF NOT EXISTS received_alert_state (
  received_repo_id TEXT    NOT NULL,
  source           TEXT    NOT NULL,
  notified_at      INTEGER NOT NULL,
  based_on         INTEGER NOT NULL,
  PRIMARY KEY (received_repo_id, source)
);`,
	},
	{
		// Per-container manual backup order (#119): the explicit sequence a scheduled
		// or multi-select batch run processes containers in. 0 (the default) means
		// "unordered" — those containers keep the existing most-overdue-first order as
		// the tiebreak, so existing setups are byte-for-byte unchanged until the user
		// assigns explicit orders. Owned by SetBackupOrder (never reset by UpsertTarget).
		version: 78, name: "target_backup_order",
		sql: "ALTER TABLE targets ADD COLUMN backup_order INTEGER NOT NULL DEFAULT 0;",
	},
	{
		// Health-gated ordered restart of the "stop other containers during backup"
		// set (#119). restart_health_wait DEFAULT 1 (ON): after a backup the stopped
		// dependencies are brought back in compose depends_on order and, with this on,
		// the restart WAITS for each to become healthy (or Running plus a short grace
		// when it has no healthcheck) before starting the containers that depend on it,
		// so a dependency is never beaten to a start by the service that needs it. The
		// depends_on ordering itself is always applied, independent of this flag.
		// restart_health_timeout_sec DEFAULT 120 caps that per-container wait, after
		// which the restart warns and proceeds so the backup flow never hangs.
		version: 79, name: "settings_restart_health_wait",
		sql: `
ALTER TABLE settings ADD COLUMN restart_health_wait        INTEGER NOT NULL DEFAULT 1;
ALTER TABLE settings ADD COLUMN restart_health_timeout_sec INTEGER NOT NULL DEFAULT 120;`,
	},
	{
		// After BombVault recreates a container in the post-backup update step (#116),
		// Unraid's Docker tab still shows a stale "update available" banner because
		// Unraid did not perform the update and never refreshed its cached status file.
		// reconcile_unraid_update_status DEFAULT 1 (ON): BombVault asks Unraid to run
		// its OWN update-status recheck for that container over the existing host SSH
		// link, so Unraid rewrites the file itself (never a racing BombVault write).
		// Best-effort and non-fatal, so a reconcile failure never affects the backup.
		version: 80, name: "settings_reconcile_unraid_update_status",
		sql: "ALTER TABLE settings ADD COLUMN reconcile_unraid_update_status INTEGER NOT NULL DEFAULT 1;",
	},
	{
		// Per-item schedule overrides (#121). An OPT-IN feature: per_item_schedules
		// DEFAULT 0 (off) keeps the per-domain schedule authoritative for everyone, so
		// existing setups are byte-for-byte unchanged. schedule_cadence on targets/vms
		// is an OPTIONAL per-item cadence (same grammar as the domain schedules); '' (the
		// default) means "use the domain default exactly as today". Only consulted when
		// per_item_schedules is on. Owned by SetScheduleCadence/SetVMScheduleCadence
		// (never reset by UpsertTarget/UpsertVMTarget).
		version: 81, name: "per_item_schedules",
		sql: `
ALTER TABLE settings ADD COLUMN per_item_schedules INTEGER NOT NULL DEFAULT 0;
ALTER TABLE targets  ADD COLUMN schedule_cadence   TEXT    NOT NULL DEFAULT '';
ALTER TABLE vms      ADD COLUMN schedule_cadence   TEXT    NOT NULL DEFAULT '';`,
	},
	{
		// #119, VMs: give VMs the same explicit manual backup order that containers
		// have (targets.backup_order, v78). 0 (the default) means "unordered" — those
		// VMs keep the existing name-order tiebreak, so existing setups are unchanged
		// until the user assigns explicit orders. Owned by SetVMBackupOrders (never
		// reset by UpsertVMTarget).
		version: 82, name: "vm_backup_order",
		sql: "ALTER TABLE vms ADD COLUMN backup_order INTEGER NOT NULL DEFAULT 0;",
	},
	{
		// #126: acknowledged lets the dashboard's error panel dismiss a failed run
		// from the UI (per-group "Resolve" or "Mark all resolved") without editing
		// SQLite by hand. DEFAULT 0 (unacknowledged) keeps every existing failed run
		// counted until the user resolves it.
		version: 83, name: "runs_acknowledged",
		sql: "ALTER TABLE runs ADD COLUMN acknowledged INTEGER NOT NULL DEFAULT 0;",
	},
	{
		// v8.0.0, VM DR drills: the VM the real-restore DR drill restores, mirroring
		// dr_drill_target (v39) for containers. Empty (the default) = auto: the most
		// recently successfully backed-up VM.
		version: 84, name: "settings_dr_drill_target_vm",
		sql: "ALTER TABLE settings ADD COLUMN dr_drill_target_vm TEXT NOT NULL DEFAULT '';",
	},
	{
		// #141: named, per-off-site-target S3/restic-REST credential sets ("stage 2",
		// see offsite_targets.CredsRef's doc comment from v75/offsite_targets). Every
		// existing off-site target keeps working unchanged (CredsRef stays empty =
		// the shared/global cloud_conf creds, today's behavior) — this only adds the
		// STORAGE for additional named sets a target can opt into. cloud_cred_sets is
		// an AES-256-GCM-encrypted JSON array (base64), same at-rest treatment as
		// cloud_conf. Empty (the default) = no additional sets defined yet.
		version: 85, name: "settings_cloud_cred_sets",
		sql: "ALTER TABLE settings ADD COLUMN cloud_cred_sets TEXT NOT NULL DEFAULT '';",
	},
	{
		// v8.0.0, Fleet view: a registry of peer BombVault instances this instance
		// polls (read-only) for their protection status. Mirrors received_repos (v76)
		// in shape: a named row with a location + encrypted credential + last-poll-
		// verdict columns. token_enc holds the PEER's fleet_token (the credential this
		// instance presents when polling that peer), encrypted at rest like
		// received_repos.app_key_enc — never logged, never returned in the clear.
		// last_poll_domains_json caches the peer's most recent DomainStatusEntry[]
		// response verbatim, so the Fleet page can render a peer's scorecard without
		// a live round-trip on every page load.
		//
		// fleet_enabled gates the whole feature (default 0 = off), same as
		// receiver_enabled. instance_name is this instance's own display name,
		// reported to polling peers. fleet_token is this instance's own credential
		// that PEERS present to poll THIS instance — mirrors widget_token exactly
		// (empty = feature off, fails closed). All three land in this migration so
		// the table and the settings row arrive together.
		version: 86, name: "fleet_peers",
		sql: `
CREATE TABLE IF NOT EXISTS fleet_peers (
  id                      TEXT    PRIMARY KEY,
  name                    TEXT    NOT NULL DEFAULT '',
  url                     TEXT    NOT NULL DEFAULT '',
  token_enc               BLOB    NOT NULL DEFAULT x'',
  enabled                 INTEGER NOT NULL DEFAULT 1,
  last_poll_at            INTEGER NOT NULL DEFAULT 0,
  last_poll_ok            INTEGER,                       -- nullable: NULL = never polled
  last_poll_error         TEXT    NOT NULL DEFAULT '',
  last_poll_instance_name TEXT    NOT NULL DEFAULT '',
  last_poll_version       TEXT    NOT NULL DEFAULT '',
  last_poll_domains_json  TEXT    NOT NULL DEFAULT '',
  created_at              INTEGER NOT NULL DEFAULT 0,
  sort_order              INTEGER NOT NULL DEFAULT 0
);

ALTER TABLE settings ADD COLUMN fleet_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE settings ADD COLUMN instance_name TEXT NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN fleet_token TEXT NOT NULL DEFAULT '';`,
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
