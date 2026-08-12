//go:build !windows

package restic

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestConfigureProcGroup_KillsOnCancel is a best-effort regression check for
// #92/#97: cancelling ctx must reap the child (and its process group) promptly
// instead of leaving cmd.Wait blocked. It relies on the "sleep" binary being on
// PATH, which holds for the Linux CI runners this package ships on; it skips
// itself elsewhere rather than flaking CI.
func TestConfigureProcGroup_KillsOnCancel(t *testing.T) {
	sleepBin, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep binary not found on PATH, skipping")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, sleepBin, "30") //nolint:gosec // G204: sleepBin is the fixed "sleep" binary resolved via exec.LookPath, no user input
	configureProcGroup(cmd)

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	cancel()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-done:
		// Expected: killed promptly instead of running the full 30s sleep.
	case <-time.After(5 * time.Second):
		t.Fatal("cmd.Wait did not return within 5s of ctx cancel; child not reaped")
	}
}

// TestConfigureProcGroup_SendsSIGTERMNotSIGKILL guards the actual fix: Cancel
// must send SIGTERM (which restic treats as a clean-abort request — stop,
// don't write the snapshot, exit) not SIGKILL (which skips that handler
// entirely and, hit at the wrong moment, can leave a snapshot whose tree
// already references a blob that never finished uploading — root-caused live
// 2026-08-12 against a real damaged repo, see the doc comment above
// configureProcGroup). SIGKILL can never be caught by any process, so the
// distinguishing test is whether a trap handler gets to run at all: a shell
// that traps TERM and writes a marker file proves SIGTERM arrived; if SIGKILL
// were sent instead, the process dies before the trap (or any of its own
// code) can execute, and the marker is never written.
func TestConfigureProcGroup_SendsSIGTERMNotSIGKILL(t *testing.T) {
	shBin, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not found on PATH, skipping")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "caught-term")
	ready := filepath.Join(dir, "trap-installed")

	ctx, cancel := context.WithCancel(context.Background())
	// trap writes the marker and exits 0 on TERM; without a trap firing, the
	// marker is never created (SIGKILL leaves no chance to run this script at all).
	// Touching `ready` right after installing the trap, and polling for it
	// below before cancelling, closes a real race: cmd.Start() only proves the
	// fork succeeded, not that the shell has reached the `trap` line yet — a
	// SIGTERM arriving before that point hits the shell's default (untrapped)
	// disposition and looks identical to this test as a failed fix.
	// "; true" after sleep matters too: some /bin/sh implementations (e.g.
	// BusyBox ash) exec()-replace their own process image for a script's FINAL
	// simple command instead of forking a child, which would wipe out the trap.
	// Giving the shell something to do after sleep forces a real fork, keeping
	// the shell (and its trap) alive as the process that receives the SIGTERM.
	script := "trap 'touch " + marker + "; exit 0' TERM; touch " + ready + "; sleep 30; true"
	cmd := exec.CommandContext(ctx, shBin, "-c", script) //nolint:gosec // G204: fixed script, no user input
	configureProcGroup(cmd)

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitUntil := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(waitUntil) {
			t.Fatal("shell never reached the trap line (ready marker missing) within 2s")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cmd.Wait did not return within 5s of ctx cancel")
	}

	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("TERM trap did not run (marker file missing) - Cancel is not sending a catchable SIGTERM: %v", err)
	}
}
