package store_test

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestRunsLifecycle(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, err := r.UpsertTarget(store.Target{ContainerName: "sonarr", AppdataPaths: []string{"/data"}})
	if err != nil {
		t.Fatalf("UpsertTarget: %v", err)
	}

	runID, err := r.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	snap := "abc123def456"
	bytes := int64(1024)
	if err := r.FinishRun(runID, "success", snap, bytes, ""); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}

	last, err := r.LastSuccessfulBackup(tg.ID)
	if err != nil {
		t.Fatalf("LastSuccessfulBackup: %v", err)
	}
	if last == nil {
		t.Fatal("expected a last successful backup run")
	}
	if last.SnapshotID != snap {
		t.Fatalf("snapshot_id mismatch: %q", last.SnapshotID)
	}
}

func TestRunsFinishFailed(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, _ := r.UpsertTarget(store.Target{ContainerName: "radarr", AppdataPaths: []string{"/data"}})
	runID, _ := r.StartRun(tg.ID, "backup")
	if err := r.FinishRun(runID, "failed", "", 0, "restic backup failed"); err != nil {
		t.Fatalf("FinishRun(failed): %v", err)
	}

	runs, err := r.ListRuns(10)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Error != "restic backup failed" {
		t.Fatalf("error not recorded: %q", runs[0].Error)
	}
}

// TestListRunsWithRunningRun covers a running or interrupted run, whose bytes
// are NULL because FinishRun never ran. ListRuns must return it instead of
// failing the scan.
func TestListRunsWithRunningRun(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, _ := r.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/data"}})
	if _, err := r.StartRun(tg.ID, "backup"); err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	runs, err := r.ListRuns(10)
	if err != nil {
		t.Fatalf("ListRuns must tolerate a NULL bytes (running/interrupted) run: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Status != "running" {
		t.Fatalf("expected running status, got %q", runs[0].Status)
	}
	if runs[0].Bytes != 0 {
		t.Fatalf("expected NULL bytes to map to 0, got %d", runs[0].Bytes)
	}
}

// TestFailRunningRunScopedToTarget checks that FailRunningRun fails the named
// target's running run and leaves another target's in-flight run alone, so
// panic recovery cannot fail an unrelated backup.
func TestFailRunningRunScopedToTarget(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	stuck, _ := r.UpsertTarget(store.Target{ContainerName: "stuck", AppdataPaths: []string{"/data"}})
	other, _ := r.UpsertTarget(store.Target{ContainerName: "other", AppdataPaths: []string{"/data"}})
	stuckRun, _ := r.StartRun(stuck.ID, "backup")
	otherRun, _ := r.StartRun(other.ID, "backup")

	n, err := r.FailRunningRun(stuck.ID, "internal error (recovered panic): boom")
	if err != nil {
		t.Fatalf("FailRunningRun: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row updated, got %d", n)
	}

	runs, err := r.ListRuns(10)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	byID := map[string]store.Run{}
	for _, run := range runs {
		byID[run.ID] = run
	}
	if got := byID[stuckRun]; got.Status != "failed" || got.FinishedAt == nil || got.Error != "internal error (recovered panic): boom" {
		t.Fatalf("stuck run not correctly failed: %+v", got)
	}
	if got := byID[otherRun]; got.Status != "running" {
		t.Fatalf("a different target's genuinely running run must be left untouched, got %+v", got)
	}

	// Nothing is left running for this target, so a second call changes nothing.
	if n, err := r.FailRunningRun(stuck.ID, "second call"); err != nil || n != 0 {
		t.Fatalf("re-calling FailRunningRun on an already-finished run should no-op, got n=%d err=%v", n, err)
	}
}

// TestReapInterruptedRuns expects the startup reap to fail orphaned running
// runs and leave finished ones alone.
func TestReapInterruptedRuns(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, _ := r.UpsertTarget(store.Target{ContainerName: "jellyfin", AppdataPaths: []string{"/data"}})
	orphan, _ := r.StartRun(tg.ID, "backup")
	done, _ := r.StartRun(tg.ID, "backup")
	if err := r.FinishRun(done, "success", "deadbeef", 1024, ""); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}

	n, err := r.ReapInterruptedRuns()
	if err != nil {
		t.Fatalf("ReapInterruptedRuns: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 reaped run, got %d", n)
	}

	runs, err := r.ListRuns(10)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	byID := map[string]store.Run{}
	for _, run := range runs {
		byID[run.ID] = run
	}
	if byID[orphan].Status != "failed" || byID[orphan].FinishedAt == nil {
		t.Fatalf("orphan run not reaped: %+v", byID[orphan])
	}
	if byID[done].Status != "success" {
		t.Fatalf("finished run must stay success, got %q", byID[done].Status)
	}
}

