package backup_test

import (
	"context"
	"errors"
	"strings"
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

func TestGracefulVMBackupThatFailsAndCannotStartTheVMAgainSaysSoFirst(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off", startErr: errors.New("device busy")}
	r := &fakeRestic{backupErr: errors.New("repository locked")}
	m := &vmMarker{vm: vm, marked: map[string]bool{}}
	deps := sampleVMBackupDeps(t, vm, r, &fakeRuns{})
	deps.Restarts = m

	_, err := backup.BackupVMGraceful(t.Context(), deps)
	if err == nil || !strings.HasPrefix(err.Error(), backup.VMRestartFailed+": device busy") || !strings.Contains(err.Error(), "repository locked") {
		t.Fatalf("err = %v", err)
	}
	if !m.marked["win10"] {
		t.Fatal("a VM left off has to stay recorded")
	}
}

func TestGracefulVMBackupWhoseGuestIsStillShuttingDownEndsAtOnceAndHandsItOver(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "running"}
	m := &vmMarker{vm: vm, marked: map[string]bool{}}
	deps := sampleVMBackupDeps(t, vm, &fakeRestic{}, &fakeRuns{})
	deps.Restarts = m
	deps.ShutdownTimeout = 1
	handedOver := 0
	deps.LateShutdown = func() { handedOver++ }

	_, err := backup.BackupVMGraceful(t.Context(), deps)
	if !errors.Is(err, backup.ErrVMShutdownTimeout) {
		t.Fatalf("err = %v", err)
	}
	if handedOver != 1 {
		t.Fatalf("handed over %d times", handedOver)
	}
	if vmContains(vm.log, "start:") || !m.marked["win10"] {
		t.Fatalf("calls = %v, record = %v", vm.log, m.marked)
	}
}

func TestGracefulVMBackupCancelledDuringTheShutdownHandsTheGuestOver(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	vm := &fakeVM{active: true, stateVal: "running", onShutdown: cancel}
	deps := sampleVMBackupDeps(t, vm, &fakeRestic{}, &fakeRuns{})
	handedOver := 0
	deps.LateShutdown = func() { handedOver++ }

	if _, err := backup.BackupVMGraceful(ctx, deps); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if handedOver != 1 || vmContains(vm.log, "start:") {
		t.Fatalf("handed over %d times, calls = %v", handedOver, vm.log)
	}
}

func TestGracefulVMBackupWaitsForAGuestAlreadyShuttingDownAndLeavesItOff(t *testing.T) {
	vm := &fakeVM{stateSeq: []string{"in shutdown", "in shutdown", "shut off"}}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}
	deps := sampleVMBackupDeps(t, vm, r, &fakeRuns{})

	if _, err := backup.BackupVMGraceful(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
	if vm.lastState != "shut off" || len(r.log) == 0 {
		t.Fatalf("read while the guest was %q: restic %v", vm.lastState, r.log)
	}
	if vmContains(vm.log, "start:") {
		t.Fatalf("calls = %v", vm.log)
	}
}

func TestGracefulVMBackupDoesNotReadAGuestThatStaysInShutdown(t *testing.T) {
	vm := &fakeVM{stateVal: "in shutdown"}
	r := &fakeRestic{}
	deps := sampleVMBackupDeps(t, vm, r, &fakeRuns{})
	deps.ShutdownTimeout = 1

	if _, err := backup.BackupVMGraceful(t.Context(), deps); !errors.Is(err, backup.ErrVMShutdownTimeout) {
		t.Fatalf("err = %v", err)
	}
	if len(r.log) != 0 {
		t.Fatalf("restic ran: %v", r.log)
	}
}
