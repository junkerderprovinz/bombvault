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

func TestTheOffsiteFieldsTargetKeepsTheKeepPolicySetOnItsLocation(t *testing.T) {
	s, st := newSyncTestService(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.OffsiteRetentionKeepDaily = 14
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	setContainersField(t, s, st, "s3:containers")
	primary := fieldTarget(t, st)
	if primary.RetentionKeepDaily != 14 {
		t.Fatalf("new primary keeps %d daily, want the 14 the off-site settings start it with", primary.RetentionKeepDaily)
	}

	primary.RetentionKeepDaily, primary.RetentionKeepWeekly = 30, 4
	if _, err := st.UpsertOffsiteTarget(primary); err != nil {
		t.Fatal(err)
	}
	settings, err = st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.OffsiteRetentionKeepDaily, settings.OffsiteRetentionKeepWeekly = 7, 0
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	s.syncAllPrimaryOffsiteTargets(settings)

	if got := fieldTarget(t, st); got.RetentionKeepDaily != 30 || got.RetentionKeepWeekly != 4 {
		t.Fatalf("after a settings save the primary keeps %d daily, %d weekly, want 30 and 4", got.RetentionKeepDaily, got.RetentionKeepWeekly)
	}
	if p := s.retentionPolicyForSource(settings, "containers", "offsite"); p.KeepDaily != 30 || p.KeepWeekly != 4 {
		t.Fatalf("an off-site prune ages by %+v, want the target's 30 daily and 4 weekly", p)
	}
	if p := s.offsiteRetentionPolicy(settings, "containers"); p.KeepDaily != 30 || p.KeepWeekly != 4 {
		t.Fatalf("the built-in off-site policy is %+v, want the target's", p)
	}
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

// Without a primary, an additional target already on the repo the settings
// name becomes the primary, so the domain does not replicate there twice.
func TestSyncPrimaryOffsiteTargetAdoptsAdditionalTargetOnTheSameRepo(t *testing.T) {
	s, st := newSyncTestService(t)

	same, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Primary", Repo: "s3:containers", CredsRef: "set-a", Enabled: true, SortOrder: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Second copy", Repo: "s3:containers-extra", Enabled: true, SortOrder: 2,
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
		t.Fatalf("want the existing row as primary and no duplicate, got %+v", targets)
	}
	if targets[0].ID != same.ID || targets[0].SortOrder != 0 || targets[0].CredsRef != "set-a" {
		t.Fatalf("primary = %+v, want row %s at sort order 0 with its credential set", targets[0], same.ID)
	}
	if targets[1].ID != other.ID || targets[1].SortOrder != 2 {
		t.Fatalf("other additional target changed: %+v", targets[1])
	}
}

// A settings save syncs the ZFS domain like the others: its additional target
// survives while the domain has no off-site repo, and becomes the primary once
// the settings name its repo.
func TestSettingsSaveKeepsThenAdoptsZFSAdditionalTarget(t *testing.T) {
	s, st := newSyncTestService(t)

	extra, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: zfsDomain, Name: "Second copy", Repo: "s3:zfs", CredsRef: "set-z", Enabled: true, SortOrder: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.syncAllPrimaryOffsiteTargets(settings)
	targets := s.offsiteTargetsFor(zfsDomain)
	if len(targets) != 1 || targets[0].ID != extra.ID || targets[0].SortOrder != 1 {
		t.Fatalf("additional zfs target should survive a settings save, got %+v", targets)
	}

	settings.ZFSOffsite = "s3:zfs"
	s.syncAllPrimaryOffsiteTargets(settings)
	targets = s.offsiteTargetsFor(zfsDomain)
	if len(targets) != 1 || targets[0].ID != extra.ID || targets[0].SortOrder != 0 || targets[0].CredsRef != "set-z" {
		t.Fatalf("want the existing zfs row as primary and no duplicate, got %+v", targets)
	}
}

// Additional targets the API once created without a sort order sit at 0 next
// to the primary. The startup repair moves them behind it, mesh or not, so a
// settings save neither deletes nor rewrites them.
func TestStartupMovesEveryAdditionalTargetOffThePrimarySortOrder(t *testing.T) {
	s, st := newSyncTestService(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersOffsite = "s3:primary"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	for _, tg := range []store.OffsiteTarget{
		{ID: "early", Domain: "containers", Name: "Early", Repo: "s3:early", Enabled: true, CreatedAt: 100},
		{ID: "primary", Domain: "containers", Name: "Primary", Repo: "s3:primary", Enabled: true, CreatedAt: 200},
		{ID: "lone", Domain: "vms", Name: "Lone", Repo: "s3:lone", Enabled: true, CreatedAt: 300},
	} {
		if _, err := st.UpsertOffsiteTarget(tg); err != nil {
			t.Fatal(err)
		}
	}

	moved, err := s.MoveTargetsOffPrimarySlot()
	if err != nil {
		t.Fatal(err)
	}
	s.syncAllPrimaryOffsiteTargets(settings)

	if moved != 2 {
		t.Fatalf("moved = %d, want 2", moved)
	}
	for id, repo := range map[string]string{"early": "s3:early", "primary": "s3:primary", "lone": "s3:lone"} {
		got, ok, err := st.GetOffsiteTarget(id)
		if err != nil || !ok {
			t.Fatalf("target %s is gone (err %v)", id, err)
		}
		if got.Repo != repo || (id == "primary") != (got.SortOrder == 0) {
			t.Fatalf("target %s = %s at sort order %d", id, got.Repo, got.SortOrder)
		}
	}
}

// Two sections whose off-site fields point at different places get targets
// with names the storage pages can tell apart.
func TestSyncPrimaryOffsiteTargetNamesANewTargetAfterItsPlace(t *testing.T) {
	s, st := newSyncTestService(t)
	settings, err := st.MutateSettings(func(s *store.Settings) error {
		s.ConfigOffsite = "user/offsite/config"
		s.FilesOffsite = "rest:http://192.168.20.185:8000/rc-b-files"
		s.VMsOffsite = "rest:http://other:8000/rc-b-files"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"config": "config", "files": "rc-b-files", "vms": "rc-b-files 2"}
	for _, d := range []string{"config", "files", "vms"} {
		if err := s.syncPrimaryOffsiteTarget(d, settings); err != nil {
			t.Fatal(err)
		}
		got, _, err := st.FieldOffsiteTarget(d)
		if err != nil || got.Name != want[d] {
			t.Fatalf("%s target = %q, %v, want %q", d, got.Name, err, want[d])
		}
	}
}

// A name given to the field's target survives the next save of the settings,
// and an existing Primary stays Primary while its place stays the same.
func TestSyncPrimaryOffsiteTargetKeepsANameWhileThePlaceStays(t *testing.T) {
	s, st := newSyncTestService(t)
	settings, err := st.MutateSettings(func(s *store.Settings) error {
		s.ContainersOffsite = "s3:b2/containers"
		s.FilesOffsite = "s3:b2/files"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for d, name := range map[string]string{"containers": "Hetzner", "files": "Primary"} {
		if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: d, Name: name, Repo: offsiteRepoFromSettings(d, settings), Enabled: true}); err != nil {
			t.Fatal(err)
		}
		if err := s.syncPrimaryOffsiteTarget(d, settings); err != nil {
			t.Fatal(err)
		}
		if got, _, _ := st.FieldOffsiteTarget(d); got.Name != name {
			t.Fatalf("%s target = %q after a save, want %q", d, got.Name, name)
		}
	}

	settings, err = st.MutateSettings(func(s *store.Settings) error { s.FilesOffsite = "s3:wasabi/bv-files"; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := s.syncPrimaryOffsiteTarget("files", settings); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := st.FieldOffsiteTarget("files"); got.Name != "bv-files" {
		t.Fatalf("files target = %q after the field moved, want it named after the new place", got.Name)
	}
}

func TestLocationNameIsTheLastFolderOrTheServer(t *testing.T) {
	for repo, want := range map[string]string{
		"rest:http://192.168.20.185:8000/rc-b-files": "rc-b-files",
		"rest:https://u:secret@box:8000/":            "box:8000",
		"user/offsite/config":                        "config",
		"/mnt/user/offsite/":                         "offsite",
		"sftp:u@box:/bv":                             "bv",
		"rclone:gdrive:":                             "gdrive",
		"b2:bucket":                                  "bucket",
		"s3:s3.amazonaws.com/bucket/flash":           "flash",
	} {
		if got := locationName(repo); got != want {
			t.Errorf("locationName(%q) = %q, want %q", repo, got, want)
		}
	}
}
