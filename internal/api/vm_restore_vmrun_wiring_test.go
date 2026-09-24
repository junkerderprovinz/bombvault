package api_test

// These tests run a real BackupVM and RestoreVM through fakeResticEngine and
// check that DumpRaw (zvol disks) and RestorePath (the file disk) get the right
// snapshot and path for each disk, including the "vmrun:<runID>" grouping.
//
// They skip on Windows: HostMountRoot is a real temp dir there, and paths.Within
// needs a leading "/". vm_restore_vmrun_internal_test.go covers the same plan on
// every platform through prepareRestoreVMForTarget.

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// seedVMsRepoConfig writes restic's config marker into <root>/backups/vms, so
// snapshotsForTag sees the repo and lists eng.snaps. Only the restore side
// needs it; the fake engine's backups never touch the filesystem.
func seedVMsRepoConfig(t *testing.T, root string) {
	t.Helper()
	repoDir := filepath.Join(root, "backups", "vms")
	if err := os.MkdirAll(repoDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "config"), []byte("cfg"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// zvolTagOf splits a "path:tag,tag,..." BackupStdin recording into its stdin
// path and its "vm:<name>:zvol:<dev>" tag.
func zvolTagOf(entry string) (path, tag string) {
	path, tags, _ := strings.Cut(entry, ":")
	for _, t := range strings.Split(tags, ",") {
		if strings.Contains(t, ":zvol:") {
			return path, t
		}
	}
	return path, ""
}

// A "latest" restore of a VM with one file disk and two zvol disks dumps each
// zvol from its own snapshot and restores the file disk from the file-backed
// one.
func TestRestoreVMRestoresEachDiskFromItsOwnSnapshot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("drives BackupVM/RestoreVM with a real OS temp dir as HostMountRoot: paths.Within needs a leading /")
	}
	svc, eng, _, root := vmZvolTestService(t, mixedVMDomainXML, &zvolTPMSSH{})

	if _, err := svc.BackupVM(context.Background(), "mixedvm"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}
	if len(eng.lastTags) != 3 {
		t.Fatalf("file-backed tags = %v, want 3 (vm:mixedvm,p2,vmrun:<id>)", eng.lastTags)
	}
	runTag := eng.lastTags[2]
	if len(eng.stdinBackups) != 2 {
		t.Fatalf("stdinBackups = %v, want 2 zvol backup calls", eng.stdinBackups)
	}

	// The listing the backup would have produced: the file-backed snapshot plus
	// one per zvol disk, each with its identity tag, the run tag and its stdin
	// path.
	snaps := []restic.Snapshot{{ID: "deadbeef12345678", Tags: []string{"vm:mixedvm", "p2", runTag}, Paths: eng.lastPaths}}
	for i, entry := range eng.stdinBackups {
		path, tag := zvolTagOf(entry)
		if tag == "" {
			t.Fatalf("stdinBackups[%d] = %q: no zvol identity tag found", i, entry)
		}
		snaps = append(snaps, restic.Snapshot{
			ID:    "zvolSnap" + strconv.Itoa(i+1),
			Tags:  []string{tag, "p2", runTag},
			Paths: []string{path},
		})
	}
	eng.snaps = snaps
	seedVMsRepoConfig(t, root)

	if err := svc.RestoreVM(context.Background(), "mixedvm", "latest", true, "", true); err != nil {
		t.Fatalf("RestoreVM: %v", err)
	}

	if len(eng.restored) != 1 || !strings.Contains(eng.restored[0], ":deadbeef12345678:") {
		t.Fatalf("restored = %v, want exactly one RestorePath call against deadbeef12345678", eng.restored)
	}

	// Each zvol disk is dumped from its own snapshot at its own stdin path.
	if len(eng.dumpRawCalls) != 2 {
		t.Fatalf("dumpRawCalls = %v, want 2 (one per zvol disk)", eng.dumpRawCalls)
	}
	wantByPath := map[string]string{} // stdin path to expected snapshot ID
	for i, entry := range eng.stdinBackups {
		path, _ := zvolTagOf(entry)
		wantByPath[path] = "zvolSnap" + strconv.Itoa(i+1)
	}
	for _, call := range eng.dumpRawCalls {
		gotID, gotPath, ok := strings.Cut(call, ":")
		if !ok {
			t.Fatalf("dumpRawCalls entry %q: malformed (want snapshotID:path)", call)
		}
		wantID, known := wantByPath[gotPath]
		if !known {
			t.Fatalf("dumpRawCalls entry %q: path %q not one of this backup's own zvol stdin paths", call, gotPath)
		}
		if gotID != wantID {
			t.Fatalf("dumpRawCalls entry %q: dumped from snapshot %q, want %q (this disk's own backup)", call, gotID, wantID)
		}
	}
}

