package backup_test

import (
	"context"
	"errors"
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

// Once restic has written the snapshot the backup is kept, so the service has
// to learn that before the restart begins: a cancel in the restart could only
// claim to stop a backup that is already done.
func TestContainerBackupCommitsBetweenTheSnapshotAndTheRestart(t *testing.T) {
	for _, c := range []struct {
		name      string
		backupErr error
		want      int
	}{
		{"snapshot written", nil, 1},
		{"restic failed", errors.New("restic: repository locked"), 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := &fakeDocker{}
			r := &fakeRestic{backupErr: c.backupErr, summary: backup.Summary{SnapshotID: "deadbeef12345678"}}
			commits := 0
			var dockerAtCommit []string

			_, _ = backup.BackupContainer(t.Context(), backup.BackupDeps{
				ContainerRef:   "nextcloud",
				ContainerName:  "Nextcloud",
				RepoPath:       "/repo",
				AppdataPaths:   []string{"/host/user/appdata/nextcloud"},
				TargetID:       "target-1",
				WasRunning:     true,
				StopContainers: []backup.StopContainer{{Name: "mariadb", WasRunning: true}},
				Docker:         d,
				Restic:         r,
				Templates:      &fakeTemplates{},
				Runs:           &fakeRuns{},
				Committed: func() {
					commits++
					dockerAtCommit = append([]string(nil), d.log...)
				},
			})

			if commits != c.want {
				t.Fatalf("committed %d times, want %d", commits, c.want)
			}
			if c.want == 0 {
				return
			}
			if !contains(dockerAtCommit, "stop:mariadb") || contains(dockerAtCommit, "start:nextcloud") || contains(dockerAtCommit, "start:mariadb") {
				t.Fatalf("docker log at the commit = %v, want everything stopped and nothing started yet", dockerAtCommit)
			}
		})
	}
}

// Docker finishes a stop the client gave up on. A cancel that lands while the
// container is stopping must not start it again before it is down, or the
// daemon stops it for good right after the restart.
func TestCancelWhileStoppingStillLeavesTheContainerRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	d := &fakeDocker{onStop: cancel}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}
	runs := &fakeRuns{}

	_, err := backup.BackupContainer(ctx, backup.BackupDeps{
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
		Runs:           runs,
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the cancel", err)
	}
	if len(r.log) != 0 {
		t.Errorf("restic ran after the cancel: %v", r.log)
	}
	for i, err := range d.stopCtxErrs {
		if err != nil {
			t.Errorf("stop #%d was abandoned by the cancel (%v): the daemon would finish it after the restart", i, err)
		}
	}
	if contains(d.log, "stop:mariadb") && !contains(d.log, "start:mariadb") {
		t.Errorf("docker log = %v, the dependency was stopped and not started again", d.log)
	}
	if !contains(d.log, "start:nextcloud") {
		t.Fatalf("docker log = %v, want the container started again", d.log)
	}
}

// The guest keeps shutting down after the request was sent, so a cancel that
// lands before it is off has to wait for it before starting it: a start on a
// VM that is still up does nothing, and the shutdown then leaves it off.
func TestCancelWhileShuttingDownStillLeavesTheVMRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	vm := &fakeVM{active: true, stateSeq: []string{"running", "shut off"}, onShutdown: cancel}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}

	_, err := backup.BackupVMGraceful(ctx, sampleVMBackupDeps(t, vm, r, &fakeRuns{}))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the cancel", err)
	}
	if len(r.log) != 0 {
		t.Errorf("restic ran after the cancel: %v", r.log)
	}
	if len(vm.startStates) != 1 || vm.startStates[0] != "shut off" {
		t.Fatalf("start saw the VM in %q (vm log %v), want it started once after it was off", vm.startStates, vm.log)
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
