package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// Settings mirrors the single-row settings table.
type Settings struct {
	EncryptionEnabled bool
	ContainersEnabled bool
	VMsEnabled        bool
	FlashEnabled      bool
	ConfigEnabled     bool
	FilesEnabled      bool
	ContainersPath    string
	VMsPath           string
	FlashPath         string
	ConfigPath        string
	FilesPath         string
	// RestoreFolder is the default folder for "restore to a folder": a relative
	// subpath under the host mount that pre-fills the restore-to-folder picker
	// (same style as the backup-path settings).
	RestoreFolder string
	// Optional off-site repo per domain. When set, a successful local backup is
	// replicated there with `restic copy` (the local repo stays primary). Empty
	// means no off-site copy for that domain.
	ContainersOffsite string
	VMsOffsite        string
	FlashOffsite      string
	ConfigOffsite     string
	FilesOffsite      string
	// Optional off-site replication schedule per domain (same cadence grammar as
	// the backup schedules). Empty = replicate after every local backup; set =
	// replicate ONLY on this cadence (decoupled from the backup schedule).
	ContainersOffsiteSchedule string
	VMsOffsiteSchedule        string
	FlashOffsiteSchedule      string
	ConfigOffsiteSchedule     string
	FilesOffsiteSchedule      string
	ContainersSchedule        string
	VMsSchedule               string
	FlashSchedule             string
	ConfigSchedule            string
	FilesSchedule             string
	// Scheduled flash ZIP export: after a successful flash backup, write the
	// snapshot out as a plain .zip to FlashZipExportPath (a relative subpath under
	// the host mount root) for off-server sync. Disabled by default. Keep = how
	// many timestamped zips to retain (0 = a single overwriting flash-latest.zip).
	FlashZipExportEnabled bool
	FlashZipExportPath    string
	FlashZipExportKeep    int
	DefaultLanguage       string
	// AuthPasswordHash is the HMAC-SHA256 password hash set by the admin.
	// An empty string means authentication is disabled (the default).
	AuthPasswordHash string
	// SessionEpoch is mixed into every session token's HMAC. Rotating it to a
	// fresh random value (POST /api/logout-all) invalidates ALL outstanding
	// session cookies at once — the only revocation path for the otherwise
	// stateless tokens. Empty (the default) is a valid legacy epoch: sessions
	// minted before this column existed keep working until the first rotation.
	SessionEpoch string
	// Retention keep-policy (global, applied via `restic forget --prune` after
	// each successful backup). All zero = retention off (snapshots kept forever).
	RetentionKeepLast    int
	RetentionKeepDaily   int
	RetentionKeepWeekly  int
	RetentionKeepMonthly int
	// Off-site retention keep-policy: a SEPARATE policy applied to the off-site
	// repo (e.g. keep longer as an archive than the local copy). All zero = no
	// off-site pruning (the off-site repo keeps everything — the default, so an
	// existing off-site repo is never silently trimmed when this ships).
	OffsiteRetentionKeepLast    int
	OffsiteRetentionKeepDaily   int
	OffsiteRetentionKeepWeekly  int
	OffsiteRetentionKeepMonthly int
	// Off-site transfer bandwidth caps (KiB/s) passed to restic's global
	// --limit-upload / --limit-download for off-site replication (and remote
	// backups). 0 = unlimited (the default), so the WAN is never throttled until
	// the user opts in.
	OffsiteLimitUpload   int
	OffsiteLimitDownload int
	// BackupCores caps the CPU threads each restic child process may use, handed
	// to it as GOMAXPROCS ([558], issue #189). 0 = every core, restic's own
	// default, so an installation that never touches this behaves as before.
	// The bandwidth caps above throttle the WAN; this one throttles the CPU, and
	// until now nothing did — a 12-thread box sat at 99% on every core and 100 °C
	// for the length of a backup.
	BackupCores int
	// RcloneConf is the rclone configuration (INI) for off-site repos, stored
	// AES-256-GCM-encrypted at rest. Empty means no rclone backends configured.
	RcloneConf string
	// NotifyConf is the notification config (webhook / Matrix / Healthchecks) as
	// an AES-256-GCM-encrypted JSON blob (base64). Empty means notifications off.
	NotifyConf string
	// CloudConf is the cloud-backend credentials (S3 keys, restic-REST auth) for
	// off-site repos, an AES-256-GCM-encrypted JSON blob (base64). Empty = none.
	CloudConf string
	// RegistryAuths holds private container-registry credentials for the
	// post-backup update pull (#106), an AES-256-GCM-encrypted JSON array
	// (base64) of {host, username, token} entries. Empty = anonymous pulls only.
	RegistryAuths string
	// MetricsEnabled exposes the Prometheus-format /metrics endpoint when true.
	// Default false (opt-in): when off, /metrics returns 404 and is not served.
	MetricsEnabled bool
	// MetricsToken is an optional bearer token for /metrics. When set, a scrape
	// must send `Authorization: Bearer <token>`; empty means open (LAN trust
	// model, like /api/health). The endpoint exposes only non-sensitive metrics.
	MetricsToken string
	// WidgetToken authorizes the session-free embeddable dashboard widget
	// (GET /widget + GET /api/widget/data, via ?token= or X-Widget-Token).
	// Empty (the default) = widget OFF; both endpoints fail closed with 403.
	// Unlike MetricsToken it is never optional-open: no token, no widget.
	WidgetToken string
	// InstanceName is this instance's own display name, reported to polling
	// fleet peers in GET /api/fleet/status so a Fleet page can label this box.
	// Empty is allowed (the Fleet page falls back to the URL it polled).
	InstanceName string
	// FleetToken authorizes the session-free peer status endpoint
	// (GET /api/fleet/status, via ?token= or X-Fleet-Token) that OTHER instances'
	// Fleet views poll on THIS instance. Empty (the default) = fleet polling of
	// THIS instance OFF; the endpoint fails closed with 403. Mirrors WidgetToken
	// exactly (never optional-open, unlike MetricsToken).
	FleetToken string
	// DrillsEnabled turns on scheduled restore-verification drills. Off by default
	// (drills read back real pack data, so they cost I/O), so existing setups are
	// unchanged until the user opts in.
	DrillsEnabled bool
	// OffsiteDrillsEnabled gates ONLY the scheduled off-site DR drill; default on
	// (true) so upgrades preserve current behavior. When off, the scheduled off-site
	// DR drill is skipped — the free scheduled local integrity check keeps running
	// and the off-site DR check can still be run manually.
	OffsiteDrillsEnabled bool
	// DrillsSchedule is the cadence for scheduled drills (same grammar as the backup
	// schedules). 'off' (the default) = no scheduled drills.
	DrillsSchedule string
	// DrillsSubsetPct is the percentage of pack data each drill reads back and
	// re-verifies (`restic check --read-data-subset`). Clamped 1..100; defaults to 5.
	DrillsSubsetPct int
	// RecoveryKitAck records that the user has downloaded + safely stored the
	// encryption-key recovery kit, so the dashboard nag can be dismissed. Default
	// false (the nag shows while encryption is on and this is unset).
	RecoveryKitAck bool
	// Per-domain "off-site repo is append-only (immutable)" flag. The far side
	// (e.g. rest-server --append-only) enforces it; with the flag set BombVault
	// skips its own off-site retention prune and refuses off-site deletes.
	ContainersOffsiteImmutable bool
	VMsOffsiteImmutable        bool
	FlashOffsiteImmutable      bool
	ConfigOffsiteImmutable     bool
	FilesOffsiteImmutable      bool
	// OffsiteGrowthBudgetGB caps how large an (only-growing) append-only off-site
	// repo may get before a notification fires — detection, not prevention.
	// 0 = budget alarm off (the default).
	OffsiteGrowthBudgetGB int
	// TamperTestSchedule is the cadence for the scheduled off-site tamper test
	// (same grammar as the backup schedules). Defaults to "weekly Sun 04:30".
	TamperTestSchedule string
	// DRDrillTarget is the container the real-restore DR drill restores. Empty
	// (the default) = auto: the most recently successfully backed-up container.
	DRDrillTarget string
	// DRDrillTargetVM is the VM the real-restore DR drill restores. Empty (the
	// default) = auto: the most recently successfully backed-up VM. Mirrors
	// DRDrillTarget exactly, one field per domain that needs a pinned target.
	DRDrillTargetVM string
	// PruneImageAfterUpdate removes the superseded (old) image after a post-backup
	// container update (#52/#56). Opt-in, default off — keeping the old image is what
	// makes a fresh-snapshot rollback cheap. Best-effort + force=false (a shared base
	// image is never deleted).
	PruneImageAfterUpdate bool
	// ResticCacheMaxMB caps restic's persistent cache (RESTIC_CACHE_DIR under
	// /config, which survives restarts and therefore grows unbounded). When the
	// cache exceeds this many MB, the least-recently-used per-repo cache
	// subdirectories are evicted after scheduled runs. 0 = no size limit.
	// Defaults to 4096 (4 GB).
	ResticCacheMaxMB int
	// DigestEnabled turns on the scheduled weekly digest: ONE summary message
	// (per-kind run counts, backup bytes, off-site currency, top failures)
	// through the existing notify fan-out. Off by default.
	DigestEnabled bool
	// DigestSchedule is the digest cadence (same grammar as the backup
	// schedules). Defaults to "weekly Mon 08:00".
	DigestSchedule string
	// CatchUpMissed runs a scheduled backup that was MISSED while the app was
	// down (the server was off across the scheduled fire) shortly after the next
	// start, anacron-style. Default on.
	CatchUpMissed bool
	// WatchdogEnabled turns on the daily overdue-backup watchdog: an active
	// notification (once per overdue episode) when a domain's backups are
	// overdue by the dashboard's own RPO rule. Default on.
	WatchdogEnabled bool
	// ExportEncryptEnabled seals the PLAIN export paths (tool-free tar.gz / xml /
	// zip exports) with age public-key encryption when true. Off by default, so
	// exports stay byte-identical plaintext until the user opts in. The restic
	// repository is always encrypted independently of this.
	ExportEncryptEnabled bool
	// ExportAgeRecipients is the whitespace/newline-separated list of age recipients
	// (age1... public keys or SSH public keys) the exports are encrypted to. These
	// are PUBLIC keys, so this is NOT a secret and is stored/returned as-is. With
	// ExportEncryptEnabled on and this empty/invalid, every export path fails loudly
	// rather than writing plaintext.
	ExportAgeRecipients string
	// ReceiverEnabled gates the read-only receiver dashboard (a box that RECEIVES
	// immutable off-site copies and monitors the received repo). Default false
	// (opt-in), exactly like the Files/VMs domain tabs.
	ReceiverEnabled bool
	// FleetEnabled gates the read-only Fleet view (a list of peer BombVault
	// instances this box polls for their protection status). Default false
	// (opt-in), exactly like ReceiverEnabled.
	FleetEnabled bool
	// RestartHealthWait gates the health-gated ordered restart of the "stop other
	// containers during backup" set (#119). The depends_on ORDERING is always
	// applied; this flag governs only whether the restart also WAITS for each
	// dependency to become healthy (or Running plus a short grace when it has no
	// healthcheck) before starting the containers that depend on it. Default on
	// (true) so the reported failure — a dependency coming back after the service
	// that needs it, leaving it stopped — is fixed out of the box.
	RestartHealthWait bool
	// RestartHealthTimeoutSec is the per-container cap (seconds) for that health
	// wait: once it elapses the restart logs a warning and proceeds to the
	// dependents anyway, so the backup flow never hangs forever. Default 120.
	RestartHealthTimeoutSec int
	// ReconcileUnraidUpdateStatus asks Unraid to refresh its OWN cached container
	// "update available" status after BombVault recreates a container in the
	// post-backup update step (#116). BombVault moved the image to the new digest,
	// but Unraid did not do the update and never rechecked, so its Docker tab keeps
	// showing a stale banner. When on, BombVault runs Unraid's own update-status
	// recheck over the existing host SSH link so Unraid rewrites its status file
	// itself (never a racing BombVault write). Default on; best-effort and
	// non-fatal, so a reconcile failure never affects the backup or update.
	ReconcileUnraidUpdateStatus bool
	// PerItemSchedules opts into per-container/VM schedule overrides (#121). Default
	// false: the per-domain schedule stays authoritative for every item, so the
	// scheduler behaves byte-for-byte as before. When true, an included item with a
	// non-empty ScheduleCadence is backed up on its OWN cadence; an item with an
	// empty override still follows its domain schedule exactly as today.
	PerItemSchedules bool
	// CloudCredSets holds additional NAMED S3/restic-REST credential sets (#141
	// stage 2), an AES-256-GCM-encrypted JSON array (base64). An off-site target
	// opts into one via its CredsRef; empty CredsRef keeps using CloudConf (the
	// original single/shared credentials), so every existing install is unaffected.
	// Empty = no additional sets defined.
	CloudCredSets string
	// EverythingSchedule is the cadence for the "Backup Everything" pass: a 6th,
	// independent pseudo-domain that runs containers, vms, flash, files, and
	// config in sequence, then fires EverythingPostHook exactly once. 'off' (the
	// default) leaves it fully inert — it does not replace or gate the five
	// domains' own schedules; a user can run both, and the Settings UI carries an
	// explicit overlap warning.
	EverythingSchedule string
	// EverythingPreHook / EverythingPostHook are optional shell commands run in
	// BombVault's OWN container (via `sh -c`, HostShell) before/after the whole
	// "Backup Everything" pass — unlike the per-container Target.PreHook/PostHook,
	// which exec INSIDE the target container, a whole-pass hook has no single
	// container to run in. The pre-hook is best-effort (logged on failure, never
	// aborts the pass); the post-hook fires unconditionally exactly once, after
	// every domain step has been attempted, regardless of outcome — the explicit
	// dead-man's-switch requirement. Empty (the default) = no hook configured.
	EverythingPreHook  string
	EverythingPostHook string
}

