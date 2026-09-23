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
	ZFSEnabled        bool
	ContainersPath    string
	VMsPath           string
	FlashPath         string
	ConfigPath        string
	FilesPath         string
	ZFSPath           string
	// RestoreFolder pre-fills the restore-to-folder picker. Like the backup
	// paths it is relative to the host mount.
	RestoreFolder string
	// Off-site repo per domain. When set, a successful local backup is
	// replicated there with `restic copy`; the local repo stays primary.
	ContainersOffsite string
	VMsOffsite        string
	FlashOffsite      string
	ConfigOffsite     string
	FilesOffsite      string
	ZFSOffsite        string
	// Off-site replication schedule per domain, in the backup-schedule grammar.
	// Empty replicates after every local backup; otherwise replication follows
	// this cadence alone.
	ContainersOffsiteSchedule string
	VMsOffsiteSchedule        string
	FlashOffsiteSchedule      string
	ConfigOffsiteSchedule     string
	FilesOffsiteSchedule      string
	ZFSOffsiteSchedule        string
	ContainersSchedule        string
	VMsSchedule               string
	FlashSchedule             string
	ConfigSchedule            string
	FilesSchedule             string
	ZFSSchedule               string
	// Flash ZIP export: after a successful flash backup the snapshot is also
	// written as a plain .zip to FlashZipExportPath for syncing off the server.
	// FlashZipExportKeep is how many timestamped zips to keep; 0 keeps a single
	// flash-latest.zip.
	FlashZipExportEnabled bool
	FlashZipExportPath    string
	FlashZipExportKeep    int
	DefaultLanguage       string
	// AuthPasswordHash is the login password as an Argon2id hash
	// (secret.HashPassword). secret.VerifyPassword also accepts a bare HMAC hex
	// string, which the login handler rehashes on the next sign-in. Empty
	// disables authentication.
	AuthPasswordHash string
	// TOTPSecret is the authenticator-app secret, encrypted with the APP_KEY and
	// hex-encoded. The server needs the plain secret to compute the expected
	// code, so it is encrypted rather than hashed.
	TOTPSecret string
	// TOTPEnabled is true only after the operator has proved the app works by
	// entering one correct code. A stored secret with this still false is an
	// abandoned enrolment and is ignored by the login.
	TOTPEnabled bool
	// TOTPRecovery is a JSON array of hashed single-use recovery codes. A spent
	// code is removed from the array, so its length is how many remain.
	TOTPRecovery string
	// SessionEpoch is mixed into every session token's HMAC. Rotating it
	// (POST /api/logout-all) is the only way to revoke the otherwise stateless
	// tokens. Empty is a valid epoch.
	SessionEpoch string
	// Retention policy, applied with `restic forget --prune` after each
	// successful backup. All zero keeps every snapshot.
	RetentionKeepLast    int
	RetentionKeepDaily   int
	RetentionKeepWeekly  int
	RetentionKeepMonthly int
	// Off-site retention policy, separate from the local one so the off-site
	// repo can serve as a longer archive. All zero never prunes the off-site
	// repo.
	OffsiteRetentionKeepLast    int
	OffsiteRetentionKeepDaily   int
	OffsiteRetentionKeepWeekly  int
	OffsiteRetentionKeepMonthly int
	// Bandwidth caps in KiB/s for off-site replication and remote backups,
	// passed to restic as --limit-upload and --limit-download. 0 is unlimited.
	OffsiteLimitUpload   int
	OffsiteLimitDownload int
	// BackupCores caps the CPU threads each restic process may use, passed to
	// it as GOMAXPROCS. 0 uses every core, restic's own default.
	BackupCores int
	// DisplayPrefs holds the look of the interface as a JSON object, so it
	// follows the user across browsers. Only the frontend reads inside it, which
	// saves a migration per new option. When empty, the first load seeds it
	// from the browser.
	DisplayPrefs string
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
	// post-backup update pull, an AES-256-GCM-encrypted JSON array (base64) of
	// {host, username, token} entries. Empty means anonymous pulls.
	RegistryAuths string
	// MetricsEnabled serves the Prometheus /metrics endpoint. When off,
	// /metrics returns 404.
	MetricsEnabled bool
	// MetricsToken is an optional bearer token for /metrics. When set, a scrape
	// must send `Authorization: Bearer <token>`; empty means open (LAN trust
	// model, like /api/health). The endpoint exposes only non-sensitive metrics.
	MetricsToken string
	// WidgetToken authorizes the session-free dashboard widget (GET /widget and
	// GET /api/widget/data, via ?token= or X-Widget-Token). Empty disables the
	// widget and both endpoints answer 403; unlike MetricsToken there is no open
	// mode.
	WidgetToken string
	// InstanceName is this instance's display name, reported in
	// GET /api/fleet/status so a peer's Fleet page can label this box. When it
	// is empty the Fleet page shows the URL it polled.
	InstanceName string
	// FleetToken authorizes the session-free GET /api/fleet/status (via ?token=
	// or X-Fleet-Token) that other instances' Fleet views poll. Empty disables
	// the endpoint with 403, as for WidgetToken.
	FleetToken string
	// DrillsEnabled turns on scheduled restore-verification drills. Off by
	// default, because a drill reads back real pack data and costs I/O.
	DrillsEnabled bool
	// OffsiteDrillsEnabled gates the scheduled off-site DR drill. When off, the
	// scheduled local integrity check still runs and the off-site check can be
	// started by hand. Default on.
	OffsiteDrillsEnabled bool
	// DrillsSchedule is the cadence for scheduled drills, in the backup-schedule
	// grammar. Default 'off'.
	DrillsSchedule string
	// DrillsSubsetPct is the percentage of pack data each drill reads back and
	// re-verifies (`restic check --read-data-subset`). Clamped 1..100; defaults to 5.
	DrillsSubsetPct int
	// RecoveryKitAck records that the user has downloaded and stored the
	// encryption-key recovery kit. Until then the dashboard shows a reminder
	// while encryption is on.
	RecoveryKitAck bool
	// Per-domain flag that the off-site repo is append-only. The far side (for
	// example rest-server --append-only) enforces it; with the flag set BombVault
	// skips its own off-site retention prune and refuses off-site deletes.
	ContainersOffsiteImmutable bool
	VMsOffsiteImmutable        bool
	FlashOffsiteImmutable      bool
	ConfigOffsiteImmutable     bool
	FilesOffsiteImmutable      bool
	ZFSOffsiteImmutable        bool
	// OffsiteGrowthBudgetGB is the size at which an append-only off-site repo,
	// which can only grow, triggers a notification. It warns and blocks nothing.
	// 0 turns the alarm off.
	OffsiteGrowthBudgetGB int
	// TamperTestSchedule is the cadence for the scheduled off-site tamper test,
	// in the backup-schedule grammar. Defaults to "weekly Sun 04:30".
	TamperTestSchedule string
	// DRDrillTarget is the container the real-restore DR drill restores. Empty
	// picks the most recently backed-up container.
	DRDrillTarget string
	// DRDrillTargetVM is the VM the real-restore DR drill restores. Empty picks
	// the most recently backed-up VM.
	DRDrillTargetVM string
	// PruneImageAfterUpdate removes the superseded image after a post-backup
	// container update. Off by default, because the old image makes a rollback
	// cheap. Removal is best-effort and not forced, so a shared base image stays.
	PruneImageAfterUpdate bool
	// ResticCacheMaxMB caps restic's cache under /config, which survives
	// restarts and would otherwise grow without bound. Above it, scheduled runs
	// evict the least recently used per-repo cache directories. 0 means no
	// limit. Defaults to 4096.
	ResticCacheMaxMB int
	// DigestEnabled turns on the scheduled digest: one summary message (run
	// counts per kind, backup bytes, off-site currency, top failures) through
	// the usual notification channels. Off by default.
	DigestEnabled bool
	// DigestSchedule is the digest cadence, in the backup-schedule grammar.
	// Defaults to "weekly Mon 08:00".
	DigestSchedule string
	// CatchUpMissed runs a scheduled backup that was missed while the app was
	// down shortly after the next start, like anacron. Default on.
	CatchUpMissed bool
	// WatchdogEnabled turns on the daily overdue-backup watchdog, which notifies
	// once per episode when a domain is overdue by the dashboard's RPO rule.
	// Default on.
	WatchdogEnabled bool
	// ExportEncryptEnabled encrypts the plain exports (tar.gz, xml, zip) to
	// ExportAgeRecipients with age. Off by default. The restic repository is
	// encrypted regardless.
	ExportEncryptEnabled bool
	// ExportAgeRecipients is the whitespace-separated list of age recipients
	// (age1... or SSH public keys), stored as is because they are public. With
	// ExportEncryptEnabled on and no valid recipient, every export fails instead
	// of writing plaintext.
	ExportAgeRecipients string
	// ReceiverEnabled shows the read-only receiver dashboard, for a box that
	// receives immutable off-site copies and monitors them. Off by default.
	ReceiverEnabled bool
	// FleetEnabled shows the read-only Fleet view, which polls peer BombVault
	// instances for their protection status. Off by default.
	FleetEnabled bool
	// PullEnabled allows fetching snapshots out of another instance's repository
	// into this one. Off by default: unlike the receiver and fleet views, it
	// writes to this box's own repository.
	PullEnabled bool
	// DBDumpsEnabled is the global switch for the automatic database dumps, the
	// one place to stop the feature for every container at once. Default on.
	DBDumpsEnabled bool
	// RestartHealthWait makes the ordered restart after a backup wait until each
	// dependency is healthy, or running plus a short grace when it has no
	// healthcheck, before starting what depends on it. Default on.
	RestartHealthWait bool
	// RestartHealthTimeoutSec caps that wait per container. When it runs out
	// the restart logs a warning and starts the dependents anyway, so a backup
	// cannot hang on it. Default 120.
	RestartHealthTimeoutSec int
	// ReconcileUnraidUpdateStatus has Unraid recheck its cached "update
	// available" status after the post-backup update step recreates a
	// container; otherwise the Docker tab keeps a stale banner. The recheck runs
	// Unraid's own script over the host SSH link and is best-effort.
	ReconcileUnraidUpdateStatus bool
	// PerItemSchedules enables per-container and per-VM schedule overrides. When
	// on, an included item with a ScheduleCadence is backed up on that cadence
	// and the rest follow their domain schedule. Off by default.
	PerItemSchedules bool
	// CloudCredSets holds additional named S3 and restic-REST credential sets as
	// an AES-256-GCM-encrypted JSON array (base64). An off-site target picks one
	// with its CredsRef; an empty CredsRef uses CloudConf.
	CloudCredSets string
	// EverythingSchedule is the cadence of the "Backup Everything" pass, which
	// runs every domain in sequence alongside the domain schedules. Default
	// 'off'.
	EverythingSchedule string
	// EverythingPreHook and EverythingPostHook are shell commands run in
	// BombVault's own container before and after the "Backup Everything" pass.
	// A failing pre-hook is logged and the pass goes on; the post-hook runs once
	// whatever the outcome, so it can feed a dead man's switch.
	EverythingPreHook  string
	EverythingPostHook string
}

