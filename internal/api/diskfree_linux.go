//go:build linux

package api

import "syscall"

// diskFreeBytes returns the bytes available to unprivileged writers on the
// filesystem containing path (statfs f_bavail × f_bsize) — the honest "how
// much more backup fits here" number for the storage forecast. Linux-only:
// the shipped container is Linux, and other platforms compile the stub in
// diskfree_other.go (the forecast then simply omits freeBytes).
func diskFreeBytes(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	if st.Bsize < 0 {
		return 0, nil // defensive: a negative block size is nonsense, claim nothing
	}
	return st.Bavail * uint64(st.Bsize), nil
}