// settingsQuerier / settingsExecer are the two halves of the database/sql API
// the settings row needs, satisfied by BOTH *sql.DB and *sql.Tx. They exist so
// the read and the write below have exactly one implementation each, used
// unchanged inside MutateSettings' transaction — a second, transaction-only
// copy of that 80-column scan/UPDATE pair is precisely the kind of duplicate
// that drifts out of step the next time a column is added.
type settingsQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

type settingsExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// GetSettings returns the current app settings.
//
// A plain read is safe on its own. What is NOT safe is pairing it with
// UpdateSettings to change one field: the write is a full-row UPDATE, so
// anything another writer changed in between is reverted across every column.
// Use MutateSettings for that — see its own doc.
func (r *Repo) GetSettings() (Settings, error) {
	return getSettings(r.db)
}

func getSettings(q settingsQuerier) (Settings, error) {
	row := q.QueryRow(`
		SELECT encryption_enabled, containers_enabled, vms_enabled, flash_enabled, config_enabled, files_enabled,
		       containers_path, vms_path, flash_path, config_path, files_path, restore_folder,
		       containers_offsite, vms_offsite, flash_offsite, config_offsite, files_offsite,
		       containers_offsite_schedule, vms_offsite_schedule, flash_offsite_schedule, config_offsite_schedule, files_offsite_schedule,
		       containers_schedule, vms_schedule, flash_schedule, config_schedule, files_schedule,
		       default_language, auth_password_hash,
		       retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly,
		       offsite_retention_keep_last, offsite_retention_keep_daily, offsite_retention_keep_weekly, offsite_retention_keep_monthly,
		       offsite_limit_upload, offsite_limit_download,
		       rclone_conf, notify_conf, cloud_conf, registry_auths,
		       metrics_enabled, metrics_token, widget_token,
		       drills_enabled, drills_schedule, drills_subset_pct, offsite_drills_enabled,
		       recovery_kit_ack,
		       containers_offsite_immutable, vms_offsite_immutable, flash_offsite_immutable, config_offsite_immutable, files_offsite_immutable,
		       offsite_growth_budget_gb, tamper_test_schedule, dr_drill_target, dr_drill_target_vm,
		       flash_zip_export_enabled, flash_zip_export_path, flash_zip_export_keep,
		       prune_image_after_update, session_epoch, restic_cache_max_mb,
		       digest_enabled, digest_schedule,
		       catch_up_missed, watchdog_enabled,
		       export_encrypt_enabled, export_age_recipients,
		       receiver_enabled,
		       restart_health_wait, restart_health_timeout_sec,
		       reconcile_unraid_update_status,
		       per_item_schedules,
		       cloud_cred_sets,
		       fleet_enabled, instance_name, fleet_token,
		       everything_schedule, everything_pre_hook, everything_post_hook,
		       backup_cores
		FROM settings WHERE id = 1`)

	var s Settings
	var encEnabled, contEnabled, vmsEnabled, flashEnabled, configEnabled, filesEnabled, metricsEnabled, drillsEnabled, offsiteDrillsEnabled, recoveryKitAck int
	var contImmutable, vmsImmutable, flashImmutable, configImmutable, filesImmutable int
	var flashZipExportEnabled, pruneImageAfterUpdate, digestEnabled int
	var catchUpMissed, watchdogEnabled, exportEncryptEnabled, receiverEnabled int
	var restartHealthWait, reconcileUnraidUpdateStatus, perItemSchedules int
	var fleetEnabled int
	err := row.Scan(
		&encEnabled, &contEnabled, &vmsEnabled, &flashEnabled, &configEnabled, &filesEnabled,
		&s.ContainersPath, &s.VMsPath, &s.FlashPath, &s.ConfigPath, &s.FilesPath, &s.RestoreFolder,
		&s.ContainersOffsite, &s.VMsOffsite, &s.FlashOffsite, &s.ConfigOffsite, &s.FilesOffsite,
		&s.ContainersOffsiteSchedule, &s.VMsOffsiteSchedule, &s.FlashOffsiteSchedule, &s.ConfigOffsiteSchedule, &s.FilesOffsiteSchedule,
		&s.ContainersSchedule, &s.VMsSchedule, &s.FlashSchedule, &s.ConfigSchedule, &s.FilesSchedule,
		&s.DefaultLanguage, &s.AuthPasswordHash,
		&s.RetentionKeepLast, &s.RetentionKeepDaily, &s.RetentionKeepWeekly, &s.RetentionKeepMonthly,
		&s.OffsiteRetentionKeepLast, &s.OffsiteRetentionKeepDaily, &s.OffsiteRetentionKeepWeekly, &s.OffsiteRetentionKeepMonthly,
		&s.OffsiteLimitUpload, &s.OffsiteLimitDownload,
		&s.RcloneConf, &s.NotifyConf, &s.CloudConf, &s.RegistryAuths,
		&metricsEnabled, &s.MetricsToken, &s.WidgetToken,
		&drillsEnabled, &s.DrillsSchedule, &s.DrillsSubsetPct, &offsiteDrillsEnabled,
		&recoveryKitAck,
		&contImmutable, &vmsImmutable, &flashImmutable, &configImmutable, &filesImmutable,
		&s.OffsiteGrowthBudgetGB, &s.TamperTestSchedule, &s.DRDrillTarget, &s.DRDrillTargetVM,
		&flashZipExportEnabled, &s.FlashZipExportPath, &s.FlashZipExportKeep,
		&pruneImageAfterUpdate, &s.SessionEpoch, &s.ResticCacheMaxMB,
		&digestEnabled, &s.DigestSchedule,
		&catchUpMissed, &watchdogEnabled,
		&exportEncryptEnabled, &s.ExportAgeRecipients,
		&receiverEnabled,
		&restartHealthWait, &s.RestartHealthTimeoutSec,
		&reconcileUnraidUpdateStatus,
		&perItemSchedules,
		&s.CloudCredSets,
		&fleetEnabled, &s.InstanceName, &s.FleetToken,
		&s.EverythingSchedule, &s.EverythingPreHook, &s.EverythingPostHook,
		&s.BackupCores,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Settings{}, fmt.Errorf("settings row missing: run Migrate first")
	}
	if err != nil {
		return Settings{}, fmt.Errorf("GetSettings: %w", err)
	}
	s.EncryptionEnabled = encEnabled != 0
	s.ContainersEnabled = contEnabled != 0
	s.VMsEnabled = vmsEnabled != 0
	s.FlashEnabled = flashEnabled != 0
	s.ConfigEnabled = configEnabled != 0
	s.FilesEnabled = filesEnabled != 0
	s.MetricsEnabled = metricsEnabled != 0
	s.DrillsEnabled = drillsEnabled != 0
	s.OffsiteDrillsEnabled = offsiteDrillsEnabled != 0
	s.RecoveryKitAck = recoveryKitAck != 0
	s.ContainersOffsiteImmutable = contImmutable != 0
	s.VMsOffsiteImmutable = vmsImmutable != 0
	s.FlashOffsiteImmutable = flashImmutable != 0
	s.ConfigOffsiteImmutable = configImmutable != 0
	s.FilesOffsiteImmutable = filesImmutable != 0
	s.FlashZipExportEnabled = flashZipExportEnabled != 0
	s.PruneImageAfterUpdate = pruneImageAfterUpdate != 0
	s.DigestEnabled = digestEnabled != 0
	s.CatchUpMissed = catchUpMissed != 0
	s.WatchdogEnabled = watchdogEnabled != 0
	s.ExportEncryptEnabled = exportEncryptEnabled != 0
	s.ReceiverEnabled = receiverEnabled != 0
	s.RestartHealthWait = restartHealthWait != 0
	s.ReconcileUnraidUpdateStatus = reconcileUnraidUpdateStatus != 0
	s.PerItemSchedules = perItemSchedules != 0
	s.FleetEnabled = fleetEnabled != 0
	return s, nil
}

