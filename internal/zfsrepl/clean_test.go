package zfsrepl

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

func replicaNames(names []string) []string {
	var out []string
	for _, n := range names {
		if strings.HasPrefix(n, "bombvault-replica-") {
			out = append(out, n)
		}
	}
	return out
}

func TestCleanLeavesOnlyTheNewestBookmarkAndKeepsTheTarget(t *testing.T) {
	r := newRig(t)
	r.ok(r.run())
	last := r.run()
	r.ok(last)
	r.src.snapshot("cache/appdata", "manual")

	if err := Clean(context.Background(), r.src, "cache/appdata", nil); err != nil {
		t.Fatal(err)
	}
	for _, ds := range []string{"cache/appdata", "cache/appdata/plex"} {
		if got := replicaNames(r.src.snapNames(ds)); got != nil {
			t.Errorf("%s still has the replica snapshots %q", ds, got)
		}
		if got := r.src.markNames(ds); !slices.Equal(got, []string{last.Snapshot}) {
			t.Errorf("%s bookmarks = %q, want only the newest", ds, got)
		}
		if holds := r.src.get(ds).holds; len(holds) != 0 {
			t.Errorf("%s still has holds %v", ds, holds)
		}
	}
	if got := r.src.snapNames("cache/appdata"); !slices.Equal(got, []string{"manual"}) {
		t.Errorf("the user's own snapshot went too: %q", got)
	}
	if got := r.dst.snapNames(rootTarget); len(got) != 2 {
		t.Errorf("target snapshots = %q, want both replica snapshots kept", got)
	}
	if err := Clean(context.Background(), r.src, "cache/appdata", nil); err != nil {
		t.Errorf("a second clean failed: %v", err)
	}
}

func TestCleanRemovesTheSnapshotsKeptOnTheSourceToo(t *testing.T) {
	r := newRig(t)
	r.entry.KeepOnSource = 3
	var last Result
	for range 3 {
		last = r.run()
		r.ok(last)
	}
	if got := replicaNames(r.src.snapNames("cache/appdata")); len(got) != 3 {
		t.Fatalf("source snapshots before the clean = %q, want three", got)
	}

	if err := Clean(context.Background(), r.src, "cache/appdata", nil); err != nil {
		t.Fatal(err)
	}
	for _, ds := range []string{"cache/appdata", "cache/appdata/plex"} {
		if got := replicaNames(r.src.snapNames(ds)); got != nil {
			t.Errorf("%s still has the replica snapshots %q", ds, got)
		}
		if got := r.src.markNames(ds); !slices.Equal(got, []string{last.Snapshot}) {
			t.Errorf("%s bookmarks = %q, want only the newest", ds, got)
		}
	}
}

func TestCleanKeepsTheNewestBookmarkAndBookmarksTheSnapshotsAfterIt(t *testing.T) {
	r := newRig(t)
	first := r.run()
	r.ok(first)
	r.src.snapshot("cache/appdata", "bombvault-replica-20261020030000")

	if err := Clean(context.Background(), r.src, "cache/appdata", nil); err != nil {
		t.Fatal(err)
	}
	if got := r.src.markNames("cache/appdata"); !slices.Equal(got, []string{first.Snapshot, "bombvault-replica-20261020030000"}) {
		t.Errorf("bookmarks = %q, want the one the target holds and the newer state", got)
	}
}

