package api

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// zfsWithoutOwnFolder is a ZFS section whose own folder is switched off and has
// no location, with one dataset on a named repository and one target.
func zfsWithoutOwnFolder(t *testing.T) (*placementFixture, string) {
	t.Helper()
	f := newPlacementFixture(t)
	settings := settingsOf(t, f.svc)
	settings.ZFSPath = ""
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := f.st.SetLocal(zfsDomain, false); err != nil {
		t.Fatal(err)
	}
	f.target(zfsDomain, "B2", "b2:bucket:zfs")
	cold := f.namedRepo("Cold", "backups/cold")
	item, err := f.st.CreateZFSDataset(store.ZFSDataset{Dataset: "tank/data", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.SetZFSDatasetRepo(item.ID, cold.ID); err != nil {
		t.Fatal(err)
	}
	loc := f.root + "/backups/cold"
	f.hold(loc, snap("z1", 100, "zfs:tank/data"))
	return f, loc
}

func TestASectionWithoutOwnFolderStillCopiesItsNamedRepositories(t *testing.T) {
	f, cold := zfsWithoutOwnFolder(t)

	if err := f.svc.ReplicateOffsite(context.Background(), zfsDomain); err != nil {
		t.Fatal(err)
	}

	if !slices.ContainsFunc(f.eng.copies, func(c copyCall) bool { return c.Src == cold }) {
		t.Fatalf("copies = %+v, want one from %s", f.eng.copies, cold)
	}
}

func TestReplicateNowStartsForASectionWithoutOwnFolder(t *testing.T) {
	f, _ := zfsWithoutOwnFolder(t)

	if err := f.svc.StartReplicateOffsite(zfsDomain); err != nil {
		t.Fatalf("Replicate now = %v, want it started", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		if unlock, ok := f.svc.tryLockDomainFor(zfsDomain, "test"); ok {
			unlock()
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the replication did not end")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestReplicateNowOnAPausedSectionSaysSoInsteadOfStarting(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	if _, err := f.st.PausePlacement("containers"); err != nil {
		t.Fatal(err)
	}

	res := f.do("POST", "/api/offsite/containers", nil)

	if res["ok"] != false || res["code"] != "replication-paused" {
		t.Fatalf("answer = %v, want replication-paused", res)
	}
	if unlock, ok := f.svc.tryLockDomainFor("containers", "test"); !ok {
		t.Fatal("the refused start kept the section")
	} else {
		unlock()
	}
}

func TestReplicateNowWithNothingToCopySaysSoInsteadOfStarting(t *testing.T) {
	f := newPlacementFixture(t)
	f.target(zfsDomain, "B2", "b2:bucket:zfs")
	if err := f.st.SetLocal(zfsDomain, false); err != nil {
		t.Fatal(err)
	}

	res := f.do("POST", "/api/offsite/zfs", nil)

	if res["ok"] != false || res["code"] != "nothing-to-copy" {
		t.Fatalf("answer = %v, want nothing-to-copy", res)
	}
}

func TestReplicateNowThatStartsLeavesARun(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("files", "B2", "b2:bucket:files")
	f.hold(f.domainPath("files"), snap("f1", time.Now().Unix(), "fileset:iso"))
	f.setDefault("files", "")

	if res := f.do("POST", "/api/offsite/files", nil); res["ok"] != true {
		t.Fatalf("answer = %v, want it started", res)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		runs, err := f.st.ListRuns(20)
		if err != nil {
			t.Fatal(err)
		}
		if slices.ContainsFunc(runs, func(r store.Run) bool { return r.Kind == "offsite" && r.TargetID == "files" && r.Status != "running" }) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("runs = %+v, want the replication that said it started", runs)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
