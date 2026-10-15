package api_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const zvolOnlyVMDomainXML = `<domain type='kvm'>
  <devices>
    <disk type='block' device='disk'>
      <source dev='/dev/zvol/tank/vms/testvm'/>
      <target dev='vda'/>
    </disk>
  </devices>
</domain>`

const noDiskVMDomainXML = `<domain type='kvm'><devices></devices></domain>`

func TestBackupVMWithOnlyZvolDisksBacksUpTheZvols(t *testing.T) {
	svc, eng, st, _ := vmZvolTestService(t, zvolOnlyVMDomainXML, &zvolTPMSSH{})

	sum, err := svc.BackupVM(context.Background(), "testvm")
	if err != nil {
		t.Fatalf("BackupVM: %v", err)
	}
	if len(eng.backedUp) != 0 {
		t.Fatalf("restic read file paths %v for a VM without file disks", eng.backedUp)
	}
	if len(eng.stdinBackups) != 2 || !strings.Contains(eng.stdinBackups[0], "vm:testvm:zvol:vda") {
		t.Fatalf("stdin backups = %v, want the zvol of dev vda and then the domain", eng.stdinBackups)
	}
	domainPath, domainTags, _ := strings.Cut(eng.stdinBackups[1], ":")
	if domainPath != "/vm-domains/testvm.xml" || !strings.HasPrefix(domainTags, "vm:testvm,p2,vmrun:") {
		t.Fatalf("domain backup = %q, want it under the VM's own tag and the run tag", eng.stdinBackups[1])
	}
	if sum.SnapshotID != "zvolSnap2" {
		t.Fatalf("summary %+v, want the run recorded under the domain snapshot", sum)
	}
	tg, err := st.GetVMTargetByName("testvm")
	if err != nil {
		t.Fatal(err)
	}
	if run, err := st.LastRunForTarget(tg.ID); err != nil || run == nil || run.Status != "success" {
		t.Fatalf("last run = %+v, %v; want a success", run, err)
	}
}

func TestBackupVMWithoutAnyDiskFailsAndShowsInTheHistory(t *testing.T) {
	svc, _, st, _ := vmZvolTestService(t, noDiskVMDomainXML, &zvolTPMSSH{})

	if _, err := svc.BackupVM(context.Background(), "emptyvm"); err == nil || !strings.Contains(err.Error(), "no disk paths found") {
		t.Fatalf("BackupVM error = %v, want the no disk paths error", err)
	}
	tg, err := st.GetVMTargetByName("emptyvm")
	if err != nil {
		t.Fatalf("a first backup that fails must leave a row to hang the run on: %v", err)
	}
	run, ok := failedRunFor(t, st, tg.ID)
	if !ok {
		t.Fatal("want a failed run for the VM, found none")
	}
	if !strings.Contains(run.Error, "no disk paths found") {
		t.Fatalf("run.Error = %q", run.Error)
	}
}

func TestBackupVMFailureBeforeTheRunKeepsAnExistingRow(t *testing.T) {
	svc, _, st, _ := vmZvolTestService(t, noDiskVMDomainXML, &zvolTPMSSH{})
	before, err := st.UpsertVMTarget(store.VMTarget{Name: "emptyvm", Method: "live", Definition: "{}"})
	if err != nil {
		t.Fatal(err)
	}

	_, _ = svc.BackupVM(context.Background(), "emptyvm")

	after, err := st.GetVMTargetByName("emptyvm")
	if err != nil {
		t.Fatal(err)
	}
	if after.ID != before.ID || after.Method != "live" {
		t.Fatalf("row changed from %+v to %+v", before, after)
	}
	if _, ok := failedRunFor(t, st, before.ID); !ok {
		t.Fatal("want a failed run on the existing row")
	}
}

func TestExportVMWithOnlyZvolDisksSaysWhereTheDataIs(t *testing.T) {
	svc, _, _, _ := vmZvolTestService(t, zvolOnlyVMDomainXML, &zvolTPMSSH{})

	_, _, err := svc.ExportVM(context.Background(), "testvm")
	if err == nil {
		t.Fatal("ExportVM: want an error for a zvol-only VM")
	}
	for _, want := range []string{"(tank/vms/testvm)", "included in its backups", "cannot be exported as files"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q lacks %q", err, want)
		}
	}
}

func TestExportVMOfAMixedVMNamesTheZvolsItLeftOut(t *testing.T) {
	svc, _, _, root := vmZvolTestService(t, mixedVMDomainXML, &zvolTPMSSH{})
	disk := filepath.Join(root, "user", "domains", "mixedvm", "vdisk0.qcow2")
	if err := os.MkdirAll(filepath.Dir(disk), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(disk, []byte("disk"), 0o600); err != nil {
		t.Fatal(err)
	}

	dir, zvols, err := svc.ExportVM(context.Background(), "mixedvm")
	if err != nil {
		t.Fatalf("ExportVM: %v", err)
	}
	want := []string{"/dev/zvol/tank/vms/mixedvm/disk1", "/dev/zvol/tank/vms/mixedvm/disk2"}
	if strings.Join(zvols, ",") != strings.Join(want, ",") {
		t.Fatalf("left out zvols = %v, want %v", zvols, want)
	}
	for _, f := range []string{"mixedvm.tar.gz", "mixedvm.xml", "mixedvm-zvols-not-exported.txt"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("export lacks %s: %v", f, err)
		}
	}
}

func TestAZvolOnlyVMListsItsBackupsAndRestoresTheZvol(t *testing.T) {
	svc, eng, _, root := vmZvolTestService(t, zvolOnlyVMDomainXML, &zvolTPMSSH{})
	if _, err := svc.BackupVM(context.Background(), "testvm"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}
	zvolPath, zvolTags, _ := strings.Cut(eng.stdinBackups[0], ":")
	domainPath, domainTags, _ := strings.Cut(eng.stdinBackups[1], ":")
	eng.snaps = []restic.Snapshot{
		{ID: "1111111111111111", Time: "2026-10-15T12:00:00Z", Tags: strings.Split(zvolTags, ","), Paths: []string{zvolPath},
			Summary: &restic.SnapshotSummary{TotalBytesProcessed: 1 << 30}},
		{ID: "deadbeef12345678", Time: "2026-10-15T12:00:05Z", Tags: strings.Split(domainTags, ","), Paths: []string{domainPath}},
	}
	seedVMsRepoConfig(t, root)

	snaps, err := svc.SnapshotsVM(context.Background(), "testvm", "")
	if err != nil {
		t.Fatalf("SnapshotsVM: %v", err)
	}
	if len(snaps) != 1 || snaps[0].ID != "deadbeef12345678" {
		t.Fatalf("listed %+v, want the run's domain snapshot", snaps)
	}
	times, _, err := svc.LatestVMBackupTimes(context.Background())
	if err != nil {
		t.Fatalf("LatestVMBackupTimes: %v", err)
	}
	if want := time.Date(2026, 10, 15, 12, 0, 5, 0, time.UTC).Unix(); times["testvm"] != want {
		t.Fatalf("last backup = %d, want %d", times["testvm"], want)
	}

	if err := svc.RestoreVM(context.Background(), "testvm", "latest", true, "", true); err != nil {
		t.Fatalf("RestoreVM: %v", err)
	}
	if len(eng.restored) != 0 {
		t.Fatalf("restored files %v for a VM without file disks", eng.restored)
	}
	if want := "1111111111111111:" + zvolPath; len(eng.dumpRawCalls) != 1 || eng.dumpRawCalls[0] != want {
		t.Fatalf("dumps = %v, want %q", eng.dumpRawCalls, want)
	}
}
