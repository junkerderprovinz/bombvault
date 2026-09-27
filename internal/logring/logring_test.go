package logring

import (
	"strings"
	"testing"
)

// TestRingKeepsTheNewestWithinItsCap pins what the ring is for: it holds
// a bounded amount of the MOST RECENT log output. A support bundle wants the
// lines just before something went wrong, and a buffer that filled up and then
// stopped recording would hold exactly the wrong end of the run.
func TestRingKeepsTheNewestWithinItsCap(t *testing.T) {
	// 16 bytes against 31 bytes of input, so the ring genuinely has to wrap.
	// An earlier version of this test used a cap the input fitted inside and
	// proved nothing at all.
	const cap = 16
	r := New(cap)
	for _, s := range []string{"alpha\n", "bravo\n", "charlie\n", "delta\n", "echo\n"} {
		if _, err := r.Write([]byte(s)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	got := r.String()
	if len(got) > cap {
		t.Fatalf("ring grew past its cap: %d bytes", len(got))
	}
	if !strings.Contains(got, "echo") {
		t.Fatalf("the newest line was dropped: %q", got)
	}
	if strings.Contains(got, "alpha") {
		t.Fatalf("the oldest line survived past the cap: %q", got)
	}
}

// TestRingHandlesAWriteLargerThanItself: a single oversized write must leave the
// ring holding that write's TAIL rather than panicking or silently keeping
// nothing. A stack trace is exactly this case.
func TestRingHandlesAWriteLargerThanItself(t *testing.T) {
	r := New(16)
	big := strings.Repeat("x", 40) + "END"
	if _, err := r.Write([]byte(big)); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := r.String()
	if len(got) > 16 {
		t.Fatalf("ring grew past its cap: %d bytes", len(got))
	}
	if !strings.HasSuffix(got, "END") {
		t.Fatalf("the tail of an oversized write was lost: %q", got)
	}
}

// TestRingIsEmptyBeforeAnythingIsWritten: a bundle taken from a freshly started
// instance must produce an empty string, not a panic and not stale bytes.
func TestRingIsEmptyBeforeAnythingIsWritten(t *testing.T) {
	if got := New(64).String(); got != "" {
		t.Fatalf("a fresh ring must be empty, got %q", got)
	}
}

// TestRingTeesToTheUnderlyingWriter pins that installing the ring does not cost
// the console its output: BombVault logs to stdout and that is what `docker
// logs` shows, so the ring has to be a tee, never a replacement.
func TestRingTeesToTheUnderlyingWriter(t *testing.T) {
	var sink strings.Builder
	r := New(64)
	w := r.Tee(&sink)

	if _, err := w.Write([]byte("hello\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if sink.String() != "hello\n" {
		t.Fatalf("the underlying writer lost output: %q", sink.String())
	}
	if !strings.Contains(r.String(), "hello") {
		t.Fatalf("the ring recorded nothing: %q", r.String())
	}
}
