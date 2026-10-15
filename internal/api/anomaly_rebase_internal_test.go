package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// rebaseRig is an import rig whose data folder lies in a ZFS tree that holds a
// second app next to it, and in a folder set of its own.
func rebaseRig(t *testing.T) (*importRig, string) {
	t.Helper()
	rig := newImportRig(t)
	if err := rig.svc.store.ReplaceZFSMembers("zfs-tank", []store.ZFSMember{
		{Dataset: "tank/appdata", HostMountpoint: "/data"},
		{Dataset: "tank/appdata/pg", HostMountpoint: "/data/pg"},
		{Dataset: "tank/appdata/web", HostMountpoint: "/data/web"},
	}); err != nil {
		t.Fatal(err)
	}
	set, err := rig.svc.store.CreateFileSet(store.FileSet{Name: "pg-data", Path: "pg", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	return rig, set.ID
}

func rebasedAt(t *testing.T, st *store.Repo, kind, id string) int64 {
	t.Helper()
	at, err := st.AnomalyRebaseAt(kind, id)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

func TestRemovingAKeptDataFolderRestartsTheBackupsOfTheFolderAbove(t *testing.T) {
	rig, setID := rebaseRig(t)
	const stamp = "20261015-224138"
	kept := rig.dataDir + dbImportKept + stamp
	if err := os.MkdirAll(kept, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kept, "PG_VERSION"), []byte("16\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := rig.svc.RemoveKeptDataDir(context.Background(), "pg", stamp); err != nil {
		t.Fatal(err)
	}

	if rebasedAt(t, rig.svc.store, anomalyScopeZFSDS, "tank/appdata") == 0 {
		t.Error("the dataset that held the kept folder judges its next backup against the files it lost")
	}
	for _, untouched := range []store.AnomalyScopeKey{
		{Kind: anomalyScopeZFSDS, ID: "tank/appdata/pg"},
		{Kind: anomalyScopeZFSDS, ID: "tank/appdata/web"},
		{Kind: anomalyScopeItem, ID: setID},
	} {
		if rebasedAt(t, rig.svc.store, untouched.Kind, untouched.ID) != 0 {
			t.Errorf("%s:%s starts learning again although the kept folder was not in it", untouched.Kind, untouched.ID)
		}
	}
}

func TestAnImportRestartsTheBackupsOfTheDatabaseAndOfTheFolderAbove(t *testing.T) {
	rig, setID := rebaseRig(t)
	tg, err := rig.svc.store.GetTargetByContainer("pg")
	if err != nil {
		t.Fatal(err)
	}

	if started, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil || !started {
		t.Fatalf("started=%v err=%v", started, err)
	}
	waitForDetachedRun(t, rig.svc)

	for _, changed := range []store.AnomalyScopeKey{
		{Kind: anomalyScopeZFSDS, ID: "tank/appdata"},
		{Kind: anomalyScopeZFSDS, ID: "tank/appdata/pg"},
		{Kind: anomalyScopeItem, ID: setID},
		{Kind: anomalyScopeItem, ID: tg.ID},
		{Kind: anomalyScopeDump, ID: tg.ID},
	} {
		if rebasedAt(t, rig.svc.store, changed.Kind, changed.ID) == 0 {
			t.Errorf("%s:%s judges the backup after the import against the database it replaced", changed.Kind, changed.ID)
		}
	}
	if rebasedAt(t, rig.svc.store, anomalyScopeZFSDS, "tank/appdata/web") != 0 {
		t.Error("the app next to the database starts learning again")
	}
}

func TestTheEngineLeavesASourceBombVaultChangedToTheNextBackup(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 11, 40*gib)
	if err := f.st.RebaseAnomalyScopes([]store.AnomalyScopeKey{{Kind: anomalyScopeItem, ID: id}}, f.now-120); err != nil {
		t.Fatal(err)
	}
	f.run(t, id, "backup", f.now-60, 20*mib)

	f.pass(t)

	if rows := f.openRows(t); len(rows) != 0 {
		t.Fatalf("the backup after BombVault's own change raised %+v", rows)
	}
}
