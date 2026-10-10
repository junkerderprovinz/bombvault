package backup_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/progress"
)

// stepsAmong returns the entries of log that a step or one of the given
// prefixes wrote, in order.
func stepsAmong(log []string, prefixes ...string) []string {
	var out []string
	for _, e := range log {
		if strings.HasPrefix(e, "step:") || slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(e, p) }) {
			out = append(out, e)
		}
	}
	return out
}

func TestBackupContainerNamesItsStepsAroundTheStopAndTheStart(t *testing.T) {
	d := &fakeDocker{}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}
	r.onBackup = func() { d.log = append(d.log, "restic") }

	_, err := backup.BackupContainer(t.Context(), backup.BackupDeps{
		ContainerRef: "plex", ContainerName: "Plex", RepoPath: "/repo",
		AppdataPaths: []string{"/host/user/appdata/plex"}, TargetID: "t1",
		WasRunning: true,
		Step:       func(step string) { d.log = append(d.log, "step:"+step) },
		Docker:     d, Restic: r, Templates: &fakeTemplates{}, Runs: &fakeRuns{},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"step:" + progress.StepStop, "stop:plex",
		"step:" + progress.StepRead, "restic",
		"step:" + progress.StepStart, "start:plex",
	}
	if got := stepsAmong(d.log, "stop:", "restic", "start:"); !slices.Equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestBackupContainerNamesItsStepsForAStoppedDependency(t *testing.T) {
	d := &fakeDocker{}
	var steps []string

	_, err := backup.BackupContainer(t.Context(), backup.BackupDeps{
		ContainerRef: "app", ContainerName: "app", RepoPath: "/repo",
		AppdataPaths: []string{"/host/user/appdata/app"}, TargetID: "t1",
		StopContainers: []backup.StopContainer{{Name: "db", WasRunning: true}},
		Step:           func(step string) { steps = append(steps, step) },
		Docker:         d, Restic: &fakeRestic{}, Templates: &fakeTemplates{}, Runs: &fakeRuns{},
	})
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{progress.StepStop, progress.StepRead, progress.StepStart}; !slices.Equal(steps, want) {
		t.Fatalf("steps = %v, want %v", steps, want)
	}
}

func TestBackupContainerThatStopsNothingNamesNoStep(t *testing.T) {
	var steps []string

	_, err := backup.BackupContainer(t.Context(), backup.BackupDeps{
		ContainerRef: "plex", ContainerName: "Plex", RepoPath: "/repo",
		AppdataPaths: []string{"/host/user/appdata/plex"}, TargetID: "t1",
		StopContainers: []backup.StopContainer{{Name: "db"}},
		Step:           func(step string) { steps = append(steps, step) },
		Docker:         &fakeDocker{}, Restic: &fakeRestic{}, Templates: &fakeTemplates{}, Runs: &fakeRuns{},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(steps) != 0 {
		t.Fatalf("steps = %v, want none", steps)
	}
}

func TestBackupContainerNamesTheStartAfterAFailedRead(t *testing.T) {
	var steps []string

	_, err := backup.BackupContainer(t.Context(), backup.BackupDeps{
		ContainerRef: "plex", ContainerName: "Plex", RepoPath: "/repo",
		AppdataPaths: []string{"/host/user/appdata/plex"}, TargetID: "t1",
		WasRunning: true,
		Step:       func(step string) { steps = append(steps, step) },
		Docker:     &fakeDocker{}, Restic: &fakeRestic{backupErr: errors.New("boom")}, Templates: &fakeTemplates{}, Runs: &fakeRuns{},
	})
	if err == nil {
		t.Fatal("the backup reported no error")
	}

	if want := []string{progress.StepStop, progress.StepRead, progress.StepStart}; !slices.Equal(steps, want) {
		t.Fatalf("steps = %v, want %v", steps, want)
	}
}

func TestBackupVMGracefulNamesItsStepsAroundTheShutdownAndTheStart(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}
	r.onBackup = func() { vm.log = append(vm.log, "restic") }
	deps := sampleVMBackupDeps(t, vm, r, &fakeRuns{})
	deps.Step = func(step string) { vm.log = append(vm.log, "step:"+step) }

	if _, err := backup.BackupVMGraceful(t.Context(), deps); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"step:" + progress.StepStop, "shutdown:win10",
		"step:" + progress.StepRead, "restic",
		"step:" + progress.StepStart, "start:win10",
	}
	if got := stepsAmong(vm.log, "shutdown:", "restic", "start:"); !slices.Equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestBackupVMGracefulOfAStoppedVMNamesNoStep(t *testing.T) {
	vm := &fakeVM{stateVal: "shut off"}
	var steps []string
	deps := sampleVMBackupDeps(t, vm, &fakeRestic{}, &fakeRuns{})
	deps.Step = func(step string) { steps = append(steps, step) }

	if _, err := backup.BackupVMGraceful(t.Context(), deps); err != nil {
		t.Fatal(err)
	}

	if len(steps) != 0 {
		t.Fatalf("steps = %v, want none", steps)
	}
}
