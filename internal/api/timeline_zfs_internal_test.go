package api

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// zfsCopy is the copy of a ZFS member snapshot at a target.
func zfsCopy(id string, of restic.Snapshot) restic.Snapshot {
	of.ID, of.Original = id, of.ID
	return of
}

func TestZFSTimelineShowsARunAsOneRowOverItsPlaces(t *testing.T) {
	f := newPlacementFixture(t)
	tank, home := f.zfsItem("tank/apps")
	nasLoc := f.localRepo("copies/zfs")
	nas := f.target(zfsDomain, "NAS copy", "copies/zfs")
	root, db := zfsSnap("a1a1a1a1", "tank/apps", firstRun), zfsSnap("a2a2a2a2", "tank/apps/db", firstRun)
	f.hold(home, root, db,
		zfsSnap("b1b1b1b1", "tank/apps", secondRun),
		zfsSnap("d1d1d1d1", "tank/other", firstRun),
	)
	f.hold(nasLoc, zfsCopy("c1c1c1c1", root))
	f.zfsRun(tank.ID, firstRun, "tank/apps", "tank/apps/db")

	tl, err := f.svc.Timeline(context.Background(), zfsDomain, tank.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tl.Places) != 2 || tl.Places[0].Kind != "home" || tl.Places[1].Place != "offsite:"+nas.ID || tl.Places[1].State != "read" {
		t.Fatalf("places = %+v", tl.Places)
	}
	if got := rowKeys(tl.Rows); !slices.Equal(got, []string{secondRun, firstRun}) {
		t.Fatalf("rows = %v, want one per run, newest first", got)
	}
	if want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339); tl.Rows[1].Time != want {
		t.Errorf("time = %s, want the instant of the stamp, %s", tl.Rows[1].Time, want)
	}
	local := markOf(t, tl.Rows, firstRun, "local")
	if !slices.Equal(local.SnapshotIDs, []string{"a1a1a1a1", "a2a2a2a2"}) || local.Incomplete {
		t.Errorf("home mark = %+v, want both datasets, the root first", local)
	}
	if !slices.Contains(local.Tags, "zfs:tank/apps") || !slices.Contains(local.Tags, "zfs:tank/apps/db") {
		t.Errorf("home tags = %v, want those of every dataset", local.Tags)
	}
	if copyMark := markOf(t, tl.Rows, firstRun, "offsite:"+nas.ID); !slices.Equal(copyMark.SnapshotIDs, []string{"c1c1c1c1"}) || !copyMark.Incomplete {
		t.Errorf("copy mark = %+v, want the one dataset it holds and incomplete", copyMark)
	}
}

func TestZFSTimelineRouteReadsOnePlaceNewestFirst(t *testing.T) {
	f := newPlacementFixture(t)
	tank, home := f.zfsItem("tank/apps")
	f.hold(home, zfsSnap("a1a1a1a1", "tank/apps", firstRun), zfsSnap("b1b1b1b1", "tank/apps", secondRun))

	m := f.do(http.MethodGet, "/api/items/zfs/"+tank.ID+"/timeline?place=local", nil)
	rows, _ := m["rows"].([]any)
	if m["ok"] != true || len(rows) != 2 || rows[0].(map[string]any)["key"] != secondRun {
		t.Fatalf("timeline = %v", m)
	}
}

