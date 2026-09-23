//go:build linux

package api

import "syscall"

// diskFreeBytes returns the bytes available to unprivileged writers on the
// filesystem containing path (f_bavail * f_bsize).
func diskFreeBytes(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	if st.Bsize < 0 {
		return 0, nil // keeps the uint64 conversion below sound
	}
	return st.Bavail * uint64(st.Bsize), nil
}
