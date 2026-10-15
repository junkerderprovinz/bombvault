package backup_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// besideDeps is a restore of plex next to the original, as plex-restored.
func besideDeps(docker *fakeDocker, restic *fakeRestic, runs *fakeRuns) backup.BesideContainerDeps {
	in := sampleInspect()
	in.Name = "plex-restored"
	return backup.BesideContainerDeps{
		Original:     "plex",
		Copy:         in,
		RepoPath:     "/repo",
		SnapshotID:   "abcd1234",
		Dirs:         []backup.RestoreDir{{Subtree: "/appdata/plex", Target: "/appdata/plex-restored"}},
		MakeDirs:     func() error { return nil },
		TargetID:     "tg-1",
		Docker:       docker,
		ImagePresent: func(context.Context, string) bool { return true },
		Restic:       restic,
		Runs:         runs,
	}
}

func touchesOriginal(log []string) bool {
	return slices.ContainsFunc(log, func(l string) bool {
		verb, _, _ := strings.Cut(l, ":")
		return verb == "stop" || verb == "remove" || verb == "start" || verb == "exec"
	})
}

func TestRestoreContainerBesideCreatesAStoppedCopyAndLeavesTheOriginalAlone(t *testing.T) {
	docker, restic, runs := &fakeDocker{}, &fakeRestic{}, &fakeRuns{}
	made := false
	d := besideDeps(docker, restic, runs)
	d.MakeDirs = func() error {
		if len(restic.capturedPaths) > 0 {
			t.Error("restic ran before the copy's folders were made")
		}
		made = true
		return nil
	}

	if err := backup.RestoreContainerBeside(context.Background(), d); err != nil {
		t.Fatalf("RestoreContainerBeside: %v", err)
	}

	if !made {
		t.Fatal("the copy's folders were never made")
	}
	if touchesOriginal(docker.log) {
		t.Fatalf("a restore next to the original stopped, removed or started something: %v", docker.log)
	}
	if !slices.Equal(restic.capturedPaths, []string{"/appdata/plex->/appdata/plex-restored"}) {
		t.Fatalf("restored %v, want the subtree into the copy's folder", restic.capturedPaths)
	}
	if docker.createdName != "plex-restored" || docker.createdStart {
		t.Fatalf("created %q, started=%v, want plex-restored left stopped", docker.createdName, docker.createdStart)
	}
	if slices.Contains(docker.log, "pull:"+d.Copy.Config.Image) {
		t.Fatalf("an image that is on the host must not be pulled again: %v", docker.log)
	}
	if f := runs.finishOf(t, "run-1"); f.status != "success" || f.sum.SnapshotID != "abcd1234" {
		t.Fatalf("run = %+v, want a successful restore of abcd1234", f)
	}
	if !slices.Equal(runs.kinds, []string{"restore"}) {
		t.Fatalf("run kinds = %v", runs.kinds)
	}
}

func TestRestoreContainerBesideRefusesATakenNameBeforeItWritesAnything(t *testing.T) {
	docker, restic, runs := &fakeDocker{liveName: "plex-restored"}, &fakeRestic{}, &fakeRuns{}
	d := besideDeps(docker, restic, runs)
	d.MakeDirs = func() error {
		t.Error("the copy's folders were made although its name is taken")
		return nil
	}

	err := backup.RestoreContainerBeside(context.Background(), d)

	if !errors.Is(err, backup.ErrBesideTaken) {
		t.Fatalf("err = %v, want ErrBesideTaken", err)
	}
	if len(restic.log) != 0 || docker.createdName != "" || touchesOriginal(docker.log) {
		t.Fatalf("a refused restore did something: restic=%v docker=%v", restic.log, docker.log)
	}
	if f := runs.finishOf(t, "run-1"); f.status != "failed" {
		t.Fatalf("run = %+v, want failed", f)
	}
}

