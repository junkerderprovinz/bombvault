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

// setContainersField stores the containers off-site field and syncs its target
// row, the way a settings save does.
func setContainersField(t *testing.T, s *Service, st *store.Repo, location string) store.Settings {
	t.Helper()
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersOffsite = location
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := s.syncPrimaryOffsiteTarget("containers", settings); err != nil {
		t.Fatalf("syncPrimaryOffsiteTarget: %v", err)
	}
	return settings
}

func fieldTarget(t *testing.T, st *store.Repo) store.OffsiteTarget {
	t.Helper()
	tg, ok, err := st.FieldOffsiteTarget("containers")
	if err != nil || !ok {
		t.Fatalf("FieldOffsiteTarget: ok=%v err=%v", ok, err)
	}
	return tg
}

func TestClearingTheOffsiteFieldSwitchesItsTargetOff(t *testing.T) {
	s, st := newSyncTestService(t)
	setContainersField(t, s, st, "s3:b2")
	first := fieldTarget(t, st)
	first.CredsRef = "set-b2"
	if _, err := st.UpsertOffsiteTarget(first); err != nil {
		t.Fatal(err)
	}

	settings := setContainersField(t, s, st, "")
	off := fieldTarget(t, st)
	if off.ID != first.ID || off.Enabled || off.Repo != "s3:b2" || off.CredsRef != "set-b2" {
		t.Fatalf("after clearing: %+v, want the same row switched off with location and credentials", off)
	}
	if got := s.offsiteRepoFor("containers", settings); got != "" {
		t.Fatalf("offsiteRepoFor after clearing = %q, want empty", got)
	}

	setContainersField(t, s, st, "s3:b2")
	back := fieldTarget(t, st)
	if back.ID != first.ID || !back.Enabled || back.CredsRef != "set-b2" {
		t.Fatalf("after filling again: %+v, want target %s switched on", back, first.ID)
	}
}

func TestTheOffsiteFieldLeavesAMeshTargetOnZeroAlone(t *testing.T) {
	s, st := newSyncTestService(t)
	setContainersField(t, s, st, "s3:b2")
	field := fieldTarget(t, st)
	mesh, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "mesh: tower", Repo: "rest:http://tower:8000/containers",
		CredsRef: "mesh-tower", Enabled: true, CreatedAt: field.CreatedAt + 60,
	})
	if err != nil {
		t.Fatal(err)
	}

	setContainersField(t, s, st, "")
	setContainersField(t, s, st, "s3:b2-eu")

	got, ok, err := st.GetOffsiteTarget(mesh.ID)
	if err != nil || !ok || got != mesh {
		t.Fatalf("mesh target = %+v (ok=%v err=%v), want it unchanged: %+v", got, ok, err, mesh)
	}
	if now := fieldTarget(t, st); now.ID != field.ID || now.Repo != "s3:b2-eu" || !now.Enabled {
		t.Fatalf("field target = %+v, want %s on s3:b2-eu", now, field.ID)
	}
}

func TestFillingTheOffsiteFieldWithNoRowOnZeroAddsOne(t *testing.T) {
	s, st := newSyncTestService(t)
	mesh, err := st.CreateOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "mesh: tower", Repo: "rest:http://tower:8000/containers", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	setContainersField(t, s, st, "s3:b2")

	field := fieldTarget(t, st)
	if field.ID == mesh.ID || field.Repo != "s3:b2" || !field.Enabled {
		t.Fatalf("field target = %+v, want a new row on s3:b2", field)
	}
	if got, _, _ := st.GetOffsiteTarget(mesh.ID); got != mesh {
		t.Fatalf("mesh target changed: %+v, want %+v", got, mesh)
	}
}
