//go:build linux

package api

import (
	"fmt"
	"os"
	"syscall"
)

// diskStatResult is how much room a filesystem has and which volume it shares
// that room with.
type diskStatResult struct {
	Free, Total uint64
	// Volume groups the repositories that draw on the same free space:
	// "dev:<device in hex>", or "pool:<name>" for a dataset of a ZFS pool.
	Volume string
}

// diskStat measures the filesystem containing path and names its volume.
func diskStat(path string) (diskStatResult, error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return diskStatResult{}, err
	}
	var res diskStatResult
	if fs.Bsize > 0 {
		res.Free = fs.Bavail * uint64(fs.Bsize)
		res.Total = fs.Blocks * uint64(fs.Bsize)
	}
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return diskStatResult{}, err
	}
	res.Volume = fmt.Sprintf("dev:%x", st.Dev)
	if mounts, err := os.Open(mountinfoPath); err == nil { //nolint:gosec // G304: mountinfoPath is a fixed package var, overridden only by tests
		defer mounts.Close() //nolint:errcheck // read-only
		if pool, ok := zfsPoolAt(mounts, path); ok {
			res.Volume = "pool:" + pool
		}
	}
	return res, nil
}

// diskFreeBytes returns the bytes available to unprivileged writers on the
// filesystem containing path (f_bavail * f_bsize).
func diskFreeBytes(path string) (uint64, error) {
	res, err := diskStat(path)
	return res.Free, err
}
