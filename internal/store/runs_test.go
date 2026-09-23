package store_test

import (
	"database/sql"
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

func seriesTarget(t *testing.T, r *store.Repo, name string) store.Target {
	t.Helper()
	tg, err := r.UpsertTarget(store.Target{ContainerName: name})
	if err != nil {
		t.Fatalf("UpsertTarget %s: %v", name, err)
	}
	return tg
}

func seriesByID(t *testing.T, r *store.Repo, targetID, kind string) map[string]store.SeriesRun {
	t.Helper()
	series, err := r.ItemSeries(targetID, kind, 1<<40, 90)
	if err != nil {
		t.Fatalf("ItemSeries: %v", err)
	}
	byID := make(map[string]store.SeriesRun, len(series))
	for _, run := range series {
		byID[run.ID] = run
	}
	return byID
}

func TestFinishRunMeasuredWritesMetricsInOneUpdate(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	tg := seriesTarget(t, r, "sonarr")

	parent := true
	runID, err := r.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	m := &store.RunMetrics{SourceBytes: 4096, SourceFiles: 12, FilesNew: 3, ResticMS: 2100, HasParent: &parent}
	if err := r.FinishRunMeasured(runID, "success", "snap", 512, "", m, "fp-1"); err != nil {
		t.Fatalf("FinishRunMeasured: %v", err)
	}

	got := seriesByID(t, r, tg.ID, "backup")[runID]
	if got.Status != "success" || got.SnapshotID != "snap" || got.Bytes != 512 {
		t.Fatalf("the finish itself did not land: %+v", got)
	}
	if got.SourceBytes == nil || *got.SourceBytes != 4096 {
		t.Fatalf("source_bytes = %v", got.SourceBytes)
	}
	if got.SourceFiles == nil || *got.SourceFiles != 12 {
		t.Fatalf("source_files = %v", got.SourceFiles)
	}
	if got.FilesNew == nil || *got.FilesNew != 3 {
		t.Fatalf("files_new = %v", got.FilesNew)
	}
	if got.ResticMS == nil || *got.ResticMS != 2100 {
		t.Fatalf("restic_ms = %v", got.ResticMS)
	}
	if got.HasParent == nil || *got.HasParent != 1 {
		t.Fatalf("has_parent = %v", got.HasParent)
	}
	if got.SelectionFP == nil || *got.SelectionFP != "fp-1" {
		t.Fatalf("selection_fp = %v", got.SelectionFP)
	}

	bareRun, err := r.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishRunMeasured(bareRun, "failed", "", 0, "boom", nil, ""); err != nil {
		t.Fatalf("FinishRunMeasured(nil metrics): %v", err)
	}
	if bare := seriesByID(t, r, tg.ID, "backup")[bareRun]; bare.SourceBytes != nil || bare.HasParent != nil || bare.SelectionFP != nil {
		t.Fatalf("nil metrics wrote values: %+v", bare)
	}

	plainRun, err := r.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishRun(plainRun, "success", "snap", 8, ""); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}
	if left := seriesByID(t, r, tg.ID, "backup")[plainRun]; left.SourceBytes != nil || left.SelectionFP != nil {
		t.Fatalf("FinishRun wrote metric columns: %+v", left)
	}

	if err := r.FinishRunMeasured("absent", "success", "snap", 0, "", m, "fp"); err == nil {
		t.Fatal("FinishRunMeasured accepted an unknown run id")
	}
}

func TestRunFinishedHookFires(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	tg := seriesTarget(t, r, "sonarr")

	unwatched, err := r.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishRun(unwatched, "success", "snap", 1, ""); err != nil {
		t.Fatalf("FinishRun without a hook: %v", err)
	}

	var seen []store.RunFinished
	r.SetRunFinishedHook(func(f store.RunFinished) { seen = append(seen, f) })

	plain, err := r.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishRun(plain, "success", "snap", 1, ""); err != nil {
		t.Fatal(err)
	}
	measured, err := r.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishRunMeasured(measured, "success", "snap", 1, "", &store.RunMetrics{SourceBytes: 1}, "fp"); err != nil {
		t.Fatal(err)
	}
	want := []store.RunFinished{{RunID: plain}, {RunID: measured}}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("after two finishes the hook saw %+v, want %+v", seen, want)
	}

	seen = nil
	if err := r.FinishRun("absent", "success", "", 0, ""); err == nil {
		t.Fatal("FinishRun accepted an unknown id")
	}
	if len(seen) != 0 {
		t.Fatalf("a finish that matched no row fired the hook: %+v", seen)
	}

	if n, err := r.FailRunningRun(tg.ID, "boom"); err != nil || n != 0 {
		t.Fatalf("FailRunningRun with nothing running = %d, %v", n, err)
	}
	if len(seen) != 0 {
		t.Fatalf("FailRunningRun fired the hook without changing a row: %+v", seen)
	}

	if _, err := r.StartRun(tg.ID, "backup"); err != nil {
		t.Fatal(err)
	}
	if n, err := r.FailRunningRun(tg.ID, "boom"); err != nil || n != 1 {
		t.Fatalf("FailRunningRun = %d, %v", n, err)
	}
	if !reflect.DeepEqual(seen, []store.RunFinished{{TargetID: tg.ID}}) {
		t.Fatalf("FailRunningRun told the hook %+v", seen)
	}
}

