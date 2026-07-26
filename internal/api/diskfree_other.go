//go:build !linux

package api

import "errors"

// diskFreeBytes is unsupported off Linux — the shipped container is Linux-only
// and the storage forecast then simply omits freeBytes/weeksToFull. Kept as a
// stub (not a build failure) so the project still builds and tests on
// developer machines (Windows/macOS); tests exercise the seam via the injected
// Service.diskFree fake instead.
func diskFreeBytes(string) (uint64, error) {
	return 0, errors.New("free-space probe is only supported on Linux")
}
