package api

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// photosOnTheNAS is the case copy rules exist for: a file set on a mounted NAS
// set to Local, and twenty older copies of it at B2, which keeps the last seven.
func photosOnTheNAS(t *testing.T, f *placementFixture) store.OffsiteTarget {
	t.Helper()
	b2 := keepLast(t, f, f.target("files", "B2", "b2:bucket:files"), 7)
	nas := f.namedRepo("NAS", "nas")
	f.fileSet("Photos", nas.ID)
	f.rule("files", "fileset:Photos", store.SkipAll)
	f.replicated("files")
	var local, remote []restic.Snapshot
	for i := range 20 {
		id, at := fmt.Sprintf("p%02d", i), int64(1000+i*100)
		local = append(local, snap(id, at, "fileset:Photos"))
		remote = append(remote, copied("c"+id, id, at, "fileset:Photos"))
	}
	f.hold(f.root+"/nas", local...)
	f.hold("b2:bucket:files", remote...)
	return b2
}

func heldAt(t *testing.T, f *placementFixture, loc string) []restic.Snapshot {
	t.Helper()
	f.eng.mu.Lock()
	defer f.eng.mu.Unlock()
	return f.eng.snaps[loc]
}

func TestOlderCopiesAtATargetNothingIsCopiedToAgeOnce(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := photosOnTheNAS(t, f)

	if err := f.svc.ReplicateOffsite(context.Background(), "files"); err != nil {
		t.Fatalf("ReplicateOffsite = %v, want a green pass", err)
	}
	if n := len(heldAt(t, f, "b2:bucket:files")); n != 7 {
		t.Fatalf("B2 holds %d, want the 7 its keep-policy leaves", n)
	}
	if len(f.eng.copies) != 0 {
		t.Fatalf("copied %+v although nothing goes to B2", f.eng.copies)
	}
	var agingOnly int
	if err := f.db.QueryRow(`SELECT aging_only FROM offsite_runs WHERE offsite_target_id = ? AND ok = 1`, b2.ID).Scan(&agingOnly); err != nil || agingOnly != 1 {
		t.Fatalf("B2's run: aging_only=%d err=%v, want one green run that only aged", agingOnly, err)
	}
	if _, found, err := f.st.LatestSuccessfulOffsiteRunForTarget("files", b2.ID); err != nil || found {
		t.Fatalf("an aging run counts as a replication of B2 (found=%v err=%v)", found, err)
	}
	if rows, err := f.st.ItemCopiesFor("files", "fileset:Photos"); err != nil || len(rows) != 1 || rows[0].SnapshotCount != 7 {
		t.Fatalf("ItemCopiesFor = %+v, %v, want the 7 that are left", rows, err)
	}

	f.eng.forgets, f.eng.prunes, f.eng.lists = nil, nil, map[string]int{}
	if err := f.svc.ReplicateOffsite(context.Background(), "files"); err != nil {
		t.Fatal(err)
	}
	if len(f.eng.forgets)+len(f.eng.prunes) != 0 {
		t.Fatalf("the second pass aged again: forgot %+v, pruned %v", f.eng.forgets, f.eng.prunes)
	}
	if n := f.eng.lists["b2:bucket:files"]; n != 1 {
		t.Fatalf("the second pass listed B2 %d times, want once", n)
	}
}

func TestAnAppendOnlyTargetKeepsItsOlderCopies(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := photosOnTheNAS(t, f)
	b2.Immutable = true
	if _, err := f.st.UpsertOffsiteTarget(b2); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ReplicateOffsite(context.Background(), "files"); err != nil {
		t.Fatal(err)
	}
	if n := len(heldAt(t, f, "b2:bucket:files")); n != 20 || len(f.eng.forgets) != 0 {
		t.Fatalf("an append-only B2 holds %d after %+v, want all 20 and no forget", n, f.eng.forgets)
	}
}

func TestADomainPathNeverCreatedDoesNotStopTheAging(t *testing.T) {
	f := newPlacementFixture(t)
	photosOnTheNAS(t, f)
	if err := os.RemoveAll(f.domainPath("files")); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ReplicateOffsite(context.Background(), "files"); err != nil {
		t.Fatal(err)
	}
	if n := len(heldAt(t, f, "b2:bucket:files")); n != 7 {
		t.Fatalf("B2 holds %d, want 7", n)
	}
}

func TestATargetHoldingNothingOfTheDomainIsNotOpenedAgain(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("files", "B2", "b2:bucket:files")
	hz := f.target("files", "Hetzner", "sftp:u@box:/files")
	nas := f.namedRepo("NAS", "nas")
	f.fileSet("Photos", nas.ID)
	f.setDefault("files", "", store.SkipAll)
	f.replicated("files")
	f.hold(f.root+"/nas", snap("p1", 100, "fileset:Photos"))
	// Hetzner still holds older copies, so the NAS stays a counted source and the
	// pass reaches its targets.
	f.hold("sftp:u@box:/files", copied("c1", "p1", 100, "fileset:Photos"))
	f.listing("files", hz.ID, 500, copiesRow("fileset:Photos", 1, 100))

	for range 2 {
		if err := f.svc.ReplicateOffsite(context.Background(), "files"); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.eng.lists["b2:bucket:files"]; n != 1 {
		t.Fatalf("B2 was listed %d times, want once: its first listing showed nothing of the domain", n)
	}
	if len(f.eng.copies) != 0 {
		t.Fatalf("copied %+v with the default on Local", f.eng.copies)
	}
}

func TestAMixedDomainAgesEveryNameAtTheTarget(t *testing.T) {
	f := newPlacementFixture(t)
	photosOnTheNAS(t, f)
	f.fileSet("Docs", "")
	var local, remote []restic.Snapshot
	for i := range 20 {
		id, at := fmt.Sprintf("d%02d", i), int64(1000+i*100)
		local = append(local, snap(id, at, "fileset:Docs"))
		remote = append(remote, copied("c"+id, id, at, "fileset:Docs"))
	}
	f.hold(f.domainPath("files"), local...)
	f.hold("b2:bucket:files", append(heldAt(t, f, "b2:bucket:files"), remote...)...)

	if err := f.svc.ReplicateOffsite(context.Background(), "files"); err != nil {
		t.Fatal(err)
	}
	if n := len(heldAt(t, f, "b2:bucket:files")); n != 14 {
		t.Fatalf("B2 holds %d, want 7 of each name", n)
	}
}

func TestAnEmptiedTargetLosesItsRows(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("files", "B2", "b2:bucket:files")
	nas := f.namedRepo("NAS", "nas")
	f.fileSet("Photos", nas.ID)
	f.rule("files", "fileset:Photos", store.SkipAll)
	f.replicated("files")
	f.listing("files", b2.ID, 500, copiesRow("fileset:Photos", 20, 400))
	f.hold(f.root+"/nas", snap("p1", 100, "fileset:Photos"))

	if err := f.svc.ReplicateOffsite(context.Background(), "files"); err != nil {
		t.Fatal(err)
	}
	if rows, err := f.st.ItemCopiesFor("files", "fileset:Photos"); err != nil || len(rows) != 0 {
		t.Fatalf("ItemCopiesFor = %+v, %v, want nothing after B2 listed empty", rows, err)
	}
}
