//go:build unix

package main

import (
	"os"
	"syscall"
	"testing"
	"time"
)

// `restic unlock` sends SIGHUP to the PID a lock names to see whether its
// owner lives, and in a restarted container that PID can be BombVault's own.
func TestIgnoreHangupSurvivesSIGHUP(t *testing.T) {
	ignoreHangup()
	if err := syscall.Kill(os.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
}
