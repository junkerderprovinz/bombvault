//go:build !linux

package api

import "os"

// dirOwner reports no owner outside Linux, where the container runs: a
// recreated directory then keeps the one this process gives it.
func dirOwner(os.FileInfo) (uid, gid int, ok bool) {
	return 0, 0, false
}
