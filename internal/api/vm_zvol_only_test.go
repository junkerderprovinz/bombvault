package api_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	if len(eng.stdinBackups) != 1 || !strings.Contains(eng.stdinBackups[0], "vm:testvm:zvol:vda") {
		t.Fatalf("zvol backups = %v, want one for dev vda", eng.stdinBackups)
	}
	if sum.SnapshotID == "" {
		t.Fatalf("summary %+v carries no snapshot id", sum)
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
	for _, want := range []string{"/dev/zvol/tank/vms/testvm", "included in its backups", "cannot be exported as files"} {
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