func TestRestoreContainerBesideRefusesACopyWhoseNetworkIsGone(t *testing.T) {
	docker, restic, runs := &fakeDocker{networks: []string{"bridge", "host", "none"}}, &fakeRestic{}, &fakeRuns{}
	d := besideDeps(docker, restic, runs)
	d.Copy.HostConfig.NetworkMode = "ix-plex_default"
	d.MakeDirs = func() error {
		t.Error("the copy's folders were made for a network that is gone")
		return nil
	}

	err := backup.RestoreContainerBeside(context.Background(), d)

	var gone *backup.NetworkGoneError
	if !errors.As(err, &gone) || gone.Name != "ix-plex_default" || !strings.Contains(err.Error(), "reinstall") {
		t.Fatalf("err = %v, want the refusal that names the network and says to reinstall the app", err)
	}
	if len(restic.log) != 0 || docker.createdName != "" || slices.Contains(docker.log, "pull:"+d.Copy.Config.Image) {
		t.Fatalf("a refused restore did something: restic=%v docker=%v", restic.log, docker.log)
	}
	if f := runs.finishOf(t, "run-1"); f.status != "failed" {
		t.Fatalf("run = %+v, want failed", f)
	}
}

func TestRestoreContainerBesideRefusesTheOriginalsOwnName(t *testing.T) {
	docker, restic, runs := &fakeDocker{}, &fakeRestic{}, &fakeRuns{}
	d := besideDeps(docker, restic, runs)
	d.Copy.Name = "/plex"

	if err := backup.RestoreContainerBeside(context.Background(), d); err == nil {
		t.Fatal("a copy named like the original must be refused")
	}
	if len(docker.log) != 0 || len(restic.log) != 0 {
		t.Fatalf("a refused restore did something: restic=%v docker=%v", restic.log, docker.log)
	}
}

func TestRestoreContainerBesidePullsAnImageTheHostLacks(t *testing.T) {
	docker, restic, runs := &fakeDocker{}, &fakeRestic{}, &fakeRuns{}
	d := besideDeps(docker, restic, runs)
	d.ImagePresent = func(context.Context, string) bool { return false }

	if err := backup.RestoreContainerBeside(context.Background(), d); err != nil {
		t.Fatalf("RestoreContainerBeside: %v", err)
	}
	if !slices.Contains(docker.log, "pull:"+d.Copy.Config.Image) {
		t.Fatalf("docker log = %v, want a pull of %s", docker.log, d.Copy.Config.Image)
	}
}

func TestRestoreContainerBesideStopsBeforeTheFoldersWhenTheSnapshotIsGone(t *testing.T) {
	docker, restic, runs := &fakeDocker{}, &fakeRestic{verifyErr: errors.New("no such snapshot")}, &fakeRuns{}
	d := besideDeps(docker, restic, runs)
	d.MakeDirs = func() error {
		t.Error("the copy's folders were made for a snapshot that is gone")
		return nil
	}

	if err := backup.RestoreContainerBeside(context.Background(), d); err == nil {
		t.Fatal("want the snapshot preflight to fail the restore")
	}
	if docker.createdName != "" {
		t.Fatal("no container may be created without its data")
	}
}

func TestRestoreContainerBesideCreatesNoContainerWhenTheRestoreFails(t *testing.T) {
	docker, restic, runs := &fakeDocker{}, &fakeRestic{restoreErr: context.Canceled}, &fakeRuns{}

	err := backup.RestoreContainerBeside(context.Background(), besideDeps(docker, restic, runs))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the cancellation", err)
	}
	if docker.createdName != "" {
		t.Fatal("no container may be created on half-restored data")
	}
	if f := runs.finishOf(t, "run-1"); f.status != "cancelled" {
		t.Fatalf("run = %+v, want cancelled", f)
	}
}

