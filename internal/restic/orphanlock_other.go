//go:build !linux

package restic

// ownerMayLive has no process table to read here, so every lock stays with
// restic's own stale test.
func ownerMayLive(int) bool { return true }
