package api

import (
	"os"
	"path/filepath"
	"testing"
)

// The verdict compares the restored sandbox itself with restic ls: files and
// bytes on disk against the file nodes it lists.
func TestDrillVerifyOK(t *testing.T) {
	const bytes = int64(2545593264)
	cases := []struct {
		name              string
		lsFiles, gotFiles int
		lsBytes, gotBytes int64
		want              bool
	}{
		{
			// Numbers from an Unraid flash restore.
			name:    "exact match passes",
			lsFiles: 988, gotFiles: 988, lsBytes: bytes, gotBytes: bytes, want: true,
		},
		{
			name:    "byte diff within the metadata floor passes",
			lsFiles: 988, gotFiles: 988, lsBytes: bytes, gotBytes: bytes + drillByteToleranceFloor, want: true,
		},
		{
			name:    "truncated restore (fewer files on disk) fails",
			lsFiles: 988, gotFiles: 980, lsBytes: bytes, gotBytes: bytes, want: false,
		},
		{
			name:    "byte diff beyond the floor fails",
			lsFiles: 988, gotFiles: 988, lsBytes: bytes, gotBytes: bytes + drillByteToleranceFloor + 1, want: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := drillVerifyOK(c.lsFiles, c.gotFiles, c.lsBytes, c.gotBytes); got != c.want {
				t.Fatalf("drillVerifyOK(ls=%d got=%d lsB=%d gotB=%d) = %v, want %v",
					c.lsFiles, c.gotFiles, c.lsBytes, c.gotBytes, got, c.want)
			}
		})
	}
}

// A hardlinked file counts once per name, the way restic ls lists it.
func TestWalkDrillSandboxCountsEveryNameOfAHardlink(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "a.bin")
	if err := os.WriteFile(first, make([]byte, 1000), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(first, filepath.Join(dir, "b.bin")); err != nil {
		t.Skipf("no hardlinks here: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, drillMarkerName), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	files, bytes, err := walkDrillSandbox(dir)
	if err != nil || files != 2 || bytes != 2000 {
		t.Fatalf("walk = %d files / %d bytes, %v; want 2 / 2000", files, bytes, err)
	}
}