func TestItemSeriesOrderAndFilter(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	tg := seriesTarget(t, r, "sonarr")
	other := seriesTarget(t, r, "radarr")

	insertRun(t, db, "old", tg.ID, "backup", "success", 100, "s1", "")
	insertRun(t, db, "tie-first", tg.ID, "backup", "failed", 200, "", "boom")
	insertRun(t, db, "tie-second", tg.ID, "backup", "success", 200, "s2", "")
	insertRun(t, db, "beyond", tg.ID, "backup", "success", 900, "s3", "")
	insertRun(t, db, "running", tg.ID, "backup", "running", 150, "", "")
	insertRun(t, db, "cancelled", tg.ID, "backup", "cancelled", 150, "", "")
	insertRun(t, db, "dump", tg.ID, "dbdump", "success", 150, "s4", "")
	insertRun(t, db, "foreign", other.ID, "backup", "success", 150, "s5", "")

	series, err := r.ItemSeries(tg.ID, "backup", 300, 90)
	if err != nil {
		t.Fatalf("ItemSeries: %v", err)
	}
	var ids []string
	for _, run := range series {
		ids = append(ids, run.ID)
	}
	if want := []string{"tie-second", "tie-first", "old"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ItemSeries = %v, want %v", ids, want)
	}

	dumps, err := r.ItemSeries(tg.ID, "dbdump", 300, 90)
	if err != nil {
		t.Fatal(err)
	}
	if len(dumps) != 1 || dumps[0].ID != "dump" {
		t.Fatalf("the dump series is %v", dumps)
	}

	limited, err := r.ItemSeries(tg.ID, "backup", 300, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 2 {
		t.Fatalf("limit ignored: %d rows", len(limited))
	}
}

func TestNewDataWindowKeepsEligibleRunsInsideTheWindow(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	tg := seriesTarget(t, r, "sonarr")

	insertRun(t, db, "before", tg.ID, "backup", "success", 50, "s0", "")
	insertRun(t, db, "inside", tg.ID, "backup", "success", 150, "s1", "")
	insertRun(t, db, "newer", tg.ID, "backup", "success", 250, "s2", "")
	insertRun(t, db, "after", tg.ID, "backup", "success", 900, "s3", "")
	insertRun(t, db, "failed", tg.ID, "backup", "failed", 200, "", "boom")
	insertRun(t, db, "bookkeeping", tg.ID, "backup", "success", 220, "", "")

	window, err := r.NewDataWindow(tg.ID, "backup", 100, 300, 1000)
	if err != nil {
		t.Fatalf("NewDataWindow: %v", err)
	}
	var ids []string
	for _, run := range window {
		ids = append(ids, run.ID)
	}
	if want := []string{"newer", "inside"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("NewDataWindow = %v, want %v", ids, want)
	}

	capped, err := r.NewDataWindow(tg.ID, "backup", 100, 300, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(capped) != 1 || capped[0].ID != "newer" {
		t.Fatalf("limit ignored: %v", capped)
	}
}

func TestFirstEligibleRunIDSkipsSnapshotlessUnmeasuredSuccess(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	tg := seriesTarget(t, r, "sonarr")

	insertRun(t, db, "bookkeeping", tg.ID, "backup", "success", 100, "", "")
	insertRun(t, db, "failed", tg.ID, "backup", "failed", 110, "", "boom")
	insertRun(t, db, "empty-measured", tg.ID, "backup", "success", 120, "", "")
	if _, err := db.Exec(`UPDATE runs SET source_bytes = 0, source_files = 0 WHERE id = 'empty-measured'`); err != nil {
		t.Fatal(err)
	}
	insertRun(t, db, "with-snapshot", tg.ID, "backup", "success", 130, "s1", "")

	first, err := r.FirstEligibleRunID(tg.ID, "backup")
	if err != nil {
		t.Fatalf("FirstEligibleRunID: %v", err)
	}
	if first != "empty-measured" {
		t.Fatalf("FirstEligibleRunID = %q, want the measured empty run", first)
	}

	if first, err = r.FirstEligibleRunID(tg.ID, "dbdump"); err != nil || first != "" {
		t.Fatalf("FirstEligibleRunID for a kind without runs = %q, %v", first, err)
	}

	insertRun(t, db, "dump", tg.ID, "dbdump", "success", 90, "s2", "")
	if first, err = r.FirstEligibleRunID(tg.ID, "dbdump"); err != nil || first != "dump" {
		t.Fatalf("the dump series = %q, %v", first, err)
	}
}

func TestRunTargetsResolvesEveryKnownRunID(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	tg := seriesTarget(t, r, "sonarr")

	insertRun(t, db, "backup", tg.ID, "backup", "success", 100, "s1", "")
	insertRun(t, db, "dump", tg.ID, "dbdump", "success", 110, "s2", "")

	// More ids than fit in one IN clause, so the chunking is exercised.
	ids := []string{"backup", "dump", "absent"}
	for i := range 900 {
		ids = append(ids, fmt.Sprintf("absent-%d", i))
	}
	got, err := r.RunTargets(ids)
	if err != nil {
		t.Fatalf("RunTargets: %v", err)
	}
	want := map[string]store.RunTargetKind{
		"backup": {TargetID: tg.ID, Kind: "backup"},
		"dump":   {TargetID: tg.ID, Kind: "dbdump"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RunTargets = %v, want %v", got, want)
	}

	empty, err := r.RunTargets(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("an empty list resolved to %v", empty)
	}
}

func TestUnmeasuredSnapshotRunsFindsWhatTheBackfillCanFill(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	tg := seriesTarget(t, r, "sonarr")

	insertRun(t, db, "wanted", tg.ID, "backup", "success", 100, "s1", "")
	insertRun(t, db, "dump", tg.ID, "dbdump", "success", 110, "s2", "")
	insertRun(t, db, "measured", tg.ID, "backup", "success", 120, "s3", "")
	if _, err := db.Exec(`UPDATE runs SET source_bytes = 7 WHERE id = 'measured'`); err != nil {
		t.Fatal(err)
	}
	insertRun(t, db, "no-snapshot", tg.ID, "backup", "success", 130, "", "")
	insertRun(t, db, "failed", tg.ID, "backup", "failed", 140, "s4", "boom")
	insertRun(t, db, "restore", tg.ID, "restore", "success", 150, "s5", "")

	runs, err := r.UnmeasuredSnapshotRuns()
	if err != nil {
		t.Fatalf("UnmeasuredSnapshotRuns: %v", err)
	}
	want := []store.UnmeasuredRun{
		{ID: "wanted", TargetID: tg.ID, Kind: "backup", SnapshotID: "s1", StartedAt: 100},
		{ID: "dump", TargetID: tg.ID, Kind: "dbdump", SnapshotID: "s2", StartedAt: 110},
	}
	if !reflect.DeepEqual(runs, want) {
		t.Fatalf("UnmeasuredSnapshotRuns = %+v, want %+v", runs, want)
	}
}

func TestSetRunMetricsNeverOverwritesLiveMeasurement(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	tg := seriesTarget(t, r, "sonarr")
	vm := seriesTarget(t, r, "win11")

	live, err := r.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishRunMeasured(live, "success", "s1", 100, "", &store.RunMetrics{SourceBytes: 4096, SourceFiles: 9}, "fp"); err != nil {
		t.Fatal(err)
	}
	insertRun(t, db, "stale", tg.ID, "backup", "success", 100, "s2", "")
	insertRun(t, db, "vm", vm.ID, "backup", "success", 100, "s3", "")

	summed := int64(999)
	parent := true
	n, err := r.SetRunMetrics(map[string]store.BackfillMetrics{
		live: {RunMetrics: store.RunMetrics{SourceBytes: 1, SourceFiles: 1}},
		"stale": {RunMetrics: store.RunMetrics{
			SourceBytes: 2048, SourceFiles: 5, FilesNew: 2, ResticMS: 700, HasParent: &parent,
		}},
		"vm":     {RunMetrics: store.RunMetrics{SourceBytes: 8192}, Bytes: &summed},
		"absent": {RunMetrics: store.RunMetrics{SourceBytes: 3}},
	})
	if err != nil {
		t.Fatalf("SetRunMetrics: %v", err)
	}
	if n != 2 {
		t.Fatalf("SetRunMetrics set %d rows, want 2", n)
	}

	byID := seriesByID(t, r, tg.ID, "backup")
	if got := byID[live]; got.SourceBytes == nil || *got.SourceBytes != 4096 || got.SourceFiles == nil || *got.SourceFiles != 9 {
		t.Fatalf("the live measurement was overwritten: %+v", got)
	}
	filled := byID["stale"]
	if filled.SourceBytes == nil || *filled.SourceBytes != 2048 || filled.FilesNew == nil || *filled.FilesNew != 2 {
		t.Fatalf("the unmeasured row was not filled: %+v", filled)
	}
	if filled.ResticMS == nil || *filled.ResticMS != 700 || filled.HasParent == nil || *filled.HasParent != 1 {
		t.Fatalf("duration or parent missing: %+v", filled)
	}

	vmRuns, err := r.ItemSeries(vm.ID, "backup", 1<<40, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(vmRuns) != 1 || vmRuns[0].Bytes != summed {
		t.Fatalf("the VM run's bytes are %+v, want %d", vmRuns, summed)
	}
}