// settingsQuerier and settingsExecer are satisfied by both *sql.DB and *sql.Tx,
// so MutateSettings runs the same read and write as the plain methods.
type settingsQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

type settingsExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// GetSettings returns the current app settings. To change a field, use
// MutateSettings rather than pairing this with UpdateSettings.
func (r *Repo) GetSettings() (Settings, error) {
	return getSettings(r.db)
}

func getSettings(q settingsQuerier) (Settings, error) {
	row := q.QueryRow(`
		SELECT encryption_enabled, containers_enabled, vms_enabled, flash_enabled, config_enabled, files_enabled, zfs_enabled,
		       containers_path, vms_path, flash_path, config_path, files_path, zfs_path, restore_folder,
		       containers_offsite, vms_offsite, flash_offsite, config_offsite, files_offsite, zfs_offsite,
		       containers_offsite_schedule, vms_offsite_schedule, flash_offsite_schedule, config_offsite_schedule, files_offsite_schedule, zfs_offsite_schedule,
		       containers_schedule, vms_schedule, flash_schedule, config_schedule, files_schedule, zfs_schedule,
		       default_language, auth_password_hash,
		       retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly,
		       offsite_retention_keep_last, offsite_retention_keep_daily, offsite_retention_keep_weekly, offsite_retention_keep_monthly,
		       offsite_limit_upload, offsite_limit_download,
		       rclone_conf, notify_conf, cloud_conf, registry_auths,
		       metrics_enabled, metrics_token, widget_token,
		       drills_enabled, drills_schedule, drills_subset_pct, offsite_drills_enabled,
		       recovery_kit_ack,
		       containers_offsite_immutable, vms_offsite_immutable, flash_offsite_immutable, config_offsite_immutable, files_offsite_immutable, zfs_offsite_immutable,
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
		       fleet_enabled, pull_enabled, db_dumps_enabled, instance_name, fleet_token,
		       everything_schedule, everything_pre_hook, everything_post_hook,
		       backup_cores, display_prefs,
		       totp_secret, totp_enabled, totp_recovery
		FROM settings WHERE id = 1`)

	var s Settings
	var encEnabled, contEnabled, vmsEnabled, flashEnabled, configEnabled, filesEnabled, zfsEnabled, metricsEnabled, drillsEnabled, offsiteDrillsEnabled, recoveryKitAck int
	var contImmutable, vmsImmutable, flashImmutable, configImmutable, filesImmutable, zfsImmutable int
	var flashZipExportEnabled, pruneImageAfterUpdate, digestEnabled int
	var catchUpMissed, watchdogEnabled, exportEncryptEnabled, receiverEnabled int
	var restartHealthWait, reconcileUnraidUpdateStatus, perItemSchedules int
	var fleetEnabled, pullEnabled, dbDumpsEnabled, totpEnabled int
	err := row.Scan(
		&encEnabled, &contEnabled, &vmsEnabled, &flashEnabled, &configEnabled, &filesEnabled, &zfsEnabled,
		&s.ContainersPath, &s.VMsPath, &s.FlashPath, &s.ConfigPath, &s.FilesPath, &s.ZFSPath, &s.RestoreFolder,
		&s.ContainersOffsite, &s.VMsOffsite, &s.FlashOffsite, &s.ConfigOffsite, &s.FilesOffsite, &s.ZFSOffsite,
		&s.ContainersOffsiteSchedule, &s.VMsOffsiteSchedule, &s.FlashOffsiteSchedule, &s.ConfigOffsiteSchedule, &s.FilesOffsiteSchedule, &s.ZFSOffsiteSchedule,
		&s.ContainersSchedule, &s.VMsSchedule, &s.FlashSchedule, &s.ConfigSchedule, &s.FilesSchedule, &s.ZFSSchedule,
		&s.DefaultLanguage, &s.AuthPasswordHash,
		&s.RetentionKeepLast, &s.RetentionKeepDaily, &s.RetentionKeepWeekly, &s.RetentionKeepMonthly,
		&s.OffsiteRetentionKeepLast, &s.OffsiteRetentionKeepDaily, &s.OffsiteRetentionKeepWeekly, &s.OffsiteRetentionKeepMonthly,
		&s.OffsiteLimitUpload, &s.OffsiteLimitDownload,
		&s.RcloneConf, &s.NotifyConf, &s.CloudConf, &s.RegistryAuths,
		&metricsEnabled, &s.MetricsToken, &s.WidgetToken,
		&drillsEnabled, &s.DrillsSchedule, &s.DrillsSubsetPct, &offsiteDrillsEnabled,
		&recoveryKitAck,
		&contImmutable, &vmsImmutable, &flashImmutable, &configImmutable, &filesImmutable, &zfsImmutable,
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
		&fleetEnabled, &pullEnabled, &dbDumpsEnabled, &s.InstanceName, &s.FleetToken,
		&s.EverythingSchedule, &s.EverythingPreHook, &s.EverythingPostHook,
		&s.BackupCores, &s.DisplayPrefs,
		&s.TOTPSecret, &totpEnabled, &s.TOTPRecovery,
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
	s.ZFSEnabled = zfsEnabled != 0
	s.MetricsEnabled = metricsEnabled != 0
	s.DrillsEnabled = drillsEnabled != 0
	s.OffsiteDrillsEnabled = offsiteDrillsEnabled != 0
	s.RecoveryKitAck = recoveryKitAck != 0
	s.TOTPEnabled = totpEnabled != 0
	s.ContainersOffsiteImmutable = contImmutable != 0
	s.VMsOffsiteImmutable = vmsImmutable != 0
	s.FlashOffsiteImmutable = flashImmutable != 0
	s.ConfigOffsiteImmutable = configImmutable != 0
	s.FilesOffsiteImmutable = filesImmutable != 0
	s.ZFSOffsiteImmutable = zfsImmutable != 0
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
	s.PullEnabled = pullEnabled != 0
	s.DBDumpsEnabled = dbDumpsEnabled != 0
	return s, nil
}

// UpdateSettings writes every column of the settings row from s. Use it only
// where s is the whole intended row: a read-modify-write through it reverts
// whatever another writer stored in between, which is what MutateSettings is
// for. settings_writers_test.go keeps production code off this method.
func (r *Repo) UpdateSettings(s Settings) error {
	r.settingsMu.Lock()
	defer r.settingsMu.Unlock()
	return updateSettings(r.db, s)
}

// MutateSettings applies fn to the current settings row and writes the result
// back, all inside one serialized transaction, so fn sees the latest row and
// changes only what it sets. It returns the settings as stored. If fn changes
// nothing, no write is issued.
//
// fn must not touch the Repo or the database: the transaction holds the only
// pooled connection (Open sets MaxOpenConns(1)), so a nested call deadlocks.
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
		  zfs_enabled         = ?,
		  containers_path     = ?,
		  vms_path            = ?,
		  flash_path          = ?,
		  config_path         = ?,
		  files_path          = ?,
		  zfs_path            = ?,
		  restore_folder      = ?,
		  containers_offsite  = ?,
		  vms_offsite         = ?,
		  flash_offsite       = ?,
		  config_offsite      = ?,
		  files_offsite       = ?,
		  zfs_offsite         = ?,
		  containers_offsite_schedule = ?,
		  vms_offsite_schedule        = ?,
		  flash_offsite_schedule      = ?,
		  config_offsite_schedule     = ?,
		  files_offsite_schedule      = ?,
		  zfs_offsite_schedule        = ?,
		  containers_schedule = ?,
		  vms_schedule        = ?,
		  flash_schedule      = ?,
		  config_schedule     = ?,
		  files_schedule      = ?,
		  zfs_schedule        = ?,
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
		  zfs_offsite_immutable        = ?,
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
		  pull_enabled                 = ?,
		  db_dumps_enabled             = ?,
		  instance_name                = ?,
		  fleet_token                  = ?,
		  everything_schedule          = ?,
		  everything_pre_hook          = ?,
		  everything_post_hook         = ?,
		  backup_cores                 = ?,
		  display_prefs                = ?,
		  totp_secret                  = ?,
		  totp_enabled                 = ?,
		  totp_recovery                = ?
		WHERE id = 1`,
		boolInt(s.EncryptionEnabled),
		boolInt(s.ContainersEnabled),
		boolInt(s.VMsEnabled),
		boolInt(s.FlashEnabled),
		boolInt(s.ConfigEnabled),
		boolInt(s.FilesEnabled),
		boolInt(s.ZFSEnabled),
		s.ContainersPath, s.VMsPath, s.FlashPath, s.ConfigPath, s.FilesPath, s.ZFSPath, s.RestoreFolder,
		s.ContainersOffsite, s.VMsOffsite, s.FlashOffsite, s.ConfigOffsite, s.FilesOffsite, s.ZFSOffsite,
		s.ContainersOffsiteSchedule, s.VMsOffsiteSchedule, s.FlashOffsiteSchedule, s.ConfigOffsiteSchedule, s.FilesOffsiteSchedule, s.ZFSOffsiteSchedule,
		s.ContainersSchedule, s.VMsSchedule, s.FlashSchedule, s.ConfigSchedule, s.FilesSchedule, s.ZFSSchedule,
		s.DefaultLanguage, s.AuthPasswordHash,
		s.RetentionKeepLast, s.RetentionKeepDaily, s.RetentionKeepWeekly, s.RetentionKeepMonthly,
		s.OffsiteRetentionKeepLast, s.OffsiteRetentionKeepDaily, s.OffsiteRetentionKeepWeekly, s.OffsiteRetentionKeepMonthly,
		s.OffsiteLimitUpload, s.OffsiteLimitDownload,
		s.RcloneConf, s.NotifyConf, s.CloudConf, s.RegistryAuths,
		boolInt(s.MetricsEnabled), s.MetricsToken, s.WidgetToken,
		boolInt(s.DrillsEnabled), s.DrillsSchedule, s.DrillsSubsetPct, boolInt(s.OffsiteDrillsEnabled),
		boolInt(s.RecoveryKitAck),
		boolInt(s.ContainersOffsiteImmutable), boolInt(s.VMsOffsiteImmutable), boolInt(s.FlashOffsiteImmutable), boolInt(s.ConfigOffsiteImmutable), boolInt(s.FilesOffsiteImmutable), boolInt(s.ZFSOffsiteImmutable),
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
		boolInt(s.PullEnabled),
		boolInt(s.DBDumpsEnabled),
		s.InstanceName,
		s.FleetToken,
		s.EverythingSchedule,
		s.EverythingPreHook,
		s.EverythingPostHook,
		s.BackupCores,
		s.DisplayPrefs,
		s.TOTPSecret,
		boolInt(s.TOTPEnabled),
		s.TOTPRecovery,
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
