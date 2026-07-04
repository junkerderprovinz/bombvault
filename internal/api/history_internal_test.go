package api

import (
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestBucketRunsByDay covers the pure heatmap-bucketing core: a contiguous grid
// of every local day in the window (zeros for empty days), success/failed
// tallied into the right domain, and "running"/non-backup/unknown-target runs
// ignored.
func TestBucketRunsByDay(t *testing.T) {
	// A 3-day window ending today (local), anchored at noon so day boundaries are
	// unambiguous regardless of the test machine's timezone.
	now := time.Now().Local()
	mid := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	day0 := mid.AddDate(0, 0, -2) // earliest day
	day1 := mid.AddDate(0, 0, -1)
	day2 := mid // today

	// "unknown" is intentionally absent from the map → it resolves to "".
	domain := map[string]string{
		"c1":                 "container",
		"v1":                 "vm",
		store.FlashTargetID:  "flash",
		store.ConfigTargetID: "config",
	}

	runs := []store.Run{
		{TargetID: "c1", Kind: "backup", Status: "success", StartedAt: day0.Unix()},
		{TargetID: "c1", Kind: "backup", Status: "failed", StartedAt: day0.Unix()},
		{TargetID: "v1", Kind: "backup", Status: "success", StartedAt: day1.Unix()},
		{TargetID: store.FlashTargetID, Kind: "backup", Status: "success", StartedAt: day2.Unix()},
		{TargetID: store.ConfigTargetID, Kind: "backup", Status: "success", StartedAt: day1.Unix()},
		// Ignored: running status, non-backup kind, unknown target.
		{TargetID: "c1", Kind: "backup", Status: "running", StartedAt: day2.Unix()},
		{TargetID: "c1", Kind: "restore", Status: "success", StartedAt: day2.Unix()},
		{TargetID: "unknown", Kind: "backup", Status: "success", StartedAt: day2.Unix()},
	}

	got := bucketRunsByDay(runs, domain, day0.Unix(), day2.Unix())

	if len(got) != 3 {
		t.Fatalf("expected 3 contiguous days, got %d", len(got))
	}
	// Ascending dates.
	if got[0].Date != day0.Format("2006-01-02") || got[2].Date != day2.Format("2006-01-02") {
		t.Fatalf("dates not ascending/contiguous: %q .. %q", got[0].Date, got[2].Date)
	}

	// Day 0: one container ok + one container failed; nothing else.
	if got[0].Containers != (DayStat{OK: 1, Failed: 1}) {
		t.Fatalf("day0 containers = %+v, want {1 1}", got[0].Containers)
	}
	if got[0].VMs != (DayStat{}) || got[0].Flash != (DayStat{}) {
		t.Fatalf("day0 vms/flash should be empty: %+v %+v", got[0].VMs, got[0].Flash)
	}

	// Day 1: one VM ok and one config ok (config must land in its own bucket,
	// not be dropped by the switch default).
	if got[1].VMs != (DayStat{OK: 1}) {
		t.Fatalf("day1 vms = %+v, want {1 0}", got[1].VMs)
	}
	if got[1].Config != (DayStat{OK: 1}) {
		t.Fatalf("day1 config = %+v, want {1 0}", got[1].Config)
	}

	// Day 2: one flash ok; running/restore/unknown all ignored.
	if got[2].Flash != (DayStat{OK: 1}) {
		t.Fatalf("day2 flash = %+v, want {1 0}", got[2].Flash)
	}
	if got[2].Containers != (DayStat{}) {
		t.Fatalf("day2 containers must ignore running/restore/unknown: %+v", got[2].Containers)
	}
}
