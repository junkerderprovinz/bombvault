package api

import (
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestBucketRunsByDay(t *testing.T) {
	// Noon keeps the day boundaries clear in any timezone.
	now := time.Now().Local()
	mid := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	day0 := mid.AddDate(0, 0, -2)
	day1 := mid.AddDate(0, 0, -1)
	day2 := mid

	// "unknown" is left out of the map.
	domain := map[string]string{
		"c1":                 "container",
		"v1":                 "vm",
		"f1":                 "files",
		store.FlashTargetID:  "flash",
		store.ConfigTargetID: "config",
	}

	runs := []store.Run{
		{TargetID: "c1", Kind: "backup", Status: "success", StartedAt: day0.Unix()},
		{TargetID: "c1", Kind: "backup", Status: "failed", StartedAt: day0.Unix()},
		{TargetID: "v1", Kind: "backup", Status: "success", StartedAt: day1.Unix()},
		{TargetID: "f1", Kind: "backup", Status: "success", StartedAt: day2.Unix()},
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
	if got[0].Date != day0.Format("2006-01-02") || got[2].Date != day2.Format("2006-01-02") {
		t.Fatalf("dates not ascending/contiguous: %q .. %q", got[0].Date, got[2].Date)
	}

	if got[0].Containers != (DayStat{OK: 1, Failed: 1}) {
		t.Fatalf("day0 containers = %+v, want {1 1}", got[0].Containers)
	}
	if got[0].VMs != (DayStat{}) || got[0].Flash != (DayStat{}) {
		t.Fatalf("day0 vms/flash should be empty: %+v %+v", got[0].VMs, got[0].Flash)
	}

	if got[1].VMs != (DayStat{OK: 1}) {
		t.Fatalf("day1 vms = %+v, want {1 0}", got[1].VMs)
	}
	if got[1].Config != (DayStat{OK: 1}) {
		t.Fatalf("day1 config = %+v, want {1 0}", got[1].Config)
	}

	if got[2].Flash != (DayStat{OK: 1}) {
		t.Fatalf("day2 flash = %+v, want {1 0}", got[2].Flash)
	}
	if got[2].Files != (DayStat{OK: 1}) {
		t.Fatalf("day2 files = %+v, want {1 0}", got[2].Files)
	}
	if got[2].Containers != (DayStat{}) {
		t.Fatalf("day2 containers must ignore running/restore/unknown: %+v", got[2].Containers)
	}
}
