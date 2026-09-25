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

// After the off-site repo changes in Settings, the sync updates the existing
// primary row, so offsiteRepoFor returns the new repo and the domain keeps one
// target with the same id.
func TestSyncPrimaryOffsiteTargetUpdatesInPlace(t *testing.T) {
	s, st := newSyncTestService(t)

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

	settings.ContainersOffsite = "s3:new"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := s.syncPrimaryOffsiteTarget("containers", settings); err != nil {
		t.Fatalf("syncPrimaryOffsiteTarget: %v", err)
	}

	if got := s.offsiteRepoFor("containers", settings); got != "s3:new" {
		t.Fatalf("offsiteRepoFor after sync = %q, want s3:new", got)
	}

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

// Clearing the off-site repo deletes the primary target, so offsiteRepoFor
// cannot return a stale repo.
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

// An additional target the user added in the targets section is not the
// primary, so saving settings with an empty off-site repo leaves it alone.
func TestSyncPrimaryOffsiteTargetKeepsAdditionalTargetWhenRepoIsEmpty(t *testing.T) {
	s, st := newSyncTestService(t)

	extra, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Second copy", Repo: "s3:containers-extra", Enabled: true, SortOrder: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.syncPrimaryOffsiteTarget("containers", settings); err != nil {
		t.Fatal(err)
	}
	targets := s.offsiteTargetsFor("containers")
	if len(targets) != 1 || targets[0].ID != extra.ID || targets[0].Repo != "s3:containers-extra" {
		t.Fatalf("additional target should survive a settings save, got %+v", targets)
	}
}

// With only an additional target in place, setting the off-site repo adds a
// primary next to it instead of pointing the additional target elsewhere.
func TestSyncPrimaryOffsiteTargetAddsPrimaryBesideAdditionalTarget(t *testing.T) {
	s, st := newSyncTestService(t)

	extra, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Second copy", Repo: "s3:containers-extra", Enabled: true, SortOrder: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersOffsite = "s3:containers"
	if err := s.syncPrimaryOffsiteTarget("containers", settings); err != nil {
		t.Fatal(err)
	}
	targets := s.offsiteTargetsFor("containers")
	if len(targets) != 2 {
		t.Fatalf("want primary and additional target, got %+v", targets)
	}
	if targets[0].SortOrder != 0 || targets[0].Repo != "s3:containers" {
		t.Fatalf("primary = %+v, want sort order 0 on s3:containers", targets[0])
	}
	if targets[1].ID != extra.ID || targets[1].Repo != "s3:containers-extra" {
		t.Fatalf("additional target changed: %+v", targets[1])
	}
}
