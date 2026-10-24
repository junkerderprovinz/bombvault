package backup_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// exitErr is a hook that ended with a code, the way dockercli reports it.
type exitErr struct{ code int }

func (e exitErr) Error() string  { return fmt.Sprintf("hook exited %d: streams active", e.code) }
func (e exitErr) ExitCode() int  { return e.code }
func (e exitErr) Output() string { return "streams active" }

func deferDeps(d *fakeDocker, r *fakeRestic, runs *fakeRuns) backup.BackupDeps {
	return backup.BackupDeps{
		ContainerRef: "plex", ContainerName: "plex", RepoPath: "/repo",
		AppdataPaths: []string{"/host/user/appdata/plex"}, TargetID: "t1",
		WasRunning: true, PreHook: "check-streams",
		Docker: d, Restic: r, Templates: &fakeTemplates{}, Runs: runs,
	}
}

func TestAPreHookExitingWithTempFailDefersTheBackup(t *testing.T) {
	d := &fakeDocker{execErr: exitErr{backup.ExitTempFail}}
	r := &fakeRestic{}
	runs := &fakeRuns{}
	_, err := backup.BackupContainer(t.Context(), deferDeps(d, r, runs))
	reason, deferred := backup.IsDeferred(err)
	if !deferred {
		t.Fatalf("err = %v, want a deferral", err)
	}
	if reason != store.ReasonDeferredHook+": streams active" {
		t.Fatalf("reason = %q", reason)
	}
	if contains(d.log, "stop:plex") || len(r.log) != 0 {
		t.Fatalf("a deferred backup stopped or read something: docker %v, restic %v", d.log, r.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != backup.StatusDeferred {
		t.Fatalf("runs = %v, want one deferred", runs.finishes)
	}
}

func TestAPreHookExitingWithAnyOtherCodeStillFails(t *testing.T) {
	for _, code := range []int{1, 2, 74, 76} {
		d := &fakeDocker{execErr: exitErr{code}}
		runs := &fakeRuns{}
		_, err := backup.BackupContainer(t.Context(), deferDeps(d, &fakeRestic{}, runs))
		if _, deferred := backup.IsDeferred(err); deferred || err == nil {
			t.Fatalf("exit %d: err = %v, want a plain failure", code, err)
		}
		if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
			t.Fatalf("exit %d: runs = %v, want one failed", code, runs.finishes)
		}
	}
}

func TestABusyAppDefersTheBackupBeforeTheHookRuns(t *testing.T) {
	d := &fakeDocker{}
	runs := &fakeRuns{}
	deps := deferDeps(d, &fakeRestic{}, runs)
	deps.Busy = func(context.Context) string { return "Plex has 2 active sessions" }
	_, err := backup.BackupContainer(t.Context(), deps)
	if reason, deferred := backup.IsDeferred(err); !deferred || reason != "Plex has 2 active sessions" {
		t.Fatalf("err = %v", err)
	}
	if len(d.log) != 0 {
		t.Fatalf("the hook or a stop ran for a busy app: %v", d.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != backup.StatusDeferred {
		t.Fatalf("runs = %v", runs.finishes)
	}
}

func TestAStoppedContainerIsNotAskedWhetherItIsBusy(t *testing.T) {
	deps := deferDeps(&fakeDocker{}, &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}, &fakeRuns{})
	deps.WasRunning = false
	deps.Busy = func(context.Context) string {
		t.Error("a stopped container was asked")
		return "busy"
	}
	if _, err := backup.BackupContainer(t.Context(), deps); err != nil {
		t.Fatal(err)
	}
}

func TestABackupRegardlessRunsThroughABusyApp(t *testing.T) {
	d := &fakeDocker{execErr: exitErr{backup.ExitTempFail}}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}
	runs := &fakeRuns{}
	deps := deferDeps(d, r, runs)
	deps.Regardless = true
	deps.Busy = func(context.Context) string {
		t.Error("the activity check was asked of a backup that runs regardless")
		return "busy"
	}
	if _, err := backup.BackupContainer(t.Context(), deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !contains(d.log, "stop:plex") || len(r.log) == 0 {
		t.Fatalf("the backup did not run: docker %v, restic %v", d.log, r.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("runs = %v", runs.finishes)
	}
}

func TestADeferralIsNoFailureToTheScheduler(t *testing.T) {
	err := fmt.Errorf("round: %w", &backup.DeferredError{Reason: "busy"})
	var d interface{ Deferred() bool }
	if !errors.As(err, &d) || !d.Deferred() {
		t.Fatal("a wrapped deferral does not say it is one")
	}
}
