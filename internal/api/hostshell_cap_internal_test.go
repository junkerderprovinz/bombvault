package api

// The hook runner keeps a bounded amount of output and never lets a chatty
// command decide how much memory it may use.

import (
	"strings"
	"testing"
)

func TestCappedBufferKeepsHeadAndCountsTheRest(t *testing.T) {
	var b cappedBuffer
	b.limit = 10

	// Reports every byte as written even past the cap: a short write would make
	// exec close the pipe and hand the hook an EPIPE, turning a chatty command
	// into a failed one.
	n, err := b.Write([]byte("0123456789abcdef"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != 16 {
		t.Fatalf("Write reported %d bytes, want 16 — a short write would EPIPE the hook", n)
	}

	got := b.String()
	if !strings.HasPrefix(got, "0123456789") {
		t.Fatalf("kept %q, want it to start with the first 10 bytes", got)
	}
	if !strings.Contains(got, "6 more bytes dropped") {
		t.Fatalf("kept %q, want it to say how much was dropped", got)
	}
}

func TestCappedBufferUnderTheCapIsVerbatim(t *testing.T) {
	var b cappedBuffer
	b.limit = 1024
	for _, part := range []string{"hook ", "said ", "something"} {
		if _, err := b.Write([]byte(part)); err != nil {
			t.Fatal(err)
		}
	}
	if got := b.String(); got != "hook said something" {
		t.Fatalf("String() = %q, want the output unchanged and no dropped-bytes note", got)
	}
}

func TestHostShellOutputCapMatchesTheSiblingPrimitive(t *testing.T) {
	// dockercli.go caps the per-container hook at 64 KiB with the comment "a
	// hook flooding stdout cannot balloon memory"; this file names that path as
	// its model, so the two must not drift.
	if hostShellOutputCap != 64<<10 {
		t.Fatalf("hostShellOutputCap = %d, want 64 KiB to match dockercli.go", hostShellOutputCap)
	}
}
