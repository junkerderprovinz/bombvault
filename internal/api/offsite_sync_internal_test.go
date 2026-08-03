package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func newSyncTestService(t *testing.T) (*Service, *store.Repo) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)
	return &Service{store: st}, st
}

// TestSyncPrimaryOffsiteTargetDualWrite is the core edit-regression proof: after
// the off-site repo is changed through the legacy Settings columns, syncing the
// primary target makes offsiteRepoFor return the NEW repo AND updates the existing
// primary row IN PLACE (same id, still a single target — N=1 identity preserved).
func TestSyncPrimaryOffsiteTargetDualWrite(t *testing.T) {
	s, st := newSyncTestService(t)

	// A backfilled N=1 install: one primary target mirroring the Settings column.
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersOffsite = "s3:old"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	primary, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Primary", Repo: "s3:old", Enabled: true, SortOrder: 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The user edits the off-site repo through the existing setter (Settings).
	settings.ContainersOffsite = "s3:new"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := s.syncPrimaryOffsiteTarget("containers", settings); err != nil {
		t.Fatalf("syncPrimaryOffsiteTarget: %v", err)
	}

	// offsiteRepoFor (read via the target rows) now sees the new repo.
	if got := s.offsiteRepoFor("containers", settings); got != "s3:new" {
		t.Fatalf("offsiteRepoFor after sync = %q, want s3:new", got)
	}

	// Still exactly one target for the domain, and it is the SAME row updated in
	// place (id preserved) — not a duplicate.
	targets, err := st.OffsiteTargetsForDomain("containers")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 {
		t.Fatalf("want 1 target after sync (N=1 identity), got %d", len(targets))
	}
	if targets[0].ID != primary.ID {
		t.Fatalf("primary id changed: %q -> %q", primary.ID, targets[0].ID)
	}
	if targets[0].Repo != "s3:new" {
		t.Fatalf("primary repo = %q, want s3:new", targets[0].Repo)
	}
}

// TestSyncPrimaryOffsiteTargetCreatesWhenMissing: a post-backfill install that
// configured off-site only through Settings (no target row yet) gets a primary
// row synthesized on the next save.
func TestSyncPrimaryOffsiteTargetCreatesWhenMissing(t *testing.T) {
	s, st := newSyncTestService(t)

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.VMsOffsite = "s3:vms"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := s.syncPrimaryOffsiteTarget("vms", settings); err != nil {
		t.Fatal(err)
	}
	targets := s.offsiteTargetsFor("vms")
	if len(targets) != 1 || targets[0].Repo != "s3:vms" {
		t.Fatalf("expected one synthesized vms target with repo s3:vms, got %+v", targets)
	}
}

// TestSyncPrimaryOffsiteTargetDeletesWhenCleared: clearing the off-site repo in
// Settings drops the primary target so offsiteRepoFor falls back to the (now
// empty) column instead of returning a stale repo.
func TestSyncPrimaryOffsiteTargetDeletesWhenCleared(t *testing.T) {
	s, st := newSyncTestService(t)

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.FlashOffsite = "s3:flash"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "flash", Name: "Primary", Repo: "s3:flash", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	// Off-site turned off for flash.
	settings.FlashOffsite = ""
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := s.syncPrimaryOffsiteTarget("flash", settings); err != nil {
		t.Fatal(err)
	}
	if got := s.offsiteRepoFor("flash", settings); got != "" {
		t.Fatalf("offsiteRepoFor(flash) after clear = %q, want empty", got)
	}
	if targets := s.offsiteTargetsFor("flash"); len(targets) != 0 {
		t.Fatalf("primary flash target should be deleted, got %d", len(targets))
	}
}
