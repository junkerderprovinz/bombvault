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
	if s.ContainersPath != "backups/bombvault/containers" {
		t.Fatalf("default containers_path wrong: %q", s.ContainersPath)
	}

	s.ContainersPath = "custom/path"
	s.ContainersSchedule = "daily 03:00"
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
}