func TestRunsSince(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, _ := r.UpsertTarget(store.Target{ContainerName: "sonarr", AppdataPaths: []string{"/data"}})
	// StartRun stamps started_at with the current time, so only the cutoff varies.
	for i := 0; i < 3; i++ {
		if _, err := r.StartRun(tg.ID, "backup"); err != nil {
			t.Fatalf("StartRun: %v", err)
		}
	}
	now := time.Now().Unix()

	recent, err := r.RunsSince(now - 3600)
	if err != nil {
		t.Fatalf("RunsSince(past): %v", err)
	}
	if len(recent) != 3 {
		t.Fatalf("expected 3 runs in the window, got %d", len(recent))
	}

	none, err := r.RunsSince(now + 3600)
	if err != nil {
		t.Fatalf("RunsSince(future): %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected 0 runs before a future cutoff, got %d", len(none))
	}
}

// TestLastRunForTarget expects the newest backup run of the target whatever its
// status, ignoring other kinds and other targets, and nil when there are no
// runs.
func TestLastRunForTarget(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, err := r.UpsertTarget(store.Target{ContainerName: "sonarr", AppdataPaths: []string{"/data"}})
	if err != nil {
		t.Fatalf("UpsertTarget: %v", err)
	}
	other, err := r.UpsertTarget(store.Target{ContainerName: "radarr", AppdataPaths: []string{"/data"}})
	if err != nil {
		t.Fatalf("UpsertTarget: %v", err)
	}

	if run, err := r.LastRunForTarget(tg.ID); err != nil || run != nil {
		t.Fatalf("no runs yet: got (%+v, %v), want (nil, nil)", run, err)
	}

	// Explicit started_at stamps, since StartRun's one-second resolution would
	// make the order ambiguous. The last two rows are newer but must not win.
	now := time.Now().Unix()
	seed := []struct {
		id, target, kind, status string
		at                       int64
	}{
		{"run-old-success", tg.ID, "backup", "success", now - 300},
		{"run-newest-backup", tg.ID, "backup", "skipped", now - 200},
		{"run-newer-tamper", tg.ID, "tamper", "success", now - 100},
		{"run-other-target", other.ID, "backup", "failed", now - 50},
	}
	for _, s := range seed {
		if _, err := db.Exec(
			`INSERT INTO runs (id, target_id, kind, status, started_at) VALUES (?, ?, ?, ?, ?)`,
			s.id, s.target, s.kind, s.status, s.at,
		); err != nil {
			t.Fatalf("seed %s: %v", s.id, err)
		}
	}

	run, err := r.LastRunForTarget(tg.ID)
	if err != nil {
		t.Fatalf("LastRunForTarget: %v", err)
	}
	if run == nil || run.ID != "run-newest-backup" || run.Status != "skipped" {
		t.Fatalf("run = %+v, want the newest BACKUP run of this target (run-newest-backup, skipped)", run)
	}
}

func TestAcknowledgeRuns(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, _ := r.UpsertTarget(store.Target{ContainerName: "sonarr", AppdataPaths: []string{"/data"}})

	fail1, _ := r.StartRun(tg.ID, "backup")
	if err := r.FinishRun(fail1, "failed", "", 0, "restic backup failed"); err != nil {
		t.Fatalf("FinishRun(fail1): %v", err)
	}
	fail2, _ := r.StartRun(tg.ID, "backup")
	if err := r.FinishRun(fail2, "failed", "", 0, "restic backup failed"); err != nil {
		t.Fatalf("FinishRun(fail2): %v", err)
	}
	okRun, _ := r.StartRun(tg.ID, "backup")
	if err := r.FinishRun(okRun, "success", "snap", 1024, ""); err != nil {
		t.Fatalf("FinishRun(ok): %v", err)
	}

	ackFlag := func(id string) bool {
		runs, err := r.ListRuns(50)
		if err != nil {
			t.Fatalf("ListRuns: %v", err)
		}
		for _, run := range runs {
			if run.ID == id {
				return run.Acknowledged
			}
		}
		t.Fatalf("run %s not found", id)
		return false
	}

	if ackFlag(fail1) || ackFlag(fail2) || ackFlag(okRun) {
		t.Fatal("runs should start unacknowledged")
	}

	if n, err := r.AcknowledgeRuns(nil); err != nil || n != 0 {
		t.Fatalf("AcknowledgeRuns(nil) = (%d, %v), want (0, nil)", n, err)
	}

	n, err := r.AcknowledgeRuns([]string{fail1})
	if err != nil {
		t.Fatalf("AcknowledgeRuns: %v", err)
	}
	if n != 1 {
		t.Fatalf("AcknowledgeRuns rows = %d, want 1", n)
	}
	if !ackFlag(fail1) {
		t.Fatal("fail1 should be acknowledged")
	}
	if ackFlag(fail2) {
		t.Fatal("fail2 should NOT be acknowledged yet")
	}

	// Only fail2 is still unacknowledged.
	n, err = r.AcknowledgeAllFailed()
	if err != nil {
		t.Fatalf("AcknowledgeAllFailed: %v", err)
	}
	if n != 1 {
		t.Fatalf("AcknowledgeAllFailed rows = %d, want 1 (only fail2 unacked)", n)
	}
	if !ackFlag(fail2) {
		t.Fatal("fail2 should be acknowledged after AcknowledgeAllFailed")
	}
	if ackFlag(okRun) {
		t.Fatal("success run must not be acknowledged by AcknowledgeAllFailed")
	}
}

