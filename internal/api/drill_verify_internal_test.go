package api

import "testing"

// The verdict compares the restored sandbox itself with restic: files on disk
// against `ls`, bytes on disk against `stats --mode restore-size`. The file
// count of restore-size can differ from ls and must not fail a perfect restore.
func TestDrillVerifyOK(t *testing.T) {
	const bytes = int64(2545593264)
	cases := []struct {
		name                 string
		lsFiles, gotFiles    int
		statsBytes, gotBytes int64
		want                 bool
	}{
		{
			// Numbers from an Unraid flash restore whose restore-size file count
			// differed from ls.
			name:    "exact match passes",
			lsFiles: 988, gotFiles: 988, statsBytes: bytes, gotBytes: bytes, want: true,
		},
		{
			name:    "byte diff within the metadata floor passes",
			lsFiles: 988, gotFiles: 988, statsBytes: bytes, gotBytes: bytes + drillByteToleranceFloor, want: true,
		},
		{
			name:    "truncated restore (fewer files on disk) fails",
			lsFiles: 988, gotFiles: 980, statsBytes: bytes, gotBytes: bytes, want: false,
		},
		{
			name:    "byte diff beyond the floor fails",
			lsFiles: 988, gotFiles: 988, statsBytes: bytes, gotBytes: bytes + drillByteToleranceFloor + 1, want: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := drillVerifyOK(c.lsFiles, c.gotFiles, c.statsBytes, c.gotBytes); got != c.want {
				t.Fatalf("drillVerifyOK(ls=%d got=%d statsB=%d gotB=%d) = %v, want %v",
					c.lsFiles, c.gotFiles, c.statsBytes, c.gotBytes, got, c.want)
			}
		})
	}
}
