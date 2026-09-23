package restic

import (
	"math"
	"os"
	"os/exec"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

// copyHelperEnv set to "1" makes a child copy of the test binary print what
// restic 0.17.3 prints for `copy`: two snapshots, each with its own "copy
// started" header and pack percentages. A real child process exercises the
// same stdout pipe and scanner that Restic.Copy uses.
const copyHelperEnv = "BOMBVAULT_RESTIC_COPY_FAKE"

// TestFakeResticCopyOutput is a helper, not a test; see copyHelperEnv.
func TestFakeResticCopyOutput(t *testing.T) {
	if os.Getenv(copyHelperEnv) != "1" {
		return
	}
	for _, line := range []string{
		"",
		"snapshot abc123 of [host] at 2026-08-16 10:00:00:",
		"  copy started, this may take a while...",
		"[0:00]          0 packs copied",
		"[0:01] 50.00%  1 / 2 packs copied",
		"[0:02] 100.00%  2 / 2 packs copied",
		"snapshot def456 saved, copied from source snapshot abc123",
		"",
		"snapshot ghi789 of [host] at 2026-08-16 10:05:00:",
		"  copy started, this may take a while...",
		"[0:00] 33.33%  1 / 3 packs copied",
		"[0:01] 100.00%  3 / 3 packs copied",
		"snapshot jkl012 saved, copied from source snapshot ghi789",
	} {
		os.Stdout.WriteString(line + "\n") //nolint:errcheck,gosec // test helper child process, best-effort write
	}
}

// TestRunStreamingCopyTracksSnapshotBoundaries checks that each "copy started"
// header advances the snapshot index and every percentage is reported against
// the current one, so the second snapshot starting over at a low percentage
// does not read as the first one going backwards.
func TestRunStreamingCopyTracksSnapshotBoundaries(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestFakeResticCopyOutput$") //nolint:gosec // G204: os.Args[0] is this test binary itself, fixed args
	cmd.Env = append(os.Environ(), copyHelperEnv+"=1")

	var got []progress.CopyProgress
	if _, err := runStreamingCopy(cmd, []string{"copy"}, func(cp progress.CopyProgress) {
		got = append(got, cp)
	}); err != nil {
		t.Fatalf("runStreamingCopy: %v", err)
	}

	want := []progress.CopyProgress{
		{SnapshotIndex: 1, Percent: 50},
		{SnapshotIndex: 1, Percent: 100},
		{SnapshotIndex: 2, Percent: 33.33},
		{SnapshotIndex: 2, Percent: 100},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d progress updates %+v, want %d: %+v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i].SnapshotIndex != want[i].SnapshotIndex || math.Abs(got[i].Percent-want[i].Percent) > 0.001 {
			t.Fatalf("update %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// copyHelperNoBoundaryEnv set to "1" makes a child copy of the test binary
// print a percentage line with no "copy started" header before it, as a restic
// release that renamed the header would.
const copyHelperNoBoundaryEnv = "BOMBVAULT_RESTIC_COPY_FAKE_NOBOUNDARY"

// TestFakeResticCopyOutputNoBoundary is a helper, not a test; see
// copyHelperNoBoundaryEnv.
func TestFakeResticCopyOutputNoBoundary(t *testing.T) {
	if os.Getenv(copyHelperNoBoundaryEnv) != "1" {
		return
	}
	for _, line := range []string{
		"",
		"[0:05] 42.00%  1 / 2 packs copied",
	} {
		os.Stdout.WriteString(line + "\n") //nolint:errcheck,gosec // test helper child process, best-effort write
	}
}

// TestRunStreamingCopyNoBoundaryStillReportsSnapshotOne checks that a
// percentage seen before any "copy started" header counts for snapshot 1, not
// snapshot 0, so progress survives restic renaming the header.
func TestRunStreamingCopyNoBoundaryStillReportsSnapshotOne(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestFakeResticCopyOutputNoBoundary$") //nolint:gosec // G204: os.Args[0] is this test binary itself, fixed args
	cmd.Env = append(os.Environ(), copyHelperNoBoundaryEnv+"=1")

	var got []progress.CopyProgress
	if _, err := runStreamingCopy(cmd, []string{"copy"}, func(cp progress.CopyProgress) {
		got = append(got, cp)
	}); err != nil {
		t.Fatalf("runStreamingCopy: %v", err)
	}

	if len(got) != 1 || got[0].SnapshotIndex != 1 || math.Abs(got[0].Percent-42.0) > 0.001 {
		t.Fatalf("expected a boundary-less percent line to be attributed to snapshot 1, got %+v", got)
	}
}

func TestPendingCopyIDs(t *testing.T) {
	src := []Snapshot{
		{ID: "src1"},
		{ID: "src2"},
		{ID: "src3", Original: "origABC"}, // src3 is itself already a copy of origABC
	}
	dst := []Snapshot{
		{ID: "dst1", Original: "src1"}, // src1 already has a copy at dest
		{ID: "origABC"},                // src3's effective identity is already present at dest, by raw id
	}
	got := PendingCopyIDs(src, dst)
	want := []string{"src2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PendingCopyIDs = %v, want %v", got, want)
	}
}

func TestPendingCopyIDsEmptyDestMeansEverythingPending(t *testing.T) {
	src := []Snapshot{{ID: "a"}, {ID: "b"}}
	got := PendingCopyIDs(src, nil)
	if len(got) != 2 {
		t.Fatalf("expected both source snapshots pending against an empty destination, got %v", got)
	}
}

func TestPendingCopyIDsNothingPendingWhenAllCopied(t *testing.T) {
	src := []Snapshot{{ID: "a"}}
	dst := []Snapshot{{ID: "x", Original: "a"}}
	if got := PendingCopyIDs(src, dst); len(got) != 0 {
		t.Fatalf("expected nothing pending, got %v", got)
	}
}

func TestPendingCopyIDsEmptySrcIsEmpty(t *testing.T) {
	dst := []Snapshot{{ID: "x"}}
	if got := PendingCopyIDs(nil, dst); len(got) != 0 {
		t.Fatalf("expected no pending ids for an empty source list, got %v", got)
	}
}
