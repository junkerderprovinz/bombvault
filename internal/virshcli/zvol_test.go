package virshcli

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// zvolDatasetFromDevPath
// ---------------------------------------------------------------------------

// TestZvolDatasetFromDevPath pins the ONLY convention this parser understands
// (/dev/zvol/<pool>/<dataset...>) and — just as important — pins that it NEVER
// guesses: anything that doesn't match exactly returns ok=false rather than an
// invented dataset name. This is a pure function; no real ZFS/TrueNAS system
// is touched by this test. See zvol.go's package-level doc comment for the
// "reasoned from documentation, not verified against real hardware" caveat
// this whole mechanism carries (Task 10, v8.0.0 TrueNAS platform expansion).
func TestZvolDatasetFromDevPath(t *testing.T) {
	cases := []struct {
		name    string
		devPath string
		want    string
		wantOK  bool
	}{
		{"simple pool/dataset", "/dev/zvol/tank/vm-disk1", "tank/vm-disk1", true},
		{"nested dataset path", "/dev/zvol/tank/vms/win10/disk0", "tank/vms/win10/disk0", true},
		{"trailing slash tolerated", "/dev/zvol/tank/vm-disk1/", "tank/vm-disk1", true},

		{"empty string", "", "", false},
		{"file-backed qcow2 path", "/mnt/cache/vms/Win/vdisk1.img", "", false},
		{"other block device, not a zvol", "/dev/sda1", "", false},
		{"zvol prefix but no dataset segment (pool only)", "/dev/zvol/tank", "", false},
		{"zvol prefix, empty remainder", "/dev/zvol/", "", false},
		{"zvol prefix, only slashes", "/dev/zvol///", "", false},
		{"path traversal attempt", "/dev/zvol/tank/../etc/passwd", "", false},
		{"embedded whitespace", "/dev/zvol/tank/my disk", "", false},
		{"embedded quote", "/dev/zvol/tank/disk'name", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ZvolDatasetFromDevPath(c.devPath)
			if ok != c.wantOK {
				t.Fatalf("ZvolDatasetFromDevPath(%q) ok = %v, want %v (got dataset %q)", c.devPath, ok, c.wantOK, got)
			}
			if ok && got != c.want {
				t.Fatalf("ZvolDatasetFromDevPath(%q) = %q, want %q", c.devPath, got, c.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ZFS argv builders — pure, unit-testable without a real ZFS system.
// ---------------------------------------------------------------------------

func TestZFSSnapshotArgs(t *testing.T) {
	got := ZFSSnapshotArgs("tank/vm-disk1", "bombvault-20260816120000")
	want := []string{"zfs", "snapshot", "tank/vm-disk1@bombvault-20260816120000"}
	if !equalStrings(got, want) {
		t.Fatalf("ZFSSnapshotArgs = %v, want %v", got, want)
	}
}

func TestZFSSnapshotDestroyArgs(t *testing.T) {
	got := ZFSSnapshotDestroyArgs("tank/vm-disk1", "bombvault-20260816120000")
	want := []string{"zfs", "destroy", "tank/vm-disk1@bombvault-20260816120000"}
	if !equalStrings(got, want) {
		t.Fatalf("ZFSSnapshotDestroyArgs = %v, want %v", got, want)
	}
}

func TestZFSSendArgs(t *testing.T) {
	got := ZFSSendArgs("tank/vm-disk1", "bombvault-20260816120000")
	want := []string{"zfs", "send", "tank/vm-disk1@bombvault-20260816120000"}
	if !equalStrings(got, want) {
		t.Fatalf("ZFSSendArgs = %v, want %v", got, want)
	}
}

func TestZFSReceiveArgs(t *testing.T) {
	got := ZFSReceiveArgs("tank/vm-disk1-bombvault-restore-1755345600000000000")
	want := []string{"zfs", "receive", "tank/vm-disk1-bombvault-restore-1755345600000000000"}
	if !equalStrings(got, want) {
		t.Fatalf("ZFSReceiveArgs = %v, want %v", got, want)
	}
}

// ---------------------------------------------------------------------------
// RestoreZvolTargetDataset — the restore-side safety property.
// ---------------------------------------------------------------------------

// TestRestoreZvolTargetDatasetNeverEqualsSource is the core safety test for
// Task 10's restore path: `zfs receive` into an EXISTING dataset can destroy
// live data, so the generated target dataset must be structurally guaranteed
// distinct from the source — never a value the caller could mistake for (or
// that could collide with) the original live dataset.
func TestRestoreZvolTargetDatasetNeverEqualsSource(t *testing.T) {
	sources := []string{"tank/vm-disk1", "tank/vms/win10/disk0", "pool2/data"}
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	for _, src := range sources {
		got := RestoreZvolTargetDataset(src, now)
		if got == src {
			t.Fatalf("RestoreZvolTargetDataset(%q) = %q, MUST NEVER equal the source dataset", src, got)
		}
		if !strings.HasPrefix(got, src+"-bombvault-restore-") {
			t.Fatalf("RestoreZvolTargetDataset(%q) = %q, want prefix %q", src, got, src+"-bombvault-restore-")
		}
	}
}

// TestRestoreZvolTargetDatasetDistinctAcrossTime confirms two restores of the
// same source dataset at different times land on different fresh datasets
// (never silently collide/overwrite an earlier restore attempt either).
func TestRestoreZvolTargetDatasetDistinctAcrossTime(t *testing.T) {
	src := "tank/vm-disk1"
	t1 := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 16, 12, 0, 1, 0, time.UTC)
	a := RestoreZvolTargetDataset(src, t1)
	b := RestoreZvolTargetDataset(src, t2)
	if a == b {
		t.Fatalf("RestoreZvolTargetDataset produced the same target %q for two different times", a)
	}
}

// ---------------------------------------------------------------------------
// ZvolSnapshotName
// ---------------------------------------------------------------------------

func TestZvolSnapshotNameIsDeterministicPerInstant(t *testing.T) {
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	got := ZvolSnapshotName(now)
	if !strings.HasPrefix(got, "bombvault-") {
		t.Fatalf("ZvolSnapshotName = %q, want a bombvault- prefixed name", got)
	}
	if ZvolSnapshotName(now) != got {
		t.Fatalf("ZvolSnapshotName must be a pure function of its input time")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
