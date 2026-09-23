package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestSettingsRoundtrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	s, err := r.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	// Check defaults from migration.
	if !s.EncryptionEnabled {
		t.Fatal("default encryption_enabled should be true")
	}
	if s.ContainersPath != "user/bombvault/container" {
		t.Fatalf("default containers_path wrong: %q", s.ContainersPath)
	}

	if s.ContainersOffsiteImmutable || s.VMsOffsiteImmutable || s.FlashOffsiteImmutable {
		t.Fatal("default *_offsite_immutable must be false")
	}
	if s.OffsiteGrowthBudgetGB != 0 {
		t.Fatalf("default offsite_growth_budget_gb must be 0, got %d", s.OffsiteGrowthBudgetGB)
	}
	if s.TamperTestSchedule != "weekly Sun 04:30" {
		t.Fatalf("default tamper_test_schedule wrong: %q", s.TamperTestSchedule)
	}
	if s.DRDrillTarget != "" {
		t.Fatalf("default dr_drill_target must be empty, got %q", s.DRDrillTarget)
	}
	if !s.OffsiteDrillsEnabled {
		t.Fatal("default offsite_drills_enabled should be true")
	}

	s.ContainersPath = "custom/path"
	s.ContainersSchedule = "daily 03:00"
	s.ContainersOffsiteImmutable = true
	s.VMsOffsiteImmutable = true
	s.FlashOffsiteImmutable = true
	s.OffsiteGrowthBudgetGB = 500
	s.TamperTestSchedule = "daily 05:15"
	s.DRDrillTarget = "plex"
	s.OffsiteDrillsEnabled = false
	if err := r.UpdateSettings(s); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	s2, err := r.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings after update: %v", err)
	}
	if s2.ContainersPath != "custom/path" {
		t.Fatalf("containers_path not updated: %q", s2.ContainersPath)
	}
	if s2.ContainersSchedule != "daily 03:00" {
		t.Fatalf("containers_schedule not updated: %q", s2.ContainersSchedule)
	}
	if !s2.ContainersOffsiteImmutable || !s2.VMsOffsiteImmutable || !s2.FlashOffsiteImmutable {
		t.Fatalf("*_offsite_immutable not persisted: %+v", s2)
	}
	if s2.OffsiteGrowthBudgetGB != 500 {
		t.Fatalf("offsite_growth_budget_gb not persisted: %d", s2.OffsiteGrowthBudgetGB)
	}
	if s2.TamperTestSchedule != "daily 05:15" {
		t.Fatalf("tamper_test_schedule not persisted: %q", s2.TamperTestSchedule)
	}
	if s2.DRDrillTarget != "plex" {
		t.Fatalf("dr_drill_target not persisted: %q", s2.DRDrillTarget)
	}
	if s2.OffsiteDrillsEnabled {
		t.Fatalf("offsite_drills_enabled not persisted as false: %+v", s2)
	}
}

func TestSettingsConfigFieldsRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	s, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	// Defaults from migration: disabled, canonical path, schedule off.
	if s.ConfigEnabled {
		t.Fatal("default config_enabled must be false")
	}
	if s.ConfigPath != "user/bombvault/config" {
		t.Fatalf("default config_path wrong: %q", s.ConfigPath)
	}
	if s.ConfigSchedule != "off" {
		t.Fatalf("default config_schedule wrong: %q", s.ConfigSchedule)
	}
	if s.ConfigOffsite != "" || s.ConfigOffsiteSchedule != "" || s.ConfigOffsiteImmutable {
		t.Fatalf("default config off-site fields must be empty/false: %+v", s)
	}

	s.ConfigEnabled = true
	s.ConfigPath = "user/bombvault/config"
	s.ConfigSchedule = "daily 03:30"
	s.ConfigOffsite = "rclone:remote:bombvault-config"
	s.ConfigOffsiteSchedule = "weekly Sun 04:00"
	s.ConfigOffsiteImmutable = true
	if err := r.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !got.ConfigEnabled || got.ConfigPath != "user/bombvault/config" ||
		got.ConfigSchedule != "daily 03:30" || got.ConfigOffsite != "rclone:remote:bombvault-config" ||
		got.ConfigOffsiteSchedule != "weekly Sun 04:00" || !got.ConfigOffsiteImmutable {
		t.Fatalf("config fields not round-tripped: %+v", got)
	}
}

func TestSettingsFilesFieldsRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	s, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	// Defaults from migration: disabled, canonical path, schedule off.
	if s.FilesEnabled {
		t.Fatal("default files_enabled must be false")
	}
	if s.FilesPath != "user/bombvault/files" {
		t.Fatalf("default files_path wrong: %q", s.FilesPath)
	}
	if s.FilesSchedule != "off" {
		t.Fatalf("default files_schedule wrong: %q", s.FilesSchedule)
	}
	if s.FilesOffsite != "" || s.FilesOffsiteSchedule != "" || s.FilesOffsiteImmutable {
		t.Fatalf("default files off-site fields must be empty/false: %+v", s)
	}

	s.FilesEnabled = true
	s.FilesPath = "user/bombvault/files"
	s.FilesSchedule = "daily 02:30"
	s.FilesOffsite = "rclone:remote:bombvault-files"
	s.FilesOffsiteSchedule = "weekly Sun 03:00"
	s.FilesOffsiteImmutable = true
	if err := r.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !got.FilesEnabled || got.FilesPath != "user/bombvault/files" ||
		got.FilesSchedule != "daily 02:30" || got.FilesOffsite != "rclone:remote:bombvault-files" ||
		got.FilesOffsiteSchedule != "weekly Sun 03:00" || !got.FilesOffsiteImmutable {
		t.Fatalf("files fields not round-tripped: %+v", got)
	}
}

func TestSettingsFlashZipExportRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// Defaults from migration: disabled, empty path, keep 0.
	s, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.FlashZipExportEnabled {
		t.Fatal("default flash_zip_export_enabled must be false")
	}
	if s.FlashZipExportPath != "" {
		t.Fatalf("default flash_zip_export_path wrong: %q", s.FlashZipExportPath)
	}
	if s.FlashZipExportKeep != 0 {
		t.Fatalf("default flash_zip_export_keep wrong: %d", s.FlashZipExportKeep)
	}

	s.FlashZipExportEnabled = true
	s.FlashZipExportPath = "backups/flash-zip"
	s.FlashZipExportKeep = 3
	if err := r.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !got.FlashZipExportEnabled || got.FlashZipExportPath != "backups/flash-zip" ||
		got.FlashZipExportKeep != 3 {
		t.Fatalf("flash zip export fields not round-tripped: %+v", got)
	}
}

func TestSettingsRestartHealthWaitRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// Defaults: enabled, 120s timeout.
	s, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !s.RestartHealthWait {
		t.Fatal("default restart_health_wait must be true")
	}
	if s.RestartHealthTimeoutSec != 120 {
		t.Fatalf("default restart_health_timeout_sec must be 120, got %d", s.RestartHealthTimeoutSec)
	}

	s.RestartHealthWait = false
	s.RestartHealthTimeoutSec = 45
	if err := r.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.RestartHealthWait {
		t.Fatal("restart_health_wait=false not round-tripped")
	}
	if got.RestartHealthTimeoutSec != 45 {
		t.Fatalf("restart_health_timeout_sec not round-tripped: got %d", got.RestartHealthTimeoutSec)
	}
}

func TestSettingsReconcileUnraidUpdateStatusRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	s, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !s.ReconcileUnraidUpdateStatus {
		t.Fatal("default reconcile_unraid_update_status must be true")
	}

	s.ReconcileUnraidUpdateStatus = false
	if err := r.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.ReconcileUnraidUpdateStatus {
		t.Fatal("reconcile_unraid_update_status=false not round-tripped")
	}
}

// TestSettingsRegistryAuthsRoundTrip expects registry_auths to default to empty
// (anonymous pulls) and the encrypted blob to round-trip unchanged.
func TestSettingsRegistryAuthsRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	s, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.RegistryAuths != "" {
		t.Fatalf("default registry_auths must be empty, got %q", s.RegistryAuths)
	}

	const blob = "b64-ciphertext-opaque"
	s.RegistryAuths = blob
	if err := r.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.RegistryAuths != blob {
		t.Fatalf("registry_auths not round-tripped: %q", got.RegistryAuths)
	}

	got.RegistryAuths = ""
	if err := r.UpdateSettings(got); err != nil {
		t.Fatal(err)
	}
	cleared, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if cleared.RegistryAuths != "" {
		t.Fatalf("registry_auths not cleared: %q", cleared.RegistryAuths)
	}
}

// TestSettingsWidgetTokenRoundTrip expects widget_token to default to empty
// (widget off), and setting and clearing it to persist.
func TestSettingsWidgetTokenRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	s, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.WidgetToken != "" {
		t.Fatalf("default widget_token must be empty (widget off), got %q", s.WidgetToken)
	}

	const tok = "0123456789abcdef0123456789abcdef"
	s.WidgetToken = tok
	if err := r.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.WidgetToken != tok {
		t.Fatalf("widget_token not round-tripped: %q", got.WidgetToken)
	}

	got.WidgetToken = ""
	if err := r.UpdateSettings(got); err != nil {
		t.Fatal(err)
	}
	cleared, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if cleared.WidgetToken != "" {
		t.Fatalf("widget_token not cleared: %q", cleared.WidgetToken)
	}
}

