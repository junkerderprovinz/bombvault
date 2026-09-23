//go:build !windows

package restic

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

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
	case <-time.After(5 * time.Second):
		t.Fatal("cmd.Wait did not return within 5s of ctx cancel; child not reaped")
	}
}

// TestConfigureProcGroup_SendsSIGTERMNotSIGKILL checks that cancelling sends
// SIGTERM to the whole process group. A leader shell and a child shell each
// trap TERM and write a marker, so SIGKILL leaves no marker at all and a signal
// to the leader alone leaves the child's missing.
func TestConfigureProcGroup_SendsSIGTERMNotSIGKILL(t *testing.T) {
	shBin, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not found on PATH, skipping")
	}
	dir := t.TempDir()
	leaderMarker := filepath.Join(dir, "leader-caught-term")
	childMarker := filepath.Join(dir, "child-caught-term")
	ready := filepath.Join(dir, "traps-installed")

	ctx, cancel := context.WithCancel(context.Background())
	// cmd.Start does not mean the traps are installed yet, and a SIGTERM before
	// that would look like a SIGKILL; the ready file closes that race. A shell
	// runs its trap only once the foreground command finishes, and a group kill
	// that lands just before the next sleep is forked misses that sleep, so both
	// shells wait in short sleeps instead of one long one.
	child := "trap 'touch " + childMarker + "; exit 0' TERM; touch " + ready + "; while :; do sleep 0.05; done"
	script := "trap 'touch " + leaderMarker + "; exit 0' TERM; " + shBin + " -c \"" + child + "\" & while :; do sleep 0.05; done"
	cmd := exec.CommandContext(ctx, shBin, "-c", script) //nolint:gosec // G204: fixed script, no user input
	configureProcGroup(cmd)

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	// The loops never end on their own, so whatever a failed run leaves
	// behind goes with the test.
	t.Cleanup(func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) })

	waitFor := func(path, what string, limit time.Duration) {
		t.Helper()
		deadline := time.Now().Add(limit)
		for {
			if _, err := os.Stat(path); err == nil {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s (%s missing after %v)", what, filepath.Base(path), limit)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	waitFor(ready, "the shells never installed their traps", 2*time.Second)
	cancel()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cmd.Wait did not return within 5s of ctx cancel")
	}

	if _, err := os.Stat(leaderMarker); err != nil {
		t.Fatalf("TERM trap did not run in the leader - Cancel is not sending a catchable SIGTERM: %v", err)
	}
	waitFor(childMarker, "TERM trap did not run in the child - Cancel is not signalling the whole process group", 2*time.Second)
}
