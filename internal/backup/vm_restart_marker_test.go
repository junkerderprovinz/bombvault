package backup_test

import (
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

// vmMarker keeps the record in the VM fake's log, in order with the shutdown
// and the start.
type vmMarker struct {
	vm     *fakeVM
	marked map[string]bool
}

func (m *vmMarker) Mark(names []string) error {
	for _, n := range names {
		m.vm.log = append(m.vm.log, "mark:"+n)
		m.marked[n] = true
	}
	return nil
}

func (m *vmMarker) Clear(names []string) error {
	for _, n := range names {
		m.vm.log = append(m.vm.log, "clear:"+n)
		delete(m.marked, n)
	}
	return nil
}

func TestGracefulVMBackupRecordsTheVMUntilItRunsAgain(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	m := &vmMarker{vm: vm, marked: map[string]bool{}}
	deps := sampleVMBackupDeps(t, vm, &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}, &fakeRuns{})
	deps.Restarts = m

	if _, err := backup.BackupVMGraceful(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{"mark:win10", "shutdown:win10"}, {"start:win10", "clear:win10"}} {
		if !before(vm.log, pair[0], pair[1]) {
			t.Errorf("%s has to come before %s: %v", pair[0], pair[1], vm.log)
		}
	}
	if len(m.marked) != 0 {
		t.Errorf("still recorded after the backup: %v", m.marked)
	}
}

func TestGracefulVMBackupKeepsTheRecordOfAVMThatDidNotStart(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off", startErr: errors.New("connection reset")}
	m := &vmMarker{vm: vm, marked: map[string]bool{}}
	deps := sampleVMBackupDeps(t, vm, &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}, &fakeRuns{})
	deps.Restarts = m

	_, _ = backup.BackupVMGraceful(t.Context(), deps)
	if !m.marked["win10"] {
		t.Fatalf("a VM left shut off has to stay recorded: %v", vm.log)
	}
}

func TestGracefulVMBackupForgetsAVMThatNeverShutDown(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "running", shutdownErr: errors.New("domain is not running")}
	m := &vmMarker{vm: vm, marked: map[string]bool{}}
	deps := sampleVMBackupDeps(t, vm, &fakeRestic{}, &fakeRuns{})
	deps.Restarts = m

	_, _ = backup.BackupVMGraceful(t.Context(), deps)
	if len(m.marked) != 0 {
		t.Fatalf("a refused shutdown left a record: %v", vm.log)
	}
}