func TestAfterACleanAnUnsentSnapshotDoesNotCostTheCommonBase(t *testing.T) {
	r := newRig(t)
	first := r.run()
	r.ok(first)
	// BombVault stopped right after the recursive snapshot of the next run.
	args, err := zfs.ReplicaSnapshotArgs("cache/appdata", zfs.ReplicaSnapshotName(r.now))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.src.Run(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	r.now = r.now.Add(24 * time.Hour)
	if err := Clean(context.Background(), r.src, "cache/appdata", nil); err != nil {
		t.Fatal(err)
	}

	res := r.run()
	r.ok(res)
	for _, ds := range []string{"cache/appdata", "cache/appdata/plex"} {
		if m := member(t, res, ds); m.Base != first.Snapshot || !m.FromBookmark {
			t.Errorf("%s = %+v, want an increment from the bookmark the target shares", ds, m)
		}
	}
}

func TestAfterACleanTheNextRunContinuesFromTheBookmark(t *testing.T) {
	r := newRig(t)
	first := r.run()
	r.ok(first)
	if err := Clean(context.Background(), r.src, "cache/appdata", nil); err != nil {
		t.Fatal(err)
	}
	r.src.resetCalls()
	res := r.run()
	r.ok(res)
	for _, ds := range []string{"cache/appdata", "cache/appdata/plex"} {
		if m := member(t, res, ds); m.Base != first.Snapshot || !m.FromBookmark {
			t.Errorf("%s = %+v, want an increment from the kept bookmark", ds, m)
		}
	}
}

func TestAfterACleanAGoneTargetCopyIsSentInFull(t *testing.T) {
	r := newRig(t)
	r.ok(r.run())
	if err := Clean(context.Background(), r.src, "cache/appdata", nil); err != nil {
		t.Fatal(err)
	}
	delete(r.dst.ds, rootTarget+"/plex")
	delete(r.dst.ds, rootTarget)

	res := r.run()
	r.ok(res)
	if m := member(t, res, "cache/appdata"); m.Base != "" {
		t.Errorf("root = %+v, want a full stream", m)
	}
}

func TestCleanBookmarksNothingOnAnExcludedDataset(t *testing.T) {
	r := newRig(t)
	r.src.add("cache/appdata/cache", "filesystem", false)
	r.entry.Excluded = []string{"cache/appdata/cache"}
	last := r.run()
	r.ok(last)
	// A run that stopped early leaves its recursive snapshot on the excluded
	// dataset, and plex was sent before the item excluded it.
	r.src.snapshot("cache/appdata", "bombvault-replica-20261020030000")
	r.src.snapshot("cache/appdata/cache", "bombvault-replica-20261020030000")

	if err := Clean(context.Background(), r.src, "cache/appdata", []string{"cache/appdata/cache", "cache/appdata/plex"}); err != nil {
		t.Fatal(err)
	}
	if got := replicaNames(r.src.snapNames("cache/appdata/cache")); got != nil {
		t.Errorf("the excluded dataset still has the replica snapshots %q", got)
	}
	if got := r.src.markNames("cache/appdata/cache"); got != nil {
		t.Errorf("the excluded dataset got the bookmarks %q", got)
	}
	if got := r.src.markNames("cache/appdata/plex"); !slices.Equal(got, []string{last.Snapshot}) {
		t.Errorf("plex bookmarks = %q, want the one it had from being sent", got)
	}
}

// receivedCopy gives the host a copy of another BombVault's dataset below cache,
// in a folder marked the way that instance's run marks it.
func receivedCopy(h *fakeHost) string {
	const folder, dataset = "cache/bombvault-replica/other", "cache/bombvault-replica/other/tank/data"
	for _, ds := range []string{"cache/bombvault-replica", folder, folder + "/tank"} {
		h.add(ds, "filesystem", false)
	}
	h.get(folder).props[zfs.SourceProperty] = "the-other-instance"
	h.add(dataset, "filesystem", false)
	h.get(dataset).props["readonly"] = "on"
	for _, s := range []string{"bombvault-replica-20261001030000", "bombvault-replica-20261002030000"} {
		h.snapshot(dataset, s)
	}
	return dataset
}

func TestCleanLeavesAReceivedCopyAlone(t *testing.T) {
	for _, excludes := range [][]string{nil, {"cache/bombvault-replica"}} {
		r := newRig(t)
		dataset := receivedCopy(r.src)
		before := r.src.snapNames(dataset)

		if err := Clean(context.Background(), r.src, "cache", excludes); err != nil {
			t.Fatal(err)
		}
		if got := r.src.snapNames(dataset); !slices.Equal(got, before) {
			t.Errorf("excluding %q: the copy holds %q, want %q", excludes, got, before)
		}
	}
}

func TestCleanOfAnEntryInsideAReceivedCopyLeavesItsHistory(t *testing.T) {
	r := newRig(t)
	dataset := receivedCopy(r.src)
	before := r.src.snapNames(dataset)

	if err := Clean(context.Background(), r.src, dataset, nil); err != nil {
		t.Fatal(err)
	}
	if got := r.src.snapNames(dataset); !slices.Equal(got, before) {
		t.Errorf("the copy holds %q, want %q", got, before)
	}
}

func TestCleanOfAParentLeavesTheKeptSnapshotsOfANestedEntry(t *testing.T) {
	r := newRig(t)
	r.entry.KeepOnSource = 3
	for range 3 {
		r.ok(r.run())
	}
	before := r.src.snapNames("cache/appdata")
	held := maps.Clone(r.src.get("cache/appdata").holds)
	marks := r.src.markNames("cache/appdata")
	// The parent's own snapshot, and the one a run of it left on the dataset it
	// excludes.
	own := zfs.ReplicaSnapshotName(r.now)
	r.src.snapshot("cache", own)
	r.src.snapshot("cache/appdata", own)

	if err := Clean(context.Background(), r.src, "cache", []string{"cache/appdata"}); err != nil {
		t.Fatal(err)
	}
	if got := r.src.snapNames("cache/appdata"); !slices.Equal(got, before) {
		t.Errorf("the nested entry holds %q, want %q without the parent's leftover", got, before)
	}
	if got := r.src.get("cache/appdata").holds; !maps.Equal(got, held) {
		t.Errorf("the nested entry's holds = %v, want %v", got, held)
	}
	if got := r.src.markNames("cache/appdata"); !slices.Equal(got, marks) {
		t.Errorf("the nested entry's bookmarks = %q, want %q", got, marks)
	}
}

func TestARunLeavesAReceivedCopyInItsTreeAloneAndSaysSo(t *testing.T) {
	r := newRig(t)
	dataset := receivedCopy(r.src)
	before := r.src.snapNames(dataset)
	r.entry.Root = "cache"

	res := r.run()
	if m := member(t, res, dataset); m.Code != "received-copy" {
		t.Errorf("the copy: code %q, err %v, want received-copy", m.Code, m.Err)
	}
	if got := r.src.snapNames(dataset); !slices.Equal(got, before) {
		t.Errorf("the copy holds %q, want %q", got, before)
	}
	if m := member(t, res, "cache/appdata"); m.Code != "" {
		t.Errorf("a dataset of the entry itself: code %q, err %v", m.Code, m.Err)
	}
	if got := r.dst.get(base + "/" + dataset); got != nil {
		t.Errorf("the copy was sent on: %+v", got)
	}
}

func TestARunOfAnEntryInsideAReceivedCopyIsRefusedBeforeItsSnapshot(t *testing.T) {
	r := newRig(t)
	dataset := receivedCopy(r.src)
	before := r.src.snapNames(dataset)
	r.entry.Root = dataset

	_, err := Run(context.Background(), r.src, r.dst, r.entry)
	if Code(err) != "received-copy" {
		t.Fatalf("Run = %v, want received-copy", err)
	}
	if got := r.src.snapNames(dataset); !slices.Equal(got, before) {
		t.Errorf("the copy holds %q, want %q", got, before)
	}
}

func TestACopyTakenOverIsNoLongerAReceivedCopy(t *testing.T) {
	r := newRig(t)
	dataset := receivedCopy(r.src)
	d := r.src.get(dataset)
	delete(d.props, "readonly")
	d.mounted = true
	r.entry.Root = dataset

	r.ok(r.run())
}

func TestARunThinsOnlyTheSnapshotsOfItsOwnEntry(t *testing.T) {
	r := newRig(t)
	r.ok(r.run())
	// A replica name this entry neither took nor bookmarked, on a dataset
	// nothing marks as a copy.
	r.src.snapshot("cache/appdata/plex", "bombvault-replica-20261001030000")

	r.ok(r.run())
	if got := r.src.snapNames("cache/appdata/plex"); !slices.Contains(got, "bombvault-replica-20261001030000") {
		t.Errorf("plex holds %q, the snapshot that is not this entry's is gone", got)
	}
}
