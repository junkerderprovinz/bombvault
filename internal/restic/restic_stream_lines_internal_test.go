package restic

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// streamLines has to fail when its scanner fails, even if restic exits 0.
// scanLines logs a scanner error and carries on, which suits a backup whose
// summary line must survive an over-long status line. A listing cannot do
// that: every dropped line is a node the caller never sees, and the clean exit
// would vouch for the short stream as complete.

// streamHelperEnv set to "1" makes a child copy of the test binary act as
// restic: TestFakeResticOverlongLine prints a line past the scanner's 16 MiB
// limit and exits 0, TestFakeResticShortLines prints three short lines.
const streamHelperEnv = "BOMBVAULT_RESTIC_STREAM_FAKE"

// overlongLineBytes is past the 16 MiB scanner limit in streamLines, so
// bufio.Scanner fails with ErrTooLong instead of growing its buffer.
const overlongLineBytes = 20 << 20

// TestFakeResticOverlongLine is a helper, not a test; see streamHelperEnv.
func TestFakeResticOverlongLine(t *testing.T) {
	if os.Getenv(streamHelperEnv) != "1" {
		return
	}
	// A readable line first shows the stream was read up to the failure.
	os.Stdout.WriteString(`{"struct_type":"node","path":"/a","type":"dir"}` + "\n") //nolint:errcheck,gosec // test helper child process, best-effort write
	os.Stdout.WriteString(strings.Repeat("x", overlongLineBytes) + "\n")            //nolint:errcheck,gosec // test helper child process, best-effort write
}

// TestStreamLinesFailsOnTruncatedRead checks that a scanner failure is returned
// even though the process exits cleanly. Lines read before it are still
// delivered; the caller discards them.
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

// TestFakeResticShortLines is a helper, not a test; see streamHelperEnv.
func TestFakeResticShortLines(t *testing.T) {
	if os.Getenv(streamHelperEnv) != "1" {
		return
	}
	for _, l := range []string{"one", "two", "three"} {
		os.Stdout.WriteString(l + "\n") //nolint:errcheck,gosec // test helper child process, best-effort write
	}
}
