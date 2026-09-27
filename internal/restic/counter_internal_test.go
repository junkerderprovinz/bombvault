package restic

import "testing"

func TestParseCounterReadsResticsCounterLines(t *testing.T) {
	cases := map[string]struct {
		done, total int64
		unit        string
	}{
		"[0:01] 51.06%  24 / 47 packs":              {24, 47, "packs"},
		"[0:00] 100.00%  1 / 1 snapshots":           {1, 1, "snapshots"},
		"[1:02:03] 12.50%  1 / 8 packs repacked":    {1, 8, "packs"},
		"[0:00] 100.00%  3 / 3 old indexes deleted": {3, 3, "indexes"},
		"[0:00] 100.00%  14 / 14 files deleted":     {14, 14, "files"},
		"[0:00] 50.00%  1 / 2 blobs":                {1, 2, "items"},
	}
	for line, want := range cases {
		got, ok := parseCounter([]byte(line))
		if !ok || got.Done != want.done || got.Total != want.total || got.Unit != want.unit {
			t.Errorf("%q = %+v %v, want %+v", line, got, ok, want)
		}
	}
	for _, line := range []string{"read all data", "no errors were found", "to repack: 16 blobs / 17.573 MiB", "[0:00] 100.00%  0 / 0 packs"} {
		if _, ok := parseCounter([]byte(line)); ok {
			t.Errorf("%q must not read as a counter", line)
		}
	}
}
