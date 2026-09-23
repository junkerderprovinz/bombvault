//go:build !linux

package api

import "errors"

// diskFreeBytes is only implemented on Linux, where the container runs. On
// other platforms the storage forecast omits free space.
func diskFreeBytes(string) (uint64, error) {
	return 0, errors.New("free-space probe is only supported on Linux")
}