// BackupVM sets no run tag for a file-only VM, which covers most VMs, so its
// restore uses the one file-backed snapshot and dumps no zvol.
func TestRestoreVMFileOnlyUsesSingleSnapshot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("drives BackupVM/RestoreVM with a real OS temp dir as HostMountRoot: paths.Within needs a leading /")
	}
	svc, eng, _, root := vmZvolTestService(t, fileOnlyVMDomainXML, &zvolTPMSSH{})

	if _, err := svc.BackupVM(context.Background(), "plainvm"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}
	if strings.Join(eng.lastTags, ",") != "vm:plainvm,p2" {
		t.Fatalf("tags = %v, want [vm:plainvm p2] (no vmrun: tag for a file-only VM)", eng.lastTags)
	}

	eng.snaps = []restic.Snapshot{{ID: "deadbeef12345678", Tags: []string{"vm:plainvm", "p2"}, Paths: eng.lastPaths}}
	seedVMsRepoConfig(t, root)

	if err := svc.RestoreVM(context.Background(), "plainvm", "latest", true, "", true); err != nil {
		t.Fatalf("RestoreVM: %v", err)
	}
	if len(eng.restored) != 1 || !strings.Contains(eng.restored[0], ":deadbeef12345678:") {
		t.Fatalf("restored = %v, want exactly one RestorePath call against deadbeef12345678", eng.restored)
	}
	if len(eng.dumpRawCalls) != 0 {
		t.Fatalf("dumpRawCalls = %v, want none for a file-only VM", eng.dumpRawCalls)
	}
}

// A mixed VM backed up before the vmrun: tag existed restores its file disk from
// the plain "vm:"+name tag. RestoreZvolDisk still runs for each zvol disk, but
// with an empty snapshot ID instead of a guessed one.
func TestRestoreVMMixedDiskHistoricalRunFallsBackWithoutInventingSnapshots(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("drives BackupVM/RestoreVM with a real OS temp dir as HostMountRoot: paths.Within needs a leading /")
	}
	svc, eng, _, root := vmZvolTestService(t, mixedVMDomainXML, &zvolTPMSSH{})

	if _, err := svc.BackupVM(context.Background(), "mixedvm"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}

	// Only the file-backed snapshot is listed, without a vmrun: tag.
	eng.snaps = []restic.Snapshot{{ID: "deadbeef12345678", Tags: []string{"vm:mixedvm", "p2"}, Paths: eng.lastPaths}}
	seedVMsRepoConfig(t, root)

	if err := svc.RestoreVM(context.Background(), "mixedvm", "latest", true, "", true); err != nil {
		t.Fatalf("RestoreVM: %v", err)
	}
	if len(eng.restored) != 1 || !strings.Contains(eng.restored[0], ":deadbeef12345678:") {
		t.Fatalf("restored = %v, want exactly one RestorePath call against deadbeef12345678 (unchanged fallback)", eng.restored)
	}
	if len(eng.dumpRawCalls) != 2 {
		t.Fatalf("dumpRawCalls = %v, want 2 (RestoreZvolDisk still runs per disk)", eng.dumpRawCalls)
	}
	for _, call := range eng.dumpRawCalls {
		if !strings.HasPrefix(call, ":") {
			t.Fatalf("dumpRawCalls entry %q: want an empty snapshot id (no vmrun: group to resolve from), got an invented one", call)
		}
	}
}
