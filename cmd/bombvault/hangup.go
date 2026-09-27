package main

import (
	"os/signal"
	"syscall"
)

// ignoreHangup keeps BombVault running on SIGHUP. It has no terminal to lose,
// and `restic unlock` sends SIGHUP to the PID a lock names to see whether its
// owner lives, which after a restart can be BombVault's own PID.
func ignoreHangup() { signal.Ignore(syscall.SIGHUP) }
