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
	// RcloneConf is the rclone configuration (INI) for off-site repos, stored
	// AES-256-GCM-encrypted at rest. Empty means no rclone backends configured.
	RcloneConf string
	// NotifyConf is the notification config (webhook / Matrix / Healthchecks) as
	// an AES-256-GCM-encrypted JSON blob (base64). Empty means notifications off.
	NotifyConf string
	// CloudConf is the cloud-backend credentials (S3 keys, restic-REST auth) for
	// off-site repos, an AES-256-GCM-encrypted JSON blob (base64). Empty = none.
	CloudConf string
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
		       rclone_conf, notify_conf, cloud_conf
		FROM settings WHERE id = 1`)

	var s Settings
	var encEnabled, contEnabled, vmsEnabled, flashEnabled int
	err := row.Scan(
		&encEnabled, &contEnabled, &vmsEnabled, &flashEnabled,
		&s.ContainersPath, &s.VMsPath, &s.FlashPath,
		&s.ContainersOffsite, &s.VMsOffsite, &s.FlashOffsite,
		&s.ContainersOffsiteSchedule, &s.VMsOffsiteSchedule, &s.FlashOffsiteSchedule,
		&s.ContainersSchedule, &s.VMsSchedule, &s.FlashSchedule,
		&s.DefaultLanguage, &s.AuthPasswordHash,
		&s.RetentionKeepLast, &s.RetentionKeepDaily, &s.RetentionKeepWeekly, &s.RetentionKeepMonthly,
		&s.RcloneConf, &s.NotifyConf, &s.CloudConf,
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
		  rclone_conf            = ?,
		  notify_conf            = ?,
		  cloud_conf             = ?
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
		s.RcloneConf, s.NotifyConf, s.CloudConf,
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