func TestRestoreContainerBesideWithoutRuntimeDropsItAndSaysSo(t *testing.T) {
	docker, restic, runs := &fakeDocker{}, &fakeRestic{}, &fakeRuns{}
	d := besideDeps(docker, restic, runs)
	d.Copy.HostConfig.Runtime = "nvidia"
	d.Copy.HostConfig.DeviceRequests = []model.DeviceRequest{{Driver: "nvidia", Count: -1}}
	d.WithoutRuntime = true

	if err := backup.RestoreContainerBeside(context.Background(), d); err != nil {
		t.Fatalf("RestoreContainerBeside: %v", err)
	}
	if hc := docker.createdInspect.HostConfig; hc.Runtime != "" || len(hc.DeviceRequests) != 0 {
		t.Fatalf("created with runtime %q and %d device requests", hc.Runtime, len(hc.DeviceRequests))
	}
	if f := runs.finishOf(t, "run-1"); f.note != store.NoteRestoredWithoutRuntime {
		t.Fatalf("note = %q", f.note)
	}
}

func TestRestoreContainerBesideOfADefinitionOnlyBackupNeedsNoSnapshot(t *testing.T) {
	docker, restic, runs := &fakeDocker{}, &fakeRestic{}, &fakeRuns{}
	d := besideDeps(docker, restic, runs)
	d.RecreateOnly, d.SnapshotID, d.Dirs = true, "", nil

	if err := backup.RestoreContainerBeside(context.Background(), d); err != nil {
		t.Fatalf("RestoreContainerBeside: %v", err)
	}
	if len(restic.log) != 0 || docker.createdName != "plex-restored" {
		t.Fatalf("restic=%v created=%q", restic.log, docker.createdName)
	}
}

func TestRestoreVMBesideRestoresTheDiskFoldersAndWritesItsFiles(t *testing.T) {
	dir := t.TempDir()
	restic, runs := &fakeRestic{}, &fakeRuns{}
	nvram := filepath.Join(dir, "VARS.fd")

	err := backup.RestoreVMBeside(context.Background(), backup.BesideVMDeps{
		RepoPath:   "/repo",
		SnapshotID: "abcd1234",
		Dirs:       []backup.VMRestoreDir{{Subtree: "/domains/win", Target: "/domains/win-restored"}},
		MakeDirs:   func() error { return nil },
		Files:      []backup.BesideFile{{Path: nvram, Data: []byte("vars")}},
		TargetID:   "vm-1",
		Restic:     restic,
		Runs:       runs,
	})
	if err != nil {
		t.Fatalf("RestoreVMBeside: %v", err)
	}
	if !slices.Equal(restic.capturedPaths, []string{"/domains/win->/domains/win-restored"}) {
		t.Fatalf("restored %v", restic.capturedPaths)
	}
	if got, err := os.ReadFile(nvram); err != nil || string(got) != "vars" { //nolint:gosec // G304: a file below the test's own folder
		t.Fatalf("nvram = %q, %v", got, err)
	}
	if f := runs.finishOf(t, "run-1"); f.status != "success" || f.sum.SnapshotID != "abcd1234" {
		t.Fatalf("run = %+v", f)
	}
}

func TestRestoreVMBesideRecordsACancelAsCancelled(t *testing.T) {
	restic, runs := &fakeRestic{restoreErr: context.Canceled}, &fakeRuns{}

	err := backup.RestoreVMBeside(context.Background(), backup.BesideVMDeps{
		RepoPath:   "/repo",
		SnapshotID: "abcd1234",
		Dirs:       []backup.VMRestoreDir{{Subtree: "/domains/win", Target: "/domains/win-restored"}},
		MakeDirs:   func() error { return nil },
		TargetID:   "vm-1",
		Restic:     restic,
		Runs:       runs,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if f := runs.finishOf(t, "run-1"); f.status != "cancelled" {
		t.Fatalf("run = %+v, want cancelled", f)
	}
}

func TestRestoreVMBesideRejectsABadSnapshotIDWithoutARun(t *testing.T) {
	runs := &fakeRuns{}
	err := backup.RestoreVMBeside(context.Background(), backup.BesideVMDeps{SnapshotID: "latest; rm -rf /", Runs: runs})
	if !errors.Is(err, backup.ErrInvalidSnapshotID) || runs.started != 0 {
		t.Fatalf("err = %v, runs started = %d", err, runs.started)
	}
}