// TestLastSuccessfulBackupDomainScoped checks that a VM backup does not count
// for containers. Both domains record kind='backup'; which table holds
// target_id decides the domain.
func TestLastSuccessfulBackupDomainScoped(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	vm, err := r.UpsertVMTarget(store.VMTarget{Name: "ubuntu"})
	if err != nil {
		t.Fatalf("UpsertVMTarget: %v", err)
	}
	runID, err := r.StartRun(vm.ID, "backup")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if err := r.FinishRun(runID, "success", "vmsnap", 2048, ""); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}

	vmLast, err := r.LastSuccessfulVMBackup()
	if err != nil {
		t.Fatalf("LastSuccessfulVMBackup: %v", err)
	}
	if vmLast.IsZero() {
		t.Fatal("LastSuccessfulVMBackup should be non-zero after a VM backup")
	}

	cLast, err := r.LastSuccessfulContainerBackup()
	if err != nil {
		t.Fatalf("LastSuccessfulContainerBackup: %v", err)
	}
	if !cLast.IsZero() {
		t.Fatalf("LastSuccessfulContainerBackup should be zero (a VM backup must not satisfy the containers gate), got %v", cLast)
	}
}

// TestLastSuccessfulFilesBackupAndCounts expects a run against a file set to
// count for the files domain and in RunCounts, but not for containers.
func TestLastSuccessfulFilesBackupAndCounts(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	fs, err := r.CreateFileSet(store.FileSet{Name: "docs", Path: "user/documents", Enabled: true})
	if err != nil {
		t.Fatalf("CreateFileSet: %v", err)
	}
	ts, err := r.LastSuccessfulFilesBackup()
	if err != nil {
		t.Fatal(err)
	}
	if !ts.IsZero() {
		t.Fatalf("expected zero time before any files backup, got %v", ts)
	}

	id, err := r.StartRun(fs.ID, "backup")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if err := r.FinishRun(id, "success", "snap1", 100, ""); err != nil {
		t.Fatal(err)
	}
	ts, err = r.LastSuccessfulFilesBackup()
	if err != nil {
		t.Fatal(err)
	}
	if ts.IsZero() {
		t.Fatal("expected a last-success time for files")
	}
	counts, err := r.RunCounts()
	if err != nil {
		t.Fatal(err)
	}
	if counts["files"]["success"] != 1 {
		t.Fatalf("expected 1 files success, got %v", counts["files"])
	}
	cLast, err := r.LastSuccessfulContainerBackup()
	if err != nil {
		t.Fatal(err)
	}
	if !cLast.IsZero() {
		t.Fatalf("LastSuccessfulContainerBackup should be zero (a files backup must not satisfy the containers gate), got %v", cLast)
	}
}

// TestSetRunGroup expects SetRunGroup to stamp one run's group_id and leave
// other runs with an empty one.
func TestSetRunGroup(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, _ := r.UpsertTarget(store.Target{ContainerName: "sonarr", AppdataPaths: []string{"/data"}})

	grouped, err := r.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	ungrouped, err := r.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	if err := r.SetRunGroup(grouped, "parent-run-id"); err != nil {
		t.Fatalf("SetRunGroup: %v", err)
	}

	runs, err := r.ListRuns(10)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	byID := map[string]store.Run{}
	for _, run := range runs {
		byID[run.ID] = run
	}
	if byID[grouped].GroupID != "parent-run-id" {
		t.Fatalf("grouped run's GroupID not set: %+v", byID[grouped])
	}
	if byID[ungrouped].GroupID != "" {
		t.Fatalf("ungrouped run's GroupID must stay empty (zero-value default), got %+v", byID[ungrouped])
	}

	// An id that matches no row is not an error.
	if err := r.SetRunGroup("no-such-run", "some-group"); err != nil {
		t.Fatalf("SetRunGroup(unknown id) must not error: %v", err)
	}
}

