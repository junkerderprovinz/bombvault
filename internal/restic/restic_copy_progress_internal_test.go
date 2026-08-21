package restic

import (
	"math"
	"os"
	"os/exec"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

// copyHelperEnv, when "1" in a spawned copy of the test binary, makes
// TestFakeResticCopyOutput act as a stand-in for a REAL restic copy process:
// it prints a canned transcript shaped exactly like restic 0.17.3's actual
// `copy` output (two snapshots, each with its own "copy started" header and
// pack-copy percentage lines) and exits 0. Mirrors restic_cancel_test.go's
// resticHelperEnv self-exec pattern, which lets this test point
// runStreamingCopy at a real (killable, cross-platform, no shell script
// needed) child process instead of a hand-rolled io.Reader — exercising the
// SAME stdout-pipe/bufio.Scanner code path Restic.Copy actually uses.
const copyHelperEnv = "BOMBVAULT_RESTIC_COPY_FAKE"

// TestFakeResticCopyOutput is not a real test; see copyHelperEnv. In a normal
// test run the env is unset and it returns immediately.
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

// TestRunStreamingCopyTracksSnapshotBoundaries pins the whole issue #159
// pipeline end to end: a real child process's plain-text restic-copy-shaped
// stdout (no --json — see Restic.Copy's doc comment) is scanned, "copy
// started" headers advance a 1-based snapshot index, and each pack-copy
// percentage line is reported against the CURRENT index — proving the second
// snapshot's counter restarting at a low percentage does not get misread as
// the first snapshot regressing, and that both snapshots' real percentages
// reach the sink in order.
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

// copyHelperNoBoundaryEnv, when "1" in a spawned copy of the test binary,
// makes TestFakeResticCopyOutputNoBoundary print a percentage line with NO
// preceding "copy started" header at all — simulating a future restic
// release that renames/removes that header (copyStartedRe stops matching).
const copyHelperNoBoundaryEnv = "BOMBVAULT_RESTIC_COPY_FAKE_NOBOUNDARY"

// TestFakeResticCopyOutputNoBoundary is not a real test; see
// copyHelperNoBoundaryEnv. In a normal test run the env is unset and it
// returns immediately.
func TestFakeResticCopyOutputNoBoundary(t *testing.T) {
	if os.Getenv(copyHelperNoBoundaryEnv) != "1" {
		return
	}
	for _, line := range []string{
		"", // deliberately no "copy started" header before the percentage line
		"[0:05] 42.00%  1 / 2 packs copied",
	} {
		os.Stdout.WriteString(line + "\n") //nolint:errcheck,gosec // test helper child process, best-effort write
	}
}

// TestRunStreamingCopyNoBoundaryStillReportsSnapshotOne is the defensive
// fallback: if a future restic version renames/removes the "copy started"
// header (copyStartedRe stops matching), a percentage line arriving before
// any recognized boundary must still be attributed to snapshot 1 rather than
// snapshot 0 — never silently dropped. This exercises the REAL
// runStreamingCopy (via a real child process's stdout pipe, same pattern as
// TestRunStreamingCopyTracksSnapshotBoundaries above) rather than a hand-
// reconstructed copy of its onLine closure: an earlier version of this test
// rebuilt that closure inline, so removing the `if snap == 0 { snap = 1 }`
// guard from the actual production code left it green.
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

// TestPendingCopyIDsEmptySrcIsEmpty guards against a nil-vs-empty slice
// surprise: no source snapshots means nothing pending, not every destination
// snapshot somehow reported back.
func TestPendingCopyIDsEmptySrcIsEmpty(t *testing.T) {
	dst := []Snapshot{{ID: "x"}}
	if got := PendingCopyIDs(nil, dst); len(got) != 0 {
		t.Fatalf("expected no pending ids for an empty source list, got %v", got)
	}
}
