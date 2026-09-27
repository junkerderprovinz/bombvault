//go:build !linux

package api

import "errors"

// errNoDiskStat is what every free-space probe reports off Linux, where the
// container runs. The storage forecast omits free space and the capacity rule
// takes no samples.
var errNoDiskStat = errors.New("free-space probe is only supported on Linux")

// diskStatResult is how much room a filesystem has and which volume it shares
// that room with.
type diskStatResult struct {
	Free, Total uint64
	Volume      string
}

func diskStat(string) (diskStatResult, error) {
	return diskStatResult{}, errNoDiskStat
}

func diskFreeBytes(string) (uint64, error) {
	return 0, errNoDiskStat
}
