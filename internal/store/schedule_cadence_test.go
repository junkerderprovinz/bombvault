package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func newRepo(t *testing.T) *store.Repo {
	t.Helper()
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return store.New(db)
}

// TestTargetScheduleCadencePersists proves the per-item container override (#121)
// round-trips, defaults to "", is owned by SetScheduleCadence (not reset by a
// backup-time UpsertTarget), and can be cleared.
func TestTargetScheduleCadencePersists(t *testing.T) {
	r := newRepo(t)

	// A fresh target defaults to no override.
	if _, err := r.UpsertTarget(store.Target{ContainerName: "web", AppdataPaths: []string{"/a"}}); err != nil {
		t.Fatalf("UpsertTarget: %v", err)
	}
	got, err := r.GetTargetByContainer("web")
	if err != nil {
		t.Fatalf("GetTargetByContainer: %v", err)
	}
	if got.ScheduleCadence != "" {
		t.Fatalf("default override: expected empty, got %q", got.ScheduleCadence)
	}

	// Setting an override persists.
	if err := r.SetScheduleCadence("web", "daily 06:00"); err != nil {
		t.Fatalf("SetScheduleCadence: %v", err)
	}
	got, _ = r.GetTargetByContainer("web")
	if got.ScheduleCadence != "daily 06:00" {
		t.Fatalf("after set: got %q, want %q", got.ScheduleCadence, "daily 06:00")
	}

	// A backup-time UpsertTarget refreshes appdata/definition but must NOT reset the
	// override (owned by SetScheduleCadence), like the other per-item setters.
	if _, err := r.UpsertTarget(store.Target{ContainerName: "web", AppdataPaths: []string{"/b"}, Definition: "{}"}); err != nil {
		t.Fatalf("UpsertTarget (refresh): %v", err)
	}
	got, _ = r.GetTargetByContainer("web")
	if got.ScheduleCadence != "daily 06:00" {
		t.Fatalf("override clobbered by Upsert: got %q", got.ScheduleCadence)
	}

	// SetScheduleCadence creates the row if the container has no target yet.
	if err := r.SetScheduleCadence("fresh", "weekly Sun 02:00"); err != nil {
		t.Fatalf("SetScheduleCadence (create): %v", err)
	}
	got, _ = r.GetTargetByContainer("fresh")
	if got.ScheduleCadence != "weekly Sun 02:00" {
		t.Fatalf("create-on-miss: got %q", got.ScheduleCadence)
	}

	// Clearing it (empty string) returns the container to the domain default.
	if err := r.SetScheduleCadence("web", ""); err != nil {
		t.Fatalf("SetScheduleCadence (clear): %v", err)
	}
	got, _ = r.GetTargetByContainer("web")
	if got.ScheduleCadence != "" {
		t.Fatalf("after clear: got %q, want empty", got.ScheduleCadence)
	}
}

// TestVMScheduleCadencePersists proves the per-item VM override (#121) round-trips
// and is not reset by a backup-time UpsertVMTarget.
func TestVMScheduleCadencePersists(t *testing.T) {
	r := newRepo(t)

	if _, err := r.UpsertVMTarget(store.VMTarget{Name: "win"}); err != nil {
		t.Fatalf("UpsertVMTarget: %v", err)
	}
	got, err := r.GetVMTargetByName("win")
	if err != nil {
		t.Fatalf("GetVMTargetByName: %v", err)
	}
	if got.ScheduleCadence != "" {
		t.Fatalf("default override: expected empty, got %q", got.ScheduleCadence)
	}

	if err := r.SetVMScheduleCadence("win", "daily 05:00"); err != nil {
		t.Fatalf("SetVMScheduleCadence: %v", err)
	}
	got, _ = r.GetVMTargetByName("win")
	if got.ScheduleCadence != "daily 05:00" {
		t.Fatalf("after set: got %q", got.ScheduleCadence)
	}

	if _, err := r.UpsertVMTarget(store.VMTarget{Name: "win", Definition: "{}"}); err != nil {
		t.Fatalf("UpsertVMTarget (refresh): %v", err)
	}
	got, _ = r.GetVMTargetByName("win")
	if got.ScheduleCadence != "daily 05:00" {
		t.Fatalf("override clobbered by Upsert: got %q", got.ScheduleCadence)
	}

	// An unknown VM is a not-found error (no create-on-miss for VMs).
	if err := r.SetVMScheduleCadence("ghost", "daily 05:00"); err == nil {
		t.Fatal("expected not-found error for an unknown VM")
	}
}

// TestPerItemSchedulesSettingPersists proves the feature toggle defaults off and
// round-trips through GetSettings/UpdateSettings.
func TestPerItemSchedulesSettingPersists(t *testing.T) {
	r := newRepo(t)

	s, err := r.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if s.PerItemSchedules {
		t.Fatal("PerItemSchedules should default to false")
	}

	s.PerItemSchedules = true
	if err := r.UpdateSettings(s); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	got, _ := r.GetSettings()
	if !got.PerItemSchedules {
		t.Fatal("PerItemSchedules did not persist as true")
	}
}