// TestLastEverythingPass counts a finished pass whether it succeeded or failed,
// and not a pass in flight. The parent run fails whenever a single item does,
// so gating on success would let one persistently broken item rerun the whole
// pass every night.
func TestLastEverythingPass(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	ts, err := r.LastEverythingPass()
	if err != nil {
		t.Fatal(err)
	}
	if !ts.IsZero() {
		t.Fatalf("expected zero time before any everything pass, got %v", ts)
	}

	runningID, err := r.StartRun(store.EverythingTargetID, "backup")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	ts, err = r.LastEverythingPass()
	if err != nil {
		t.Fatal(err)
	}
	if !ts.IsZero() {
		t.Fatalf("a pass that has not finished must not satisfy the gate, got %v", ts)
	}

	// One item failed, so the pass finishes "failed", but it ran and the
	// interval starts here.
	if err := r.FinishRun(runningID, "failed", "", 0, "flash: not mounted"); err != nil {
		t.Fatal(err)
	}
	failedAt, err := r.LastEverythingPass()
	if err != nil {
		t.Fatal(err)
	}
	if failedAt.IsZero() {
		t.Fatal("a completed pass must satisfy the gate even when an item failed; otherwise the pass runs every night")
	}

	okID, err := r.StartRun(store.EverythingTargetID, "backup")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if err := r.FinishRun(okID, "success", "", 0, ""); err != nil {
		t.Fatal(err)
	}
	ts, err = r.LastEverythingPass()
	if err != nil {
		t.Fatal(err)
	}
	if ts.Before(failedAt) {
		t.Fatalf("expected the newest completed pass, got %v (older than %v)", ts, failedAt)
	}
}

