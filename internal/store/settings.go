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
	ContainersPath    string
	VMsPath           string
	FlashPath         string
	// Optional off-site repo per domain. When set, a successful local backup is
	// replicated there with `restic copy` (the local repo stays primary). Empty
	// means no off-site copy for that domain.
	ContainersOffsite string
	VMsOffsite        string
	FlashOffsite      string
	// Optional off-site replication schedule per domain (same cadence grammar as
	// the backup schedules). Empty = replicate after every local backup; set =
	// replicate ONLY on this cadence (decoupled from the backup schedule).
	ContainersOffsiteSchedule string
	VMsOffsiteSchedule        string
	FlashOffsiteSchedule      string
	ContainersSchedule        string
	VMsSchedule               string
	FlashSchedule             string
	DefaultLanguage           string
	// AuthPasswordHash is the HMAC-SHA256 password hash set by the admin.
	// An empty string means authentication is disabled (the default).
	AuthPasswordHash string
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
	// RcloneConf is the rclone configuration (INI) for off-site repos, stored
	// AES-256-GCM-encrypted at rest. Empty means no rclone backends configured.
	RcloneConf string
	// NotifyConf is the notification config (webhook / Matrix / Healthchecks) as
	// an AES-256-GCM-encrypted JSON blob (base64). Empty means notifications off.
	NotifyConf string
	// CloudConf is the cloud-backend credentials (S3 keys, restic-REST auth) for
	// off-site repos, an AES-256-GCM-encrypted JSON blob (base64). Empty = none.
	CloudConf string
	// MetricsEnabled exposes the Prometheus-format /metrics endpoint when true.
	// Default false (opt-in): when off, /metrics returns 404 and is not served.
	MetricsEnabled bool
	// MetricsToken is an optional bearer token for /metrics. When set, a scrape
	// must send `Authorization: Bearer <token>`; empty means open (LAN trust
	// model, like /api/health). The endpoint exposes only non-sensitive metrics.
	MetricsToken string
	// DrillsEnabled turns on scheduled restore-verification drills. Off by default
	// (drills read back real pack data, so they cost I/O), so existing setups are
	// unchanged until the user opts in.
	DrillsEnabled bool
	// DrillsSchedule is the cadence for scheduled drills (same grammar as the backup
	// schedules). 'off' (the default) = no scheduled drills.
	DrillsSchedule string
	// DrillsSubsetPct is the percentage of pack data each drill reads back and
	// re-verifies (`restic check --read-data-subset`). Clamped 1..100; defaults to 5.
	DrillsSubsetPct int
}

// GetSettings returns the current app settings.
func (r *Repo) GetSettings() (Settings, error) {
	row := r.db.QueryRow(`
		SELECT encryption_enabled, containers_enabled, vms_enabled, flash_enabled,
		       containers_path, vms_path, flash_path,
		       containers_offsite, vms_offsite, flash_offsite,
		       containers_offsite_schedule, vms_offsite_schedule, flash_offsite_schedule,
		       containers_schedule, vms_schedule, flash_schedule,
		       default_language, auth_password_hash,
		       retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly,
		       offsite_retention_keep_last, offsite_retention_keep_daily, offsite_retention_keep_weekly, offsite_retention_keep_monthly,
		       offsite_limit_upload, offsite_limit_download,
		       rclone_conf, notify_conf, cloud_conf,
		       metrics_enabled, metrics_token,
		       drills_enabled, drills_schedule, drills_subset_pct
		FROM settings WHERE id = 1`)

	var s Settings
	var encEnabled, contEnabled, vmsEnabled, flashEnabled, metricsEnabled, drillsEnabled int
	err := row.Scan(
		&encEnabled, &contEnabled, &vmsEnabled, &flashEnabled,
		&s.ContainersPath, &s.VMsPath, &s.FlashPath,
		&s.ContainersOffsite, &s.VMsOffsite, &s.FlashOffsite,
		&s.ContainersOffsiteSchedule, &s.VMsOffsiteSchedule, &s.FlashOffsiteSchedule,
		&s.ContainersSchedule, &s.VMsSchedule, &s.FlashSchedule,
		&s.DefaultLanguage, &s.AuthPasswordHash,
		&s.RetentionKeepLast, &s.RetentionKeepDaily, &s.RetentionKeepWeekly, &s.RetentionKeepMonthly,
		&s.OffsiteRetentionKeepLast, &s.OffsiteRetentionKeepDaily, &s.OffsiteRetentionKeepWeekly, &s.OffsiteRetentionKeepMonthly,
		&s.OffsiteLimitUpload, &s.OffsiteLimitDownload,
		&s.RcloneConf, &s.NotifyConf, &s.CloudConf,
		&metricsEnabled, &s.MetricsToken,
		&drillsEnabled, &s.DrillsSchedule, &s.DrillsSubsetPct,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Settings{}, fmt.Errorf("settings row missing — run Migrate first")
	}
	if err != nil {
		return Settings{}, fmt.Errorf("GetSettings: %w", err)
	}
	s.EncryptionEnabled = encEnabled != 0
	s.ContainersEnabled = contEnabled != 0
	s.VMsEnabled = vmsEnabled != 0
	s.FlashEnabled = flashEnabled != 0
	s.MetricsEnabled = metricsEnabled != 0
	s.DrillsEnabled = drillsEnabled != 0
	return s, nil
}

// UpdateSettings persists s back to the single settings row.
func (r *Repo) UpdateSettings(s Settings) error {
	_, err := r.db.Exec(`
		UPDATE settings SET
		  encryption_enabled  = ?,
		  containers_enabled  = ?,
		  vms_enabled         = ?,
		  flash_enabled       = ?,
		  containers_path     = ?,
		  vms_path            = ?,
		  flash_path          = ?,
		  containers_offsite  = ?,
		  vms_offsite         = ?,
		  flash_offsite       = ?,
		  containers_offsite_schedule = ?,
		  vms_offsite_schedule        = ?,
		  flash_offsite_schedule      = ?,
		  containers_schedule = ?,
		  vms_schedule        = ?,
		  flash_schedule      = ?,
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
		  metrics_enabled        = ?,
		  metrics_token          = ?,
		  drills_enabled         = ?,
		  drills_schedule        = ?,
		  drills_subset_pct      = ?
		WHERE id = 1`,
		boolInt(s.EncryptionEnabled),
		boolInt(s.ContainersEnabled),
		boolInt(s.VMsEnabled),
		boolInt(s.FlashEnabled),
		s.ContainersPath, s.VMsPath, s.FlashPath,
		s.ContainersOffsite, s.VMsOffsite, s.FlashOffsite,
		s.ContainersOffsiteSchedule, s.VMsOffsiteSchedule, s.FlashOffsiteSchedule,
		s.ContainersSchedule, s.VMsSchedule, s.FlashSchedule,
		s.DefaultLanguage, s.AuthPasswordHash,
		s.RetentionKeepLast, s.RetentionKeepDaily, s.RetentionKeepWeekly, s.RetentionKeepMonthly,
		s.OffsiteRetentionKeepLast, s.OffsiteRetentionKeepDaily, s.OffsiteRetentionKeepWeekly, s.OffsiteRetentionKeepMonthly,
		s.OffsiteLimitUpload, s.OffsiteLimitDownload,
		s.RcloneConf, s.NotifyConf, s.CloudConf,
		boolInt(s.MetricsEnabled), s.MetricsToken,
		boolInt(s.DrillsEnabled), s.DrillsSchedule, s.DrillsSubsetPct,
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
