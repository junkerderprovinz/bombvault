package restic

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// streamLines' all-or-nothing contract (issue #175).
//
// streamLines is scanLines without the accumulating buffer, and it was copied
// with scanLines' "a scanner error is logged, not fatal" behaviour intact. That
// is deliberate for a BACKUP, whose trailing summary line must survive an
// over-long status line in the middle of the run. It is wrong for a LISTING:
// every line dropped is a node the caller never sees, and if restic then exits
// 0 the caller is handed a silently short stream that the exit code certifies
// as complete. Every candidate built from it would carry complete:true.
//
// That is defect #175 on a different feeder, and against the guarantee stated
// in excludes_suggest.go's own header ("there is no code path that emits a
// partial snapshot list"), so the scan error has to win over the clean exit.
// ---------------------------------------------------------------------------

// streamHelperEnv, when "1" in a spawned copy of the test binary, makes
// TestFakeResticOverlongLine stand in for a real restic process that emits one
// line longer than the scanner's 16 MiB ceiling and THEN EXITS 0 — the exact
// combination that used to be reported as a successful, complete listing.
// Mirrors restic_copy_progress_internal_test.go's self-exec pattern.
const streamHelperEnv = "BOMBVAULT_RESTIC_STREAM_FAKE"

// overlongLineBytes is comfortably past the 16 MiB scanner ceiling in
// streamLines, so bufio.Scanner fails with ErrTooLong rather than merely
// growing its buffer.
const overlongLineBytes = 20 << 20

// TestFakeResticOverlongLine is not a real test; see streamHelperEnv. In a
// normal run the env is unset and it returns immediately.
func TestFakeResticOverlongLine(t *testing.T) {
	if os.Getenv(streamHelperEnv) != "1" {
		return
	}
	// One short, readable node line first, so the test can prove the stream was
	// consumed up to the failure and not rejected before it started.
	os.Stdout.WriteString(`{"struct_type":"node","path":"/a","type":"dir"}` + "\n") //nolint:errcheck,gosec // test helper child process, best-effort write
	os.Stdout.WriteString(strings.Repeat("x", overlongLineBytes) + "\n")            //nolint:errcheck,gosec // test helper child process, best-effort write
	// …and exits 0, which is the whole point: nothing else in the pipeline has
	// any reason to suspect the listing was cut short.
}

// TestStreamLinesFailsOnTruncatedRead: a scanner failure must NOT be swallowed
// into a nil error just because the process exited cleanly. The lines that were
// read are still delivered (the caller discards them under its own
// all-or-nothing rule), but the error says the read did not finish.
func TestStreamLinesFailsOnTruncatedRead(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestFakeResticOverlongLine$") //nolint:gosec // G204: os.Args[0] is this test binary itself, fixed args
	cmd.Env = append(os.Environ(), streamHelperEnv+"=1")

	var lines int
	err := streamLines(cmd, []string{"ls"}, func([]byte) { lines++ })
	if err == nil {
		t.Fatal("streamLines returned nil for a listing it could not finish reading; " +
			"the caller then aggregates a partial stream and reports every candidate as exact (#175)")
	}
	if !strings.Contains(err.Error(), "stdout scan") {
		t.Fatalf("err = %v, want it to name the scan failure", err)
	}
	if lines != 1 {
		t.Fatalf("delivered %d line(s) before failing, want the 1 readable node", lines)
	}
}

// TestStreamLinesSucceedsOnACleanStream is the guard that the change above did
// not simply make streamLines fail: an ordinary listing still returns nil and
// delivers every line.
func TestStreamLinesSucceedsOnACleanStream(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestFakeResticShortLines$") //nolint:gosec // G204: os.Args[0] is this test binary itself, fixed args
	cmd.Env = append(os.Environ(), streamHelperEnv+"=1")

	var got []string
	if err := streamLines(cmd, []string{"ls"}, func(b []byte) { got = append(got, string(b)) }); err != nil {
		t.Fatalf("streamLines: %v", err)
	}
	// The child is a test binary, so it writes its own "PASS" trailer after the
	// canned lines; only the leading ones are the fixture.
	want := []string{"one", "two", "three"}
	if len(got) < len(want) {
		t.Fatalf("got %d lines %q, want at least %q", len(got), got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("line %d = %q, want %q (all of %q)", i, got[i], w, got)
		}
	}
}

// TestFakeResticShortLines is not a real test; see streamHelperEnv.
func TestFakeResticShortLines(t *testing.T) {
	if os.Getenv(streamHelperEnv) != "1" {
		return
	}
	for _, l := range []string{"one", "two", "three"} {
		os.Stdout.WriteString(l + "\n") //nolint:errcheck,gosec // test helper child process, best-effort write
	}
}
