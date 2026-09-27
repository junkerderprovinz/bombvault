//go:build linux

package api

import (
	"os"
	"syscall"
)

// dirOwner returns the numeric owner of a directory, so a directory recreated
// beside it can be handed to the same user and group.
func dirOwner(info os.FileInfo) (uid, gid int, ok bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return int(st.Uid), int(st.Gid), true
}