func TestZFSTimelineDeleteTakesEveryDatasetOfTheRun(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	tank, home := f.zfsItem("tank/apps")
	f.hold(home,
		zfsSnap("a1a1a1a1", "tank/apps", firstRun),
		zfsSnap("a2a2a2a2", "tank/apps/db", firstRun),
		zfsSnap("b1b1b1b1", "tank/apps", secondRun),
	)
	path := "/api/items/zfs/" + tank.ID + "/timeline/" + firstRun
	preview := f.do(http.MethodGet, path+"/delete", nil)
	del, _ := preview["delete"].([]any)
	if preview["ok"] != true || len(del) != 1 {
		t.Fatalf("preview = %v", preview)
	}
	res := f.do(http.MethodDelete, path, map[string]any{"places": del})
	if res["ok"] != true {
		t.Fatalf("delete = %v", res)
	}
	if len(f.eng.deletes) != 1 || !slices.Equal(f.eng.deletes[0].IDs, []string{"a1a1a1a1", "a2a2a2a2"}) {
		t.Fatalf("forgot %+v, want both datasets of the first run", f.eng.deletes)
	}
	held, _ := f.eng.Snapshots(context.Background(), home, restic.Mode{})
	if len(held) != 1 || held[0].ID != "b1b1b1b1" {
		t.Fatalf("left %+v, want the second run", held)
	}
}

// An entry inside another entry's tree, which that one leaves out, writes to
// the same repository. Its backups are its own on every page, the outer
// entry's delete included.
func TestAZFSEntryNestedInAnotherKeepsItsBackups(t *testing.T) {
	const thirdRun = "bombvault-20260103000000"
	f := newPlacementFixture(t)
	f.signIn()
	outer, err := f.st.CreateZFSDataset(store.ZFSDataset{Dataset: "tank/data", ExcludedChildren: []string{"tank/data/vm"}, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	inner, home := f.zfsItem("tank/data/vm")
	f.hold(home,
		zfsSnap("a1a1a1a1", "tank/data", firstRun),
		zfsSnap("b1b1b1b1", "tank/data/vm", secondRun),
		zfsSnap("b2b2b2b2", "tank/data/vm/disk", secondRun),
		// A run no row records, as from a database older than the snapshots.
		zfsSnap("c1c1c1c1", "tank/data/vm", thirdRun),
	)
	f.zfsRun(outer.ID, firstRun, "tank/data")
	f.zfsRun(inner.ID, secondRun, "tank/data/vm", "tank/data/vm/disk")

	for _, c := range []struct {
		id   string
		want []string
	}{
		{outer.ID, []string{firstRun}},
		{inner.ID, []string{thirdRun, secondRun}},
	} {
		tl, err := f.svc.Timeline(context.Background(), zfsDomain, c.id)
		if err != nil {
			t.Fatal(err)
		}
		if got := rowKeys(tl.Rows); !slices.Equal(got, c.want) {
			t.Errorf("rows of %s = %v, want %v", c.id, got, c.want)
		}
	}

	preview := f.do(http.MethodGet, "/api/items/zfs/"+outer.ID+"/timeline/"+secondRun+"/delete", nil)
	if del, _ := preview["delete"].([]any); len(del) != 0 {
		t.Fatalf("the outer entry offers to delete the inner one's run: %v", preview)
	}
	snaps, err := f.svc.zfsItemSnapshots(context.Background(), outer, "local")
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].ID != "a1a1a1a1" {
		t.Fatalf("the outer entry's snapshots are %+v, want only its own", snaps)
	}
}

func TestZFSTimelineOfARealRepositoryGroupsByStamp(t *testing.T) {
	skipWithoutSnapshotStamps(t)
	f := newResticFixture(t)
	d := f.zfsEntry("tank/apps")
	root := f.zfsBackup("tank/apps", firstRun, map[string]string{"a.txt": "one"})
	db := f.zfsBackup("tank/apps/db", firstRun, map[string]string{"b.txt": "two"})
	f.zfsBackup("tank/apps", secondRun, map[string]string{"a.txt": "three"})

	tl, err := f.svc.Timeline(context.Background(), zfsDomain, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := rowKeys(tl.Rows); !slices.Equal(got, []string{secondRun, firstRun}) {
		t.Fatalf("rows = %v", got)
	}
	if mark := markOf(t, tl.Rows, firstRun, "local"); !slices.Equal(mark.SnapshotIDs, []string{root.ID, db.ID}) {
		t.Fatalf("first run holds %v, want %s and %s", mark.SnapshotIDs, root.ID, db.ID)
	}
}
