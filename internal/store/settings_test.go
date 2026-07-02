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

	// Immutable off-site defaults: flags off, budget off, weekly tamper test,
	// auto drill target.
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

	s.ContainersPath = "custom/path"
	s.ContainersSchedule = "daily 03:00"
	s.ContainersOffsiteImmutable = true
	s.VMsOffsiteImmutable = true
	s.FlashOffsiteImmutable = true
	s.OffsiteGrowthBudgetGB = 500
	s.TamperTestSchedule = "daily 05:15"
	s.DRDrillTarget = "plex"
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

	// Set a hash.
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