// UpdateSettings persists s back to the single settings row as ONE full-row
// UPDATE: every column is written from s, including the ones the caller never
// looked at.
//
// It is therefore a REPLACE, not a patch, and must only be used where s is the
// whole intended new state of the row. Reading with GetSettings, changing one
// field and writing the result back with UpdateSettings is a lost-update bug,
// not a patch: whatever another writer stored in between is reverted across
// every column, and the writer that lost is told its save succeeded. Use
// MutateSettings for any read-modify-write; internal/store/settings_writers_test.go
// keeps production code off this method.
func (r *Repo) UpdateSettings(s Settings) error {
	r.settingsMu.Lock()
	defer r.settingsMu.Unlock()
	return updateSettings(r.db, s)
}

// MutateSettings applies fn to the CURRENT settings row and writes the result
// back, with the read, the mutation and the write inside ONE serialized
// transaction. It returns the settings as they now stand.
//
// This is the only safe way to change part of the row. The alternative —
// GetSettings, edit a field, UpdateSettings — spans a window the caller does
// not control (a probe, an HTTP round-trip, a restic run), and because the
// write is a full-row UPDATE every column another writer touched inside that
// window is silently reverted. Here the read happens under the same lock and
// transaction as the write, so fn always sees the freshest row and can only
// change what it actually sets.
//
// When fn leaves the settings byte-identical, NO write is issued at all: a
// no-op detection or acknowledgement never bumps the row.
//
// fn must be pure — it must not touch this Repo (or the database at all).
// The transaction holds the single pooled connection (store.Open sets
// MaxOpenConns(1)), so a nested store call inside fn would deadlock.
func (r *Repo) MutateSettings(fn func(*Settings) error) (Settings, error) {
	if fn == nil {
		return Settings{}, fmt.Errorf("MutateSettings: nil mutation")
	}
	r.settingsMu.Lock()
	defer r.settingsMu.Unlock()

	tx, err := r.db.Begin()
	if err != nil {
		return Settings{}, fmt.Errorf("MutateSettings: begin: %w", err)
	}
	//nolint:errcheck // no-op once Commit succeeded; on every error path the
	// rollback error is not actionable and must not mask the original error.
	defer tx.Rollback()

	before, err := getSettings(tx)
	if err != nil {
		return Settings{}, err
	}
	after := before
	if err := fn(&after); err != nil {
		return Settings{}, err
	}
	if after != before {
		if err := updateSettings(tx, after); err != nil {
			return Settings{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Settings{}, fmt.Errorf("MutateSettings: commit: %w", err)
	}
	return after, nil
}

func updateSettings(e settingsExecer, s Settings) error {
	_, err := e.Exec(`
		UPDATE settings SET
		  encryption_enabled  = ?,
		  containers_enabled  = ?,
		  vms_enabled         = ?,
		  flash_enabled       = ?,
		  config_enabled      = ?,
		  files_enabled       = ?,
		  containers_path     = ?,
		  vms_path            = ?,
		  flash_path          = ?,
		  config_path         = ?,
		  files_path          = ?,
		  restore_folder      = ?,
		  containers_offsite  = ?,
		  vms_offsite         = ?,
		  flash_offsite       = ?,
		  config_offsite      = ?,
		  files_offsite       = ?,
		  containers_offsite_schedule = ?,
		  vms_offsite_schedule        = ?,
		  flash_offsite_schedule      = ?,
		  config_offsite_schedule     = ?,
		  files_offsite_schedule      = ?,
		  containers_schedule = ?,
		  vms_schedule        = ?,
		  flash_schedule      = ?,
		  config_schedule     = ?,
		  files_schedule      = ?,
		  default_language    = ?,
		  auth_password_hash  = ?,
		  retention_keep_last    = ?,
		  retention_keep_daily   = ?,
		  retention_keep_weekly  = ?,
		  retention_keep_monthly = ?,
		  offsite_retention_keep_last    = ?,
		  offsite_retention_keep_daily   = ?,
		  offsite_retention_keep_weekly  = ?,
		  offsite_retention_keep_monthly = ?,
		  offsite_limit_upload   = ?,
		  offsite_limit_download = ?,
		  rclone_conf            = ?,
		  notify_conf            = ?,
		  cloud_conf             = ?,
		  registry_auths         = ?,
		  metrics_enabled        = ?,
		  metrics_token          = ?,
		  widget_token           = ?,
		  drills_enabled         = ?,
		  drills_schedule        = ?,
		  drills_subset_pct      = ?,
		  offsite_drills_enabled = ?,
		  recovery_kit_ack       = ?,
		  containers_offsite_immutable = ?,
		  vms_offsite_immutable        = ?,
		  flash_offsite_immutable      = ?,
		  config_offsite_immutable     = ?,
		  files_offsite_immutable      = ?,
		  offsite_growth_budget_gb     = ?,
		  tamper_test_schedule         = ?,
		  dr_drill_target              = ?,
		  dr_drill_target_vm           = ?,
		  flash_zip_export_enabled     = ?,
		  flash_zip_export_path        = ?,
		  flash_zip_export_keep        = ?,
		  prune_image_after_update     = ?,
		  session_epoch                = ?,
		  restic_cache_max_mb          = ?,
		  digest_enabled               = ?,
		  digest_schedule              = ?,
		  catch_up_missed              = ?,
		  watchdog_enabled             = ?,
		  export_encrypt_enabled       = ?,
		  export_age_recipients        = ?,
		  receiver_enabled             = ?,
		  restart_health_wait          = ?,
		  restart_health_timeout_sec   = ?,
		  reconcile_unraid_update_status = ?,
		  per_item_schedules           = ?,
		  cloud_cred_sets              = ?,
		  fleet_enabled                = ?,
		  instance_name                = ?,
		  fleet_token                  = ?,
		  everything_schedule          = ?,
		  everything_pre_hook          = ?,
		  everything_post_hook         = ?,
		  backup_cores                 = ?
		WHERE id = 1`,
		boolInt(s.EncryptionEnabled),
		boolInt(s.ContainersEnabled),
		boolInt(s.VMsEnabled),
		boolInt(s.FlashEnabled),
		boolInt(s.ConfigEnabled),
		boolInt(s.FilesEnabled),
		s.ContainersPath, s.VMsPath, s.FlashPath, s.ConfigPath, s.FilesPath, s.RestoreFolder,
		s.ContainersOffsite, s.VMsOffsite, s.FlashOffsite, s.ConfigOffsite, s.FilesOffsite,
		s.ContainersOffsiteSchedule, s.VMsOffsiteSchedule, s.FlashOffsiteSchedule, s.ConfigOffsiteSchedule, s.FilesOffsiteSchedule,
		s.ContainersSchedule, s.VMsSchedule, s.FlashSchedule, s.ConfigSchedule, s.FilesSchedule,
		s.DefaultLanguage, s.AuthPasswordHash,
		s.RetentionKeepLast, s.RetentionKeepDaily, s.RetentionKeepWeekly, s.RetentionKeepMonthly,
		s.OffsiteRetentionKeepLast, s.OffsiteRetentionKeepDaily, s.OffsiteRetentionKeepWeekly, s.OffsiteRetentionKeepMonthly,
		s.OffsiteLimitUpload, s.OffsiteLimitDownload,
		s.RcloneConf, s.NotifyConf, s.CloudConf, s.RegistryAuths,
		boolInt(s.MetricsEnabled), s.MetricsToken, s.WidgetToken,
		boolInt(s.DrillsEnabled), s.DrillsSchedule, s.DrillsSubsetPct, boolInt(s.OffsiteDrillsEnabled),
		boolInt(s.RecoveryKitAck),
		boolInt(s.ContainersOffsiteImmutable), boolInt(s.VMsOffsiteImmutable), boolInt(s.FlashOffsiteImmutable), boolInt(s.ConfigOffsiteImmutable), boolInt(s.FilesOffsiteImmutable),
		s.OffsiteGrowthBudgetGB, s.TamperTestSchedule, s.DRDrillTarget, s.DRDrillTargetVM,
		boolInt(s.FlashZipExportEnabled), s.FlashZipExportPath, s.FlashZipExportKeep,
		boolInt(s.PruneImageAfterUpdate), s.SessionEpoch, s.ResticCacheMaxMB,
		boolInt(s.DigestEnabled), s.DigestSchedule,
		boolInt(s.CatchUpMissed), boolInt(s.WatchdogEnabled),
		boolInt(s.ExportEncryptEnabled), s.ExportAgeRecipients,
		boolInt(s.ReceiverEnabled),
		boolInt(s.RestartHealthWait), s.RestartHealthTimeoutSec,
		boolInt(s.ReconcileUnraidUpdateStatus),
		boolInt(s.PerItemSchedules),
		s.CloudCredSets,
		boolInt(s.FleetEnabled),
		s.InstanceName,
		s.FleetToken,
		s.EverythingSchedule,
		s.EverythingPreHook,
		s.EverythingPostHook,
		s.BackupCores,
	)
	if err != nil {
		return fmt.Errorf("UpdateSettings: %w", err)
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
