package zfsrepl

import (
	"context"
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
