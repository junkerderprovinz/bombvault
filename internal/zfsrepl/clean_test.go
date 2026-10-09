package zfsrepl

import (
	"context"
	"testing"
)

func TestCleanLeavesNoReplicaPointOnTheSourceAndKeepsTheTarget(t *testing.T) {
	r := newRig(t)
	r.ok(r.run())
	last := r.run()
	r.ok(last)
	r.src.snapshot("cache/appdata", "manual")
	// A replica snapshot from before the child was excluded.
	r.src.snapshot("cache/appdata/plex", "bombvault-replica-20261001030000")
	r.entry.Excluded = []string{"cache/appdata/plex"}

	if err := Clean(context.Background(), r.src, "cache/appdata"); err != nil {
		t.Fatal(err)
	}
	for _, ds := range []string{"cache/appdata", "cache/appdata/plex"} {
		for _, name := range r.src.snapNames(ds) {
			if name != "manual" {
				t.Errorf("%s still has the snapshot %s", ds, name)
			}
		}
		if marks := r.src.markNames(ds); len(marks) != 0 {
			t.Errorf("%s still has the bookmarks %q", ds, marks)
		}
		if holds := r.src.get(ds).holds; len(holds) != 0 {
			t.Errorf("%s still has holds %v", ds, holds)
		}
	}
	if got := r.src.snapNames("cache/appdata"); len(got) != 1 {
		t.Errorf("the user's own snapshot went too: %q", got)
	}
	if got := r.dst.snapNames(rootTarget); len(got) != 2 {
		t.Errorf("target snapshots = %q, want both replica snapshots kept", got)
	}
	if err := Clean(context.Background(), r.src, "cache/appdata"); err != nil {
		t.Errorf("a second clean failed: %v", err)
	}
}

func TestAfterACleanTheKeptCopyNeedsAFullTransfer(t *testing.T) {
	r := newRig(t)
	r.ok(r.run())
	if err := Clean(context.Background(), r.src, "cache/appdata"); err != nil {
		t.Fatal(err)
	}
	res := r.run()
	if m := member(t, res, "cache/appdata"); m.Code != "no-common-base" {
		t.Errorf("a run after the clean = %+v, want no-common-base on the kept copy", m)
	}
}