// TestSettingsEverythingFieldsRoundTrip expects the "Backup Everything"
// schedule to default to 'off' with empty hooks, and all three fields to
// round-trip.
func TestSettingsEverythingFieldsRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	s, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.EverythingSchedule != "off" {
		t.Fatalf("default everything_schedule wrong: %q", s.EverythingSchedule)
	}
	if s.EverythingPreHook != "" || s.EverythingPostHook != "" {
		t.Fatalf("default everything hooks must be empty: %+v", s)
	}

	s.EverythingSchedule = "daily 06:00"
	s.EverythingPreHook = "echo starting"
	s.EverythingPostHook = "curl -fsS https://hc-ping.com/your-uuid"
	if err := r.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.EverythingSchedule != "daily 06:00" ||
		got.EverythingPreHook != "echo starting" ||
		got.EverythingPostHook != "curl -fsS https://hc-ping.com/your-uuid" {
		t.Fatalf("everything fields not round-tripped: %+v", got)
	}
}

func TestSettingsAuthPasswordHashRoundtrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// Default must be empty (auth off).
	s, err := r.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if s.AuthPasswordHash != "" {
		t.Fatalf("default auth_password_hash must be empty, got %q", s.AuthPasswordHash)
	}

	const fakeHash = "deadbeef"
	s.AuthPasswordHash = fakeHash
	if err := r.UpdateSettings(s); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	s2, err := r.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings after update: %v", err)
	}
	if s2.AuthPasswordHash != fakeHash {
		t.Fatalf("auth_password_hash not persisted: %q", s2.AuthPasswordHash)
	}

	// Clear the hash (disable auth).
	s2.AuthPasswordHash = ""
	if err := r.UpdateSettings(s2); err != nil {
		t.Fatalf("UpdateSettings (clear): %v", err)
	}
	s3, err := r.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings after clear: %v", err)
	}
	if s3.AuthPasswordHash != "" {
		t.Fatalf("auth_password_hash not cleared: %q", s3.AuthPasswordHash)
	}
}

func TestSettingsRoundTripZFSFields(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	s, err := r.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if s.ZFSEnabled || s.ZFSPath != "user/bombvault/zfs" || s.ZFSSchedule != "off" {
		t.Fatalf("fresh settings are enabled=%v path=%q schedule=%q", s.ZFSEnabled, s.ZFSPath, s.ZFSSchedule)
	}

	s.ZFSEnabled = true
	s.ZFSPath = "user/tank/datasets"
	s.ZFSSchedule = "daily 03:30"
	s.ZFSOffsite = "sftp:box:/srv/zfs"
	s.ZFSOffsiteSchedule = "weekly Sun 05:00"
	s.ZFSOffsiteImmutable = true
	if err := r.UpdateSettings(s); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	back, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if back != s {
		t.Fatalf("settings did not round-trip:\ngot  %+v\nwant %+v", back, s)
	}
}

func TestDBDumpsEnabledRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	s, err := r.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if !s.DBDumpsEnabled {
		t.Fatal("automatic database dumps must be on for an existing install")
	}

	s.DBDumpsEnabled = false
	if err := r.UpdateSettings(s); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	s, err = r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.DBDumpsEnabled {
		t.Fatal("the global switch did not stay off")
	}
}

func TestSettingsAnomalyFieldsRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	s, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !s.AnomalyEnabled || !s.AnomalyRetentionHold {
		t.Fatal("anomaly detection and its retention hold must default to on")
	}
	if s.AnomalySensitivity != "balanced" || s.AnomalyNotifyMin != "critical" {
		t.Fatalf("defaults are (%q, %q)", s.AnomalySensitivity, s.AnomalyNotifyMin)
	}

	got, err := r.MutateSettings(func(m *store.Settings) error {
		m.AnomalyEnabled = false
		m.AnomalySensitivity = "strict"
		m.AnomalyNotifyMin = "warning"
		m.AnomalyRetentionHold = false
		return nil
	})
	if err != nil {
		t.Fatalf("MutateSettings: %v", err)
	}
	if got.AnomalyEnabled || got.AnomalyRetentionHold {
		t.Fatal("the switches did not stay off")
	}

	stored, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if stored.AnomalyEnabled || stored.AnomalyRetentionHold {
		t.Fatal("the switches came back on after a read")
	}
	if stored.AnomalySensitivity != "strict" || stored.AnomalyNotifyMin != "warning" {
		t.Fatalf("preset and minimum round-tripped as (%q, %q)", stored.AnomalySensitivity, stored.AnomalyNotifyMin)
	}
}
