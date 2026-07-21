//go:build !windows

package restic

import (
	"context"
	"os/exec"
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
