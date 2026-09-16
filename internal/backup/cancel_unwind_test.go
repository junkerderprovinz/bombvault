package backup_test

import (
	"context"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

// Cancelling a backup (the cancel button, BACKUP_MAX_HOURS, shutdown) ends the
// run's context. The restart that follows used the same context, and the Docker
// SDK refuses a request on a done context straight away, so the container that
// had been stopped for its backup stayed stopped: cancelling a backup took the
// app down. The restart has to outlive the cancellation.
func TestCancelledContainerBackupStillRestartsTargetAndDependencies(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	d := &fakeDocker{}
	r := &fakeRestic{onBackup: cancel, summary: backup.Summary{SnapshotID: "deadbeef12345678"}}

	_, _ = backup.BackupContainer(ctx, backup.BackupDeps{
		ContainerRef:   "nextcloud",
		ContainerName:  "Nextcloud",
		RepoPath:       "/repo",
		AppdataPaths:   []string{"/host/user/appdata/nextcloud"},
		StopTimeout:    30 * time.Second,
		TargetID:       "target-1",
		WasRunning:     true,
		StopContainers: []backup.StopContainer{{Name: "mariadb", WasRunning: true}},
		Docker:         d,
		Restic:         r,
		Templates:      &fakeTemplates{},
		Runs:           &fakeRuns{},
	})

	if ctx.Err() == nil {
		t.Fatal("test setup: the backup context was never cancelled")
	}
	if !contains(d.log, "start:nextcloud") || !contains(d.log, "start:mariadb") {
		t.Fatalf("docker log = %v, want both containers started again", d.log)
	}
	for i, err := range d.startCtxErrs {
		if err != nil {
			t.Errorf("start #%d ran on a cancelled context (%v): the real SDK would refuse it and leave the container stopped", i, err)
		}
	}
	for i, err := range d.waitCtxErrs {
		if err != nil {
			t.Errorf("waitRunning #%d ran on a cancelled context (%v)", i, err)
		}
	}
}

// A live backup that is cancelled still has to fold its overlays back.
// Otherwise the VM keeps running on the snapshot overlay and every later backup
// of it fails until someone resolves the overlay by hand.
func TestCancelledLiveVMBackupStillCommitsOverlays(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	vm := &fakeVM{active: true}
	r := &fakeRestic{onBackup: cancel, summary: backup.Summary{SnapshotID: "deadbeef12345678"}}
	d := liveDeps(t, vm, r, &fakeRuns{})
	d.CommitDevs = []string{"vda", "vdb"}

	_, _ = backup.BackupVMLive(ctx, d)

	if ctx.Err() == nil {
		t.Fatal("test setup: the backup context was never cancelled")
	}
	if len(vm.commitCtxErrs) != 2 {
		t.Fatalf("blockcommit calls = %d, want 2 (vm log %v)", len(vm.commitCtxErrs), vm.log)
	}
	for i, err := range vm.commitCtxErrs {
		if err != nil {
			t.Errorf("blockcommit #%d ran on a cancelled context (%v): the VM would stay on its overlay", i, err)
		}
	}
}

// The same holds for a VM shut down for a graceful backup.
func TestCancelledVMBackupStillStartsTheVM(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{onBackup: cancel, summary: backup.Summary{SnapshotID: "deadbeef12345678"}}

	_, _ = backup.BackupVMGraceful(ctx, sampleVMBackupDeps(t, vm, r, &fakeRuns{}))

	if ctx.Err() == nil {
		t.Fatal("test setup: the backup context was never cancelled")
	}
	if !vmContains(vm.log, "start:win10") {
		t.Fatalf("vm log = %v, want the VM started again", vm.log)
	}
	for i, err := range vm.startCtxErrs {
		if err != nil {
			t.Errorf("start #%d ran on a cancelled context (%v): libvirt would refuse it and leave the VM off", i, err)
		}
	}
}
