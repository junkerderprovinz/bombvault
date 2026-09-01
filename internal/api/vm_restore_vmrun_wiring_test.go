package api_test

// End-to-end proof that the real RestoreVM (via prepareRestoreVMForTarget's
// "vmrun:<runID>" group resolution, internal/api/service.go) actually reaches
// the restic engine with the CORRECT values — v8.0.0 VM service-layer
// integration, Task 3 (the design notes). Complements
// vm_restore_vmrun_internal_test.go's plan-level assertions by driving a real
// BackupVM->RestoreVM round trip through fakeResticEngine (service_test.go)
// and confirming DumpRaw (the zvol restore-side dump, resticZvolAdapter.
// DumpTo's target) and RestorePath (the main file-backed restore) each
// receive the right (snapshotID, path) pair for the right disk.
//
// Windows-skipped, same reason and same precedent as
// foreign_vm_restore_internal_test.go's TestForeignRestoreVMLeavesStoppedAndRemaps:
// this drives BackupVM/RestoreVM through vmZvolTestService's REAL OS temp dir
// as HostMountRoot, and paths.Within requires a leading "/" — a real Windows
// path never has one, so the disk-path containment check that guards restore
// always (correctly, structurally) refuses it there. Unaffected by Task 3;
// vm_restore_vmrun_internal_test.go covers the same requirements cross-platform
// via prepareRestoreVMForTarget directly with the project's established
// slash-literal HostMountRoot convention.

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

// seedVMsRepoConfig writes restic's "config" marker into <root>/backups/vms —
// the vms domain's local repo dir (settings.VMsPath, set by
// vmZvolTestService) — so snapshotsForTag's localRepoMissing check reads the
// repo as present and actually lists eng.snaps instead of silently reporting
// "no snapshots yet". BackupVM never needs this (the fake engine's
// Backup/BackupStdin calls never touch the filesystem), but RestoreVM's
// snapshot-listing path does.
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

// zvolTagOf returns the tag of the zvol identity form "vm:<name>:zvol:<dev>"
// out of a "path:tag,tag,..." BackupStdin recording (fakeResticEngine.
// stdinBackups' shape) — used to attribute each recorded zvol backup call to
// its device, and pull out its stdin path, without hardcoding call order.
func zvolTagOf(entry string) (path, tag string) {
	path, tags, _ := strings.Cut(entry, ":")
	for _, t := range strings.Split(tags, ",") {
		if strings.Contains(t, ":zvol:") {
			return path, t
		}
	}
	return path, ""
}

// TestRestoreVMWithVmrunGroupRestoresAllThreeSnapshotsToCorrectTargets pins
// requirement (a)+(c): a "latest" restore of a mixed file+2-zvol-disk VM
// backup dumps EACH zvol disk from ITS OWN restic snapshot (not the main
// file-backed one, not each other's) and restores the main file disk from
// the file-backed snapshot — proven by inspecting the fake engine's actual
// DumpRaw/RestorePath call recordings, the same fakes the rest of this
// package's restore tests verify against.
func TestRestoreVMWithVmrunGroupRestoresAllThreeSnapshotsToCorrectTargets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("drives BackupVM/RestoreVM with a real OS temp dir as HostMountRoot (paths.Within needs a leading /) — see file header comment")
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

	// Build the restic snapshot listing the backup above would have produced:
	// the main file-backed snapshot (the fake's fixed Backup id) plus one per
	// zvol disk (the fake's fixed, sequential BackupStdin ids), each carrying
	// its own identity tag + the shared runTag, with Paths = the exact stdin
	// path BackupStdin recorded (mirrors what a real restic snapshot listing
	// reports).
	snaps := []restic.Snapshot{{ID: "deadbeef12345678", Tags: []string{"vm:mixedvm", "p2", runTag}}}
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

	// The main file disk was restored from the FILE-BACKED snapshot.
	if len(eng.restored) != 1 || !strings.Contains(eng.restored[0], ":deadbeef12345678:") {
		t.Fatalf("restored = %v, want exactly one RestorePath call against deadbeef12345678", eng.restored)
	}

	// Each zvol disk was dumped from ITS OWN snapshot at ITS OWN stdin path —
	// never the main snapshot, never the other disk's.
	if len(eng.dumpRawCalls) != 2 {
		t.Fatalf("dumpRawCalls = %v, want 2 (one per zvol disk)", eng.dumpRawCalls)
	}
	wantByPath := map[string]string{} // stdin path -> expected snapshot id, from the backup-side recording
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
			t.Fatalf("dumpRawCalls entry %q: dumped from snapshot %q, want %q (this disk's OWN backup)", call, gotID, wantID)
		}
	}
}