// TestLastEverythingPassIgnoresAbandonedRuns covers a pass closed out by the
// startup reap or the panic path, which has a finished_at but did not complete.
// Counting it would let a reboot during the hours-long pass skip the next whole
// interval.
func TestLastEverythingPassIgnoresAbandonedRuns(t *testing.T) {
	for _, tc := range []struct {
		name    string
		abandon func(*testing.T, *store.Repo)
	}{
		{
			name: "reaped at startup",
			abandon: func(t *testing.T, r *store.Repo) {
				t.Helper()
				if _, err := r.ReapInterruptedRuns(); err != nil {
					t.Fatalf("ReapInterruptedRuns: %v", err)
				}
			},
		},
		{
			name: "closed out by the panic path",
			abandon: func(t *testing.T, r *store.Repo) {
				t.Helper()
				if _, err := r.FailRunningRun(store.EverythingTargetID, "panic: boom"); err != nil {
					t.Fatalf("FailRunningRun: %v", err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := store.OpenMem(t)
			if err := store.Migrate(db); err != nil {
				t.Fatal(err)
			}
			r := store.New(db)

			if _, err := r.StartRun(store.EverythingTargetID, "backup"); err != nil {
				t.Fatalf("StartRun: %v", err)
			}
			tc.abandon(t, r)

			// The row itself is closed, so the dashboard does not show it running.
			runs, err := r.ListRuns(10)
			if err != nil {
				t.Fatal(err)
			}
			if len(runs) != 1 || runs[0].FinishedAt == nil {
				t.Fatalf("expected exactly one closed-out run, got %+v", runs)
			}

			ts, err := r.LastEverythingPass()
			if err != nil {
				t.Fatal(err)
			}
			if !ts.IsZero() {
				t.Fatalf("an abandoned pass must not satisfy the everyN gate, got %v; the next interval of whole-server backups would be skipped", ts)
			}
		})
	}
}

// TestLastEverythingPassCountsPassAfterReap expects a pass that completes after
// an abandoned one to count.
func TestLastEverythingPassCountsPassAfterReap(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if _, err := r.StartRun(store.EverythingTargetID, "backup"); err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if _, err := r.ReapInterruptedRuns(); err != nil {
		t.Fatalf("ReapInterruptedRuns: %v", err)
	}

	id, err := r.StartRun(store.EverythingTargetID, "backup")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if err := r.FinishRun(id, "failed", "", 0, "flash: not mounted"); err != nil {
		t.Fatal(err)
	}

	ts, err := r.LastEverythingPass()
	if err != nil {
		t.Fatal(err)
	}
	if ts.IsZero() {
		t.Fatal("a pass that ran to completion must satisfy the gate even when an item failed, and even when an abandoned run precedes it")
	}
}

// TestLastSuccessfulConfigBackupAndCounts expects a run under ConfigTargetID to
// count for the config domain and in RunCounts.
func TestLastSuccessfulConfigBackupAndCounts(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	id, err := r.StartRun(store.ConfigTargetID, "backup")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if err := r.FinishRun(id, "success", "snap1", 100, ""); err != nil {
		t.Fatal(err)
	}
	ts, err := r.LastSuccessfulConfigBackup()
	if err != nil {
		t.Fatal(err)
	}
	if ts.IsZero() {
		t.Fatal("expected a last-success time for config")
	}
	counts, err := r.RunCounts()
	if err != nil {
		t.Fatal(err)
	}
	if counts["config"]["success"] != 1 {
		t.Fatalf("expected 1 config success, got %v", counts["config"])
	}
}

// insertRun writes a finished run directly, so a test can choose started_at and
// the insertion order that decides ties.
func insertRun(t *testing.T, db *sql.DB, id, targetID, kind, status string, startedAt int64, snapshotID, errMsg string) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO runs (id, target_id, kind, status, started_at, finished_at, snapshot_id, bytes, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?)`,
		id, targetID, kind, status, startedAt, startedAt+1, snapshotID, errMsg)
	if err != nil {
		t.Fatalf("insert run %s: %v", id, err)
	}
}

func TestRecentRunsOfKind(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	tg, err := r.UpsertTarget(store.Target{ContainerName: "pg"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := r.UpsertTarget(store.Target{ContainerName: "maria"})
	if err != nil {
		t.Fatal(err)
	}

	insertRun(t, db, "old", tg.ID, "dbdump", "success", 100, "s1", "")
	insertRun(t, db, "tie-a", tg.ID, "dbdump", "failed", 200, "", "boom")
	insertRun(t, db, "tie-b", tg.ID, "dbdump", "success", 200, "s2", "")
	insertRun(t, db, "backup", tg.ID, "backup", "success", 300, "s3", "")
	insertRun(t, db, "foreign", other.ID, "dbdump", "success", 400, "s4", "")
	if _, err := r.StartRun(tg.ID, "dbdump"); err != nil {
		t.Fatal(err)
	}

	got, err := r.RecentRunsOfKind(tg.ID, "dbdump", 10)
	if err != nil {
		t.Fatalf("RecentRunsOfKind: %v", err)
	}
	var ids []string
	for _, run := range got {
		ids = append(ids, run.ID)
	}
	want := []string{"tie-b", "tie-a", "old"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("RecentRunsOfKind = %v, want %v", ids, want)
	}

	got, err = r.RecentRunsOfKind(tg.ID, "dbdump", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "tie-b" {
		t.Fatalf("limit not honoured: %v", got)
	}

	got, err = r.RecentRunsOfKind(tg.ID, "dbimport", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("a kind without runs returned %v", got)
	}
}

func TestLastRunOfKind(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	tg, err := r.UpsertTarget(store.Target{ContainerName: "pg"})
	if err != nil {
		t.Fatal(err)
	}

	last, err := r.LastRunOfKind(tg.ID, "dbdump")
	if err != nil {
		t.Fatalf("LastRunOfKind: %v", err)
	}
	if last != nil {
		t.Fatalf("expected nil without any run, got %+v", last)
	}
	at, err := r.LastSuccessOfKind(tg.ID, "dbdump")
	if err != nil {
		t.Fatalf("LastSuccessOfKind: %v", err)
	}
	if at != 0 {
		t.Fatalf("LastSuccessOfKind = %d without any run, want 0", at)
	}

	insertRun(t, db, "ok", tg.ID, "dbdump", "success", 100, "s1", "")
	insertRun(t, db, "bad", tg.ID, "dbdump", "failed", 200, "", "boom")
	insertRun(t, db, "backup", tg.ID, "backup", "success", 300, "s3", "")

	last, err = r.LastRunOfKind(tg.ID, "dbdump")
	if err != nil {
		t.Fatal(err)
	}
	if last == nil || last.ID != "bad" {
		t.Fatalf("LastRunOfKind = %+v, want the failed dump", last)
	}

	at, err = r.LastSuccessOfKind(tg.ID, "dbdump")
	if err != nil {
		t.Fatal(err)
	}
	if at != 101 {
		t.Fatalf("LastSuccessOfKind = %d, want the successful dump's finished_at 101", at)
	}
}

func TestRunCountsOfKind(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	tg, err := r.UpsertTarget(store.Target{ContainerName: "pg"})
	if err != nil {
		t.Fatal(err)
	}

	insertRun(t, db, "d1", tg.ID, "dbdump", "success", 100, "s1", "")
	insertRun(t, db, "d2", tg.ID, "dbdump", "success", 200, "s2", "")
	insertRun(t, db, "d3", tg.ID, "dbdump", "failed", 300, "", "boom")
	insertRun(t, db, "b1", tg.ID, "backup", "failed", 400, "", "boom")
	if _, err := r.StartRun(tg.ID, "dbdump"); err != nil {
		t.Fatal(err)
	}

	counts, err := r.RunCountsOfKind("dbdump")
	if err != nil {
		t.Fatalf("RunCountsOfKind: %v", err)
	}
	if counts["containers"]["success"] != 2 || counts["containers"]["failed"] != 1 {
		t.Fatalf("counts = %v, want 2 success and 1 failed for containers", counts)
	}
}

func TestBackupSnapshotsOfRuns(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	tg, err := r.UpsertTarget(store.Target{ContainerName: "pg"})
	if err != nil {
		t.Fatal(err)
	}

	got, err := r.BackupSnapshotsOfRuns(nil)
	if err != nil {
		t.Fatalf("BackupSnapshotsOfRuns: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("an empty list returned %v", got)
	}

	insertRun(t, db, "ok", tg.ID, "backup", "success", 100, "snap-ok", "")
	insertRun(t, db, "failed", tg.ID, "backup", "failed", 200, "snap-failed", "boom")
	insertRun(t, db, "dump", tg.ID, "dbdump", "success", 300, "snap-dump", "")

	// More ids than fit in one IN clause, so the chunking is exercised.
	ids := []string{"ok", "failed", "dump"}
	for i := range 900 {
		ids = append(ids, fmt.Sprintf("absent-%d", i))
	}

	got, err = r.BackupSnapshotsOfRuns(ids)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"ok": "snap-ok"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BackupSnapshotsOfRuns = %v, want %v", got, want)
	}
}

func TestFailedDBDumpSnapshots(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	tg, err := r.UpsertTarget(store.Target{ContainerName: "pg"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := r.UpsertTarget(store.Target{ContainerName: "maria"})
	if err != nil {
		t.Fatal(err)
	}

	insertRun(t, db, "leftover", tg.ID, "dbdump", "failed", 100, "damaged", "boom")
	insertRun(t, db, "clean-failure", tg.ID, "dbdump", "failed", 200, "", "boom")
	insertRun(t, db, "good", tg.ID, "dbdump", "success", 300, "healthy", "")
	insertRun(t, db, "backup", tg.ID, "backup", "failed", 400, "other-snap", "boom")
	insertRun(t, db, "foreign", other.ID, "dbdump", "failed", 500, "not-mine", "boom")

	got, err := r.FailedDBDumpSnapshots(tg.ID)
	if err != nil {
		t.Fatalf("FailedDBDumpSnapshots: %v", err)
	}
	want := map[string]bool{"damaged": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FailedDBDumpSnapshots = %v, want %v", got, want)
	}
}

// runRow is a run written straight into the table, so a test can choose the
// stamps, the duration and the origin the queries under test read.
type runRow struct {
	id, targetID, kind, status string
	startedAt, finishedAt      int64
	via, viaKey                string
}

func insertRunRow(t *testing.T, db *sql.DB, row runRow) {
	t.Helper()
	var finished any
	if row.finishedAt != 0 {
		finished = row.finishedAt
	}
	_, err := db.Exec(`
		INSERT INTO runs (id, target_id, kind, status, started_at, finished_at, started_via, started_via_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		row.id, row.targetID, row.kind, row.status, row.startedAt, finished, row.via, row.viaKey)
	if err != nil {
		t.Fatalf("insert run %s: %v", row.id, err)
	}
}

func TestStartRunWithWritesMetaInOneInsert(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	tg, err := r.UpsertTarget(store.Target{ContainerName: "pg"})
	if err != nil {
		t.Fatal(err)
	}

	meta := store.RunMeta{GroupID: "g1", StartedVia: "mcp", StartedViaKey: "k1"}
	id, err := r.StartRunWith(tg.ID, "backup", meta)
	if err != nil {
		t.Fatalf("StartRunWith: %v", err)
	}

	check := func(label string, run *store.Run) {
		t.Helper()
		if run == nil {
			t.Fatalf("%s: no run", label)
		}
		if run.GroupID != "g1" || run.StartedVia != "mcp" || run.StartedViaKey != "k1" {
			t.Fatalf("%s: group %q, via %q/%q, want g1 and mcp/k1", label, run.GroupID, run.StartedVia, run.StartedViaKey)
		}
	}

	runs, err := r.ListRuns(10)
	if err != nil || len(runs) != 1 {
		t.Fatalf("ListRuns = %v, %v", runs, err)
	}
	check("ListRuns", &runs[0])

	last, err := r.LastRunForTarget(tg.ID)
	if err != nil {
		t.Fatalf("LastRunForTarget: %v", err)
	}
	check("LastRunForTarget", last)

	if err := r.FinishRun(id, "success", "snap", 1, ""); err != nil {
		t.Fatal(err)
	}
	success, err := r.LastSuccessfulBackup(tg.ID)
	if err != nil {
		t.Fatalf("LastSuccessfulBackup: %v", err)
	}
	check("LastSuccessfulBackup", success)

	since, err := r.RunsSince(0)
	if err != nil || len(since) != 1 {
		t.Fatalf("RunsSince = %v, %v", since, err)
	}
	check("RunsSince", &since[0])

	pruneID, err := r.StartRun(tg.ID, "prune")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	runs, err = r.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	var plain store.Run
	for _, run := range runs {
		if run.ID == pruneID {
			plain = run
		}
	}
	if plain.ID == "" {
		t.Fatal("ListRuns does not carry the run StartRun just wrote")
	}
	if plain.GroupID != "" || plain.StartedVia != "" || plain.StartedViaKey != "" {
		t.Fatalf("StartRun stamped %q/%q/%q, want all empty", plain.GroupID, plain.StartedVia, plain.StartedViaKey)
	}
}

func TestListRunsFilteredByTargetsKindsStatusesSince(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	rows := []runRow{
		{id: "a1", targetID: "a", kind: "backup", status: "success", startedAt: 100, finishedAt: 110},
		{id: "a2", targetID: "a", kind: "dbdump", status: "failed", startedAt: 200, finishedAt: 210},
		{id: "b1", targetID: "b", kind: "backup", status: "running", startedAt: 300},
		{id: "b2", targetID: "b", kind: "backup", status: "success", startedAt: 400, finishedAt: 410},
	}
	for _, row := range rows {
		insertRunRow(t, db, row)
	}

	ids := func(f store.RunFilter) []string {
		t.Helper()
		got, err := r.ListRunsFiltered(f)
		if err != nil {
			t.Fatalf("ListRunsFiltered(%+v): %v", f, err)
		}
		out := make([]string, len(got))
		for i, run := range got {
			out[i] = run.ID
		}
		return out
	}

	cases := []struct {
		name   string
		filter store.RunFilter
		want   []string
	}{
		{"everything, newest first", store.RunFilter{}, []string{"b2", "b1", "a2", "a1"}},
		{"by target", store.RunFilter{TargetIDs: []string{"a"}}, []string{"a2", "a1"}},
		{"by kind", store.RunFilter{Kinds: []string{"dbdump"}}, []string{"a2"}},
		{"by status", store.RunFilter{Statuses: []string{"success"}}, []string{"b2", "a1"}},
		{"since", store.RunFilter{Since: 200}, []string{"b2", "b1", "a2"}},
		{"limit", store.RunFilter{Limit: 2}, []string{"b2", "b1"}},
		{
			"combined",
			store.RunFilter{TargetIDs: []string{"a", "b"}, Kinds: []string{"backup"}, Statuses: []string{"success"}, Since: 200},
			[]string{"b2"},
		},
	}
	for _, c := range cases {
		if got := ids(c.filter); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLatestBackupsByTargetIgnoresInsaneStamps(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	now := time.Now().Unix()
	future := time.Now().AddDate(9, 0, 0).Unix()
	rows := []runRow{
		{id: "a-old", targetID: "a", kind: "backup", status: "success", startedAt: now - 900, finishedAt: now - 880},
		{id: "a-good", targetID: "a", kind: "backup", status: "success", startedAt: now - 600, finishedAt: now - 570},
		{id: "a-fail", targetID: "a", kind: "backup", status: "failed", startedAt: now - 300, finishedAt: now - 290},
		{id: "a-future", targetID: "a", kind: "backup", status: "success", startedAt: now - 100, finishedAt: future},
		{id: "a-dump", targetID: "a", kind: "dbdump", status: "success", startedAt: now - 60, finishedAt: now - 50},
		{id: "b-fail", targetID: "b", kind: "backup", status: "failed", startedAt: now - 200, finishedAt: now - 190},
		{id: "c-running", targetID: "c", kind: "backup", status: "running", startedAt: now - 10},
	}
	for _, row := range rows {
		insertRunRow(t, db, row)
	}

	got, err := r.LatestBackupsByTarget()
	if err != nil {
		t.Fatalf("LatestBackupsByTarget: %v", err)
	}
	want := map[string]store.BackupStamp{
		"a": {LastSuccessAt: now - 570, LastDurationSeconds: 30, LastRunAt: now - 300, LastRunStatus: "failed"},
		"b": {LastRunAt: now - 200, LastRunStatus: "failed"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LatestBackupsByTarget = %+v, want %+v", got, want)
	}
}

func TestGetRun(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	insertRunRow(t, db, runRow{
		id: "r1", targetID: "t1", kind: "backup", status: "success",
		startedAt: 100, finishedAt: 160, via: "mcp", viaKey: "k1",
	})
	_, err := db.Exec(`UPDATE runs SET snapshot_id = 'snap', bytes = 42, error = 'note', acknowledged = 1, group_id = 'g1' WHERE id = 'r1'`)
	if err != nil {
		t.Fatal(err)
	}

	run, err := r.GetRun("r1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	finished := int64(160)
	want := store.Run{
		ID: "r1", TargetID: "t1", Kind: "backup", Status: "success",
		StartedAt: 100, FinishedAt: &finished, SnapshotID: "snap", Bytes: 42, Error: "note",
		Acknowledged: true, GroupID: "g1", StartedVia: "mcp", StartedViaKey: "k1",
	}
	if run.FinishedAt == nil || *run.FinishedAt != finished {
		t.Fatalf("FinishedAt = %v, want %d", run.FinishedAt, finished)
	}
	run.FinishedAt = want.FinishedAt
	if !reflect.DeepEqual(run, want) {
		t.Fatalf("GetRun = %+v, want %+v", run, want)
	}

	if _, err := r.GetRun("nope"); !errors.Is(err, store.ErrRunNotFound) {
		t.Fatalf("GetRun of an unknown id = %v, want ErrRunNotFound", err)
	}
}

func TestMCPStartQueries(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	rows := []runRow{
		{id: "web", targetID: "a", kind: "backup", status: "success", startedAt: 100, finishedAt: 110},
		{id: "mcp1", targetID: "a", kind: "backup", status: "success", startedAt: 200, finishedAt: 210, via: "mcp", viaKey: "k1"},
		{id: "mcp2", targetID: "a", kind: "backup", status: "running", startedAt: 300, via: "mcp", viaKey: "k2"},
		{id: "mcp3", targetID: "a", kind: "backup", status: "failed", startedAt: 400, finishedAt: 410, via: "mcp", viaKey: "k1"},
		{id: "mcp4", targetID: "a", kind: "dbdump", status: "success", startedAt: 500, finishedAt: 510, via: "mcp", viaKey: "k1"},
		{id: "mcp5", targetID: "b", kind: "backup", status: "success", startedAt: 600, finishedAt: 610, via: "mcp", viaKey: "k1"},
		{id: "mcp6", targetID: "c", kind: "backup", status: "running", startedAt: 700, via: "mcp", viaKey: "k1"},
	}
	for _, row := range rows {
		insertRunRow(t, db, row)
	}

	latest := []struct {
		name    string
		targets []string
		since   int64
		want    int64
	}{
		{"newest mcp start of the target", []string{"a"}, 0, 500},
		{"a run still in flight does not count", []string{"c"}, 0, 0},
		{"several targets", []string{"a", "b"}, 0, 600},
		{"since cuts the older starts", []string{"a"}, 501, 0},
		{"a target without mcp runs", []string{"d"}, 0, 0},
		{"no targets", nil, 0, 0},
	}
	for _, c := range latest {
		got, err := r.LatestMCPStartAt(c.targets, c.since)
		if err != nil {
			t.Fatalf("LatestMCPStartAt(%v, %d): %v", c.targets, c.since, err)
		}
		if got != c.want {
			t.Fatalf("%s: LatestMCPStartAt = %d, want %d", c.name, got, c.want)
		}
	}

	counts := []struct {
		target     string
		since      int64
		want       int
		wantOldest int64
	}{
		{"a", 0, 2, 200},
		{"a", 250, 1, 300},
		{"b", 0, 1, 600},
		{"c", 0, 1, 700},
		{"d", 0, 0, 0},
	}
	for _, c := range counts {
		got, oldest, err := r.MCPBackupsSince(c.target, c.since)
		if err != nil {
			t.Fatalf("MCPBackupsSince(%s, %d): %v", c.target, c.since, err)
		}
		if got != c.want || oldest != c.wantOldest {
			t.Fatalf("MCPBackupsSince(%s, %d) = %d/%d, want %d/%d", c.target, c.since, got, oldest, c.want, c.wantOldest)
		}
	}

	origins := []struct {
		name          string
		kind          string
		n             int
		total, viaMCP int
	}{
		{"newest backup came through mcp", "backup", 1, 1, 1},
		{"the one before it did not", "backup", 2, 2, 1},
		{"there are only two successful backups", "backup", 5, 2, 1},
		{"dumps are counted as their own series", "dbdump", 5, 1, 1},
	}
	for _, c := range origins {
		total, viaMCP, err := r.NewestBackupOrigins("a", c.kind, c.n)
		if err != nil {
			t.Fatalf("NewestBackupOrigins(a, %s, %d): %v", c.kind, c.n, err)
		}
		if total != c.total || viaMCP != c.viaMCP {
			t.Fatalf("%s: NewestBackupOrigins(a, %s, %d) = %d/%d, want %d/%d", c.name, c.kind, c.n, total, viaMCP, c.total, c.viaMCP)
		}
	}
}

func TestLastRunsOfKind(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	rows := []runRow{
		{id: "a-old", targetID: "a", kind: "dbdump", status: "success", startedAt: 100, finishedAt: 110},
		{id: "a-new", targetID: "a", kind: "dbdump", status: "failed", startedAt: 200, finishedAt: 210},
		{id: "a-running", targetID: "a", kind: "dbdump", status: "running", startedAt: 300},
		{id: "a-backup", targetID: "a", kind: "backup", status: "success", startedAt: 400, finishedAt: 410},
		{id: "b-dump", targetID: "b", kind: "dbdump", status: "success", startedAt: 150, finishedAt: 160, via: "mcp", viaKey: "k1"},
	}
	for _, row := range rows {
		insertRunRow(t, db, row)
	}

	got, err := r.LastRunsOfKind("dbdump")
	if err != nil {
		t.Fatalf("LastRunsOfKind: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("LastRunsOfKind returned %d targets, want 2", len(got))
	}
	if got["a"].ID != "a-new" {
		t.Fatalf("target a got run %q, want the newest finished dump a-new", got["a"].ID)
	}
	if got["b"].ID != "b-dump" || got["b"].StartedVia != "mcp" {
		t.Fatalf("target b got %+v, want b-dump with its origin", got["b"])
	}
}
