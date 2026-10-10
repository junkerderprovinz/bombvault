package zfsrepl

import (
	"context"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// selfPruning is a target that runs its own retention, as a paired instance
// does behind its receive slot.
type selfPruning struct{ *fakeHost }

func (selfPruning) PrunesItself() {}

func TestARunLeavesTheSnapshotsOfASelfPruningTargetAlone(t *testing.T) {
	r := newRig(t)
	r.entry.Keep = store.RetentionKeep{KeepLast: 1}
	target := selfPruning{r.dst}
	for range 3 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		res, err := Run(ctx, r.src, target, r.entry)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		r.ok(res)
		if p := member(t, res, "cache/appdata").Pruned; len(p) != 0 {
			t.Fatalf("pruned %q on a target that prunes itself", p)
		}
		r.entry.Placeholders = append(r.entry.Placeholders, res.Created...)
		r.now = r.now.Add(24 * time.Hour)
	}
	if got := r.dst.snapNames(rootTarget); len(got) != 3 {
		t.Fatalf("target snapshots = %q, want all three runs", got)
	}
	if calls := r.dst.callsOf("destroy"); len(calls) != 0 {
		t.Fatalf("the run destroyed on the target: %q", calls)
	}
}