// TestRestoreVMFileOnlyByteIdenticalWithNoVmrunGroupQuery pins requirement
// (b), the critical regression case: a file-only VM's restore (every Unraid
// VM, and most VMs in production generally — BackupVM never sets RunTag for
// one, so its snapshot never carries a "vmrun:" tag) must restore EXACTLY the
// one file-backed snapshot, with zero zvol dump calls — completely unaffected
// by the vmrun: group-resolution logic Task 3 adds.
func TestRestoreVMFileOnlyByteIdenticalWithNoVmrunGroupQuery(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("drives BackupVM/RestoreVM with a real OS temp dir as HostMountRoot (paths.Within needs a leading /) — see file header comment")
	}
	svc, eng, _, root := vmZvolTestService(t, fileOnlyVMDomainXML, &zvolTPMSSH{})

	if _, err := svc.BackupVM(context.Background(), "plainvm"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}
	if strings.Join(eng.lastTags, ",") != "vm:plainvm,p2" {
		t.Fatalf("tags = %v, want [vm:plainvm p2] (no vmrun: tag for a file-only VM)", eng.lastTags)
	}

	eng.snaps = []restic.Snapshot{{ID: "deadbeef12345678", Tags: []string{"vm:plainvm", "p2"}}}
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

// TestRestoreVMMixedDiskHistoricalRunFallsBackWithoutInventingSnapshots pins
// requirement (b)'s OTHER shape: a mixed file+zvol VM whose Run predates the
// "vmrun:" correlation tag (its snapshot carries none) must restore the main
// disk from the plain "vm:"+name tag exactly as before, and must NOT invent a
// snapshot id for the zvol disks — RestoreZvolDisk is still invoked (a no-op
// domain-XML-driven decision, unchanged since Task 2) but with an EMPTY
// snapshot id, exactly as it was before this task, rather than silently
// resolving to the wrong (or any) snapshot.
func TestRestoreVMMixedDiskHistoricalRunFallsBackWithoutInventingSnapshots(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("drives BackupVM/RestoreVM with a real OS temp dir as HostMountRoot (paths.Within needs a leading /) — see file header comment")
	}
	svc, eng, _, root := vmZvolTestService(t, mixedVMDomainXML, &zvolTPMSSH{})

	if _, err := svc.BackupVM(context.Background(), "mixedvm"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}

	// Simulate a pre-existing Run: only the main file-backed snapshot is
	// listed, and it carries NO "vmrun:" tag at all.
	eng.snaps = []restic.Snapshot{{ID: "deadbeef12345678", Tags: []string{"vm:mixedvm", "p2"}}}
	seedVMsRepoConfig(t, root)

	if err := svc.RestoreVM(context.Background(), "mixedvm", "latest", true, "", true); err != nil {
		t.Fatalf("RestoreVM: %v", err)
	}
	if len(eng.restored) != 1 || !strings.Contains(eng.restored[0], ":deadbeef12345678:") {
		t.Fatalf("restored = %v, want exactly one RestorePath call against deadbeef12345678 (unchanged fallback)", eng.restored)
	}
	if len(eng.dumpRawCalls) != 2 {
		t.Fatalf("dumpRawCalls = %v, want 2 (RestoreZvolDisk still runs per disk, unchanged since Task 2)", eng.dumpRawCalls)
	}
	for _, call := range eng.dumpRawCalls {
		if !strings.HasPrefix(call, ":") {
			t.Fatalf("dumpRawCalls entry %q: want an EMPTY snapshot id (no vmrun: group to resolve from) — got a non-empty one, meaning a snapshot id was invented", call)
		}
	}
}
