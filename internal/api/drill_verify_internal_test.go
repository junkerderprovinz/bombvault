package api

import "testing"

// TestDrillVerifyOK pins the restorability verdict to the ACTUAL restored sandbox
// (on-disk file count vs restic ls, and on-disk bytes vs restic restore-size),
// and specifically guards against the #30 regression where restic's
// `stats --mode restore-size` file count differing from `ls` wrongly failed a
// perfect restore.
func TestDrillVerifyOK(t *testing.T) {
	const bytes = int64(2545593264)
	cases := []struct {
		name                 string
		lsFiles, gotFiles    int
		statsBytes, gotBytes int64
		want                 bool
	}{
		{
			// #30 (manilx): the Unraid flash restored the exact file count and bytes,
			// yet was flagged because restic stats' file count != ls'. Must pass now.
			name:    "exact match passes (statsFiles no longer consulted)",
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
