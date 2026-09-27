//go:build linux

package api

import (
	"bytes"
	"fmt"
	"os"
	"syscall"
)

// diskStatResult is how much room a filesystem has and which volume it shares
// that room with.
type diskStatResult struct {
	// Free is what an unprivileged writer can still use. Used leaves out the
	// blocks reserved for root, so Used and Free need not add up to Total.
	Free, Used, Total uint64
	// Volume groups the repositories that draw on the same free space:
	// "dev:<device in hex>", or "pool:<name>" for a dataset of a ZFS pool.
	Volume string
	// FSType is the type of the mount holding the path, such as cifs or nfs4.
	FSType string
}

// diskStat measures the filesystem containing path and names its volume.
func diskStat(path string) (diskStatResult, error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return diskStatResult{}, err
	}
	var res diskStatResult
	res.Free, res.Used, res.Total = statfsRoom(&fs)
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return diskStatResult{}, err
	}
	res.Volume = fmt.Sprintf("dev:%x", st.Dev)
	if mounts, err := os.ReadFile(mountinfoPath); err == nil {
		if pool, ok := zfsPoolAt(bytes.NewReader(mounts), path); ok {
			res.Volume = "pool:" + pool
		}
		res.FSType = fsTypeAt(bytes.NewReader(mounts), path)
	}
	return res, nil
}

// diskFreeBytes returns the bytes available to unprivileged writers on the
// filesystem containing path (f_bavail * f_bsize).
func diskFreeBytes(path string) (uint64, error) {
	res, err := diskStat(path)
	return res.Free, err
}

// statfsRoom turns the block counts of a statfs answer into bytes.
func statfsRoom(fs *syscall.Statfs_t) (free, used, total uint64) {
	if fs.Bsize <= 0 {
		return 0, 0, 0
	}
	size := uint64(fs.Bsize)
	return fs.Bavail * size, (fs.Blocks - fs.Bfree) * size, fs.Blocks * size
}
