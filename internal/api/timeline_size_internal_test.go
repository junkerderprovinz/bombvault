package api

import (
	"context"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// sized gives a snapshot the summary restic 0.17 and later record on it.
func sized(snap restic.Snapshot, bytes uint64) restic.Snapshot {
	snap.Summary = &restic.SnapshotSummary{TotalBytesProcessed: bytes}
	return snap
}

// measuredRun records a successful backup of an item that wrote snapshotID and
// read bytes.
func measuredRun(t *testing.T, st *store.Repo, itemID, snapshotID string, bytes int64) {
	t.Helper()
	id, err := st.StartRun(itemID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRunMeasured(id, "success", snapshotID, 0, "", &store.RunMetrics{SourceBytes: bytes}, ""); err != nil {
		t.Fatal(err)
	}
}

// sizeOf is the size a timeline shows for the row key, -1 when it shows none.
func sizeOf(t *testing.T, rows []timelineRow, key string) int64 {
	t.Helper()
	for _, row := range rows {
		if row.Key == key {
			if row.Size == nil {
				return -1
			}
			return *row.Size
		}
	}
	t.Fatalf("no row %s in %v", key, rowKeys(rows))
	return 0
}

func TestTimelineRowsCarryTheSizeOfTheirBackup(t *testing.T) {
	f := newPlacementFixture(t)
	nginx := f.container("nginx", "")
	nasLoc := f.localRepo("copies/containers")
	f.target("containers", "NAS copy", "copies/containers")
	f.hold(f.domainPath("containers"),
		snap("a1a1a1a1", 100, "container:nginx"),
		sized(snap("a2a2a2a2", 200, "container:nginx"), 300),
		sized(snap("a3a3a3a3", 300, "container:nginx"), 4999),
	)
	f.hold(nasLoc, sized(copied("c2c2c2c2", "a2a2a2a2", 200, "container:nginx"), 300))
	measuredRun(t, f.st, nginx.ID, "a3a3a3a3", 5000)

	tl, err := f.svc.Timeline(context.Background(), "containers", "nginx")
	if err != nil {
		t.Fatal(err)
	}
	if got := sizeOf(t, tl.Rows, "a3a3a3a3"); got != 5000 {
		t.Errorf("a measured backup shows %d, want what its run read", got)
	}
	if got := sizeOf(t, tl.Rows, "a2a2a2a2"); got != 300 {
		t.Errorf("a backup without a run shows %d, want what restic recorded on it", got)
	}
	if got := sizeOf(t, tl.Rows, "a1a1a1a1"); got != -1 {
		t.Errorf("a backup neither a run nor restic measured shows %d, want no size", got)
	}
}

func TestAVMRowAddsItsZvolDisksToItsSize(t *testing.T) {
	f := newPlacementFixture(t)
	f.zvolVM("truenas-vm")
	nasLoc := f.localRepo("copies/vms")
	nas := f.target("vms", "NAS copy", "copies/vms")
	run := sized(snap("a1a1a1a1", 100, "vm:truenas-vm", "p2", "vmrun:r1"), 100)
	disk := sized(snap("a2a2a2a2", 100, "vm:truenas-vm:zvol:sda", "p2", "vmrun:r1"), 900)
	f.hold(f.domainPath("vms"), run, disk)
	f.hold(nasLoc, sized(copied("c1c1c1c1", "a1a1a1a1", 100, "vm:truenas-vm", "p2", "vmrun:r1"), 100))
	ctx := context.Background()

	tl, err := f.svc.Timeline(ctx, "vms", "truenas-vm")
	if err != nil {
		t.Fatal(err)
	}
	if got := sizeOf(t, tl.Rows, "a1a1a1a1"); got != 1000 {
		t.Fatalf("size = %d, want the run and its disk", got)
	}
	_, rows, err := f.svc.TimelinePlace(ctx, "vms", "truenas-vm", "offsite:"+nas.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := sizeOf(t, rows, "a1a1a1a1"); got != -1 {
		t.Fatalf("a place without the disk shows %d, want no size", got)
	}
}

func TestAZFSRowSumsItsDatasets(t *testing.T) {
	f := newPlacementFixture(t)
	tank, home := f.zfsItem("tank/apps")
	f.hold(home,
		sized(zfsSnap("a1a1a1a1", "tank/apps", firstRun), 4000),
		sized(zfsSnap("a2a2a2a2", "tank/apps/db", firstRun), 900),
		sized(zfsSnap("b1b1b1b1", "tank/apps", secondRun), 4100),
		zfsSnap("b2b2b2b2", "tank/apps/db", secondRun),
	)
	tl, err := f.svc.Timeline(context.Background(), zfsDomain, tank.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := sizeOf(t, tl.Rows, firstRun); got != 4900 {
		t.Errorf("first run = %d, want the sum of both datasets", got)
	}
	if got := sizeOf(t, tl.Rows, secondRun); got != -1 {
		t.Errorf("a run with an unmeasured dataset shows %d, want no size", got)
	}
}

func TestTheSizeOfARealBackupSurvivesATag(t *testing.T) {
	f := newResticFixture(t)
	f.write("boot", map[string]string{"bzimage": "0123456789", "config/go": "abcde"})
	snap := f.backup("flash", "boot")
	ctx := context.Background()

	tl, err := f.svc.Timeline(ctx, "flash", "flash")
	if err != nil {
		t.Fatal(err)
	}
	if got := sizeOf(t, tl.Rows, snap.ID); got != 15 {
		t.Fatalf("size = %d, want the 15 bytes of the two files", got)
	}

	measuredRun(t, f.st, store.FlashTargetID, snap.ID, 16)
	if err := f.svc.TagBackup(ctx, "flash", "flash", "local", snap.ID, []string{"keep"}, nil); err != nil {
		t.Fatal(err)
	}
	if tl, err = f.svc.Timeline(ctx, "flash", "flash"); err != nil {
		t.Fatal(err)
	}
	if got := sizeOf(t, tl.Rows, snap.ID); got != 16 || tl.Rows[0].Places[0].SnapshotIDs[0] == snap.ID {
		t.Fatalf("after a tag the row shows %d under %v, want what the run read under a new snapshot id", got, tl.Rows[0].Places[0].SnapshotIDs)
	}
}
