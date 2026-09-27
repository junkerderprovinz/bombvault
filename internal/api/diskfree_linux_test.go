package api

import (
	"syscall"
	"testing"
)

// ext4 keeps 5% of its blocks for root by default. Those blocks are neither
// free to an unprivileged writer nor used by anything.
func TestStatfsRoomLeavesReservedBlocksOutOfUsed(t *testing.T) {
	fs := syscall.Statfs_t{Bsize: 4096, Blocks: 1000, Bfree: 400, Bavail: 350}
	free, used, total := statfsRoom(&fs)
	if free != 350*4096 || used != 600*4096 || total != 1000*4096 {
		t.Fatalf("free, used, total = %d, %d, %d; want %d, %d, %d", free, used, total, 350*4096, 600*4096, 1000*4096)
	}
}
