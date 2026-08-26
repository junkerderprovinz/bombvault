package store_test

import (
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

// TestListRunsWithRunningRun guards the dashboard's "Failed to load runs"
// regression: a run still in flight (or interrupted mid-backup) has a NULL
// `bytes` column because StartRun never sets it and FinishRun was never reached.
// ListRuns must return such a row instead of failing the whole scan on the NULL.
func TestListRunsWithRunningRun(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, _ := r.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/data"}})
	// StartRun only — simulates a backup that is running or was interrupted, so
	// the row keeps bytes = NULL (FinishRun, which sets bytes, never ran).
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

// TestFailRunningRunScopedToTarget verifies FailRunningRun closes out ONLY the
// named target's running run, as 'failed' with the given error, and never
// touches a different target's genuinely in-flight run — the property that
// makes it safe to call from a recovered panic (api.Service.failStuckRun)
// without accidentally failing an unrelated domain's concurrent backup.
func TestFailRunningRunScopedToTarget(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	stuck, _ := r.UpsertTarget(store.Target{ContainerName: "stuck", AppdataPaths: []string{"/data"}})
	other, _ := r.UpsertTarget(store.Target{ContainerName: "other", AppdataPaths: []string{"/data"}})
	stuckRun, _ := r.StartRun(stuck.ID, "backup")
	otherRun, _ := r.StartRun(other.ID, "backup") // still genuinely running elsewhere

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

	// Calling it again (nothing left running for this target) is a harmless no-op.
	if n, err := r.FailRunningRun(stuck.ID, "second call"); err != nil || n != 0 {
		t.Fatalf("re-calling FailRunningRun on an already-finished run should no-op, got n=%d err=%v", n, err)
	}
}

// TestReapInterruptedRuns verifies a startup reap turns orphaned 'running' runs
// into 'failed' (with a finished_at) while leaving completed runs untouched.
func TestReapInterruptedRuns(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, _ := r.UpsertTarget(store.Target{ContainerName: "jellyfin", AppdataPaths: []string{"/data"}})
	// One orphaned (running) + one cleanly finished run.
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

// TestRunsSince verifies the time-windowed query: runs at or after the cutoff
// are returned (newest first) and older runs are excluded. Powers the
// dashboard's backup-health heatmap window.
func TestRunsSince(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, _ := r.UpsertTarget(store.Target{ContainerName: "sonarr", AppdataPaths: []string{"/data"}})
	// StartRun stamps started_at = now, so every seeded run is recent. The cutoff
	// is what we vary: a cutoff in the future excludes them, one in the past keeps
	// them.
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

// TestLastRunForTarget pins the skip-warning debounce query (#111): it must
// return the most recent BACKUP run regardless of status (here a "skipped" run
// recorded after a success), ignore other run kinds (a newer tamper run must not
// mask the backup history) and other targets' runs, and report nil when the
// target has no backup runs at all.
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

	// Seed with explicit started_at stamps (StartRun's second granularity would
	// make same-second ordering ambiguous): a success, then a newer skip, then an
	// even newer non-backup run and a newer run of ANOTHER target — neither of the
	// last two may win.
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

// TestAcknowledgeRuns verifies both acknowledge paths (#126): AcknowledgeRuns
// flips the flag for the given ids only and reports the right rows-affected
// count (a no-op on an empty list), and AcknowledgeAllFailed acknowledges every
// still-unacknowledged failed run while leaving successful runs untouched.
func TestAcknowledgeRuns(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, _ := r.UpsertTarget(store.Target{ContainerName: "sonarr", AppdataPaths: []string{"/data"}})

	// Two failed backup runs + one successful one.
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

	// Fresh runs start unacknowledged.
	if ackFlag(fail1) || ackFlag(fail2) || ackFlag(okRun) {
		t.Fatal("runs should start unacknowledged")
	}

	// Empty id list is a no-op.
	if n, err := r.AcknowledgeRuns(nil); err != nil || n != 0 {
		t.Fatalf("AcknowledgeRuns(nil) = (%d, %v), want (0, nil)", n, err)
	}

	// Acknowledge one run by id.
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

	// Acknowledge all remaining failed runs — only fail2 is still unacknowledged.
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
	// A successful run is never touched.
	if ackFlag(okRun) {
		t.Fatal("success run must not be acknowledged by AcknowledgeAllFailed")
	}
}

// TestLastSuccessfulBackupDomainScoped verifies that the per-domain everyN
// due-gate queries are scoped to their own table: a VM backup must NOT satisfy
// the containers gate, and vice versa. (Both kinds share kind='backup'; the
// distinction is whether target_id lives in `targets` or `vms`.)
func TestLastSuccessfulBackupDomainScoped(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// Record a successful VM backup only — no container backup.
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

	// The VM gate sees it…
	vmLast, err := r.LastSuccessfulVMBackup()
	if err != nil {
		t.Fatalf("LastSuccessfulVMBackup: %v", err)
	}
	if vmLast.IsZero() {
		t.Fatal("LastSuccessfulVMBackup should be non-zero after a VM backup")
	}

	// …but the containers gate must NOT (no container has been backed up).
	cLast, err := r.LastSuccessfulContainerBackup()
	if err != nil {
		t.Fatalf("LastSuccessfulContainerBackup: %v", err)
	}
	if !cLast.IsZero() {
		t.Fatalf("LastSuccessfulContainerBackup should be zero (a VM backup must not satisfy the containers gate), got %v", cLast)
	}
}

// TestLastSuccessfulFilesBackupAndCounts verifies the files domain helpers: a
// run recorded against a file_sets.id satisfies the files everyN due-gate, is
// attributed to the "files" bucket by RunCounts, and does NOT satisfy the
// containers gate (scoping mirrors TestLastSuccessfulBackupDomainScoped).
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
	// Before any run the gate must report zero.
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
	// A file-set backup must NOT satisfy the containers gate.
	cLast, err := r.LastSuccessfulContainerBackup()
	if err != nil {
		t.Fatal(err)
	}
	if !cLast.IsZero() {
		t.Fatalf("LastSuccessfulContainerBackup should be zero (a files backup must not satisfy the containers gate), got %v", cLast)
	}
}

// TestSetRunGroup verifies SetRunGroup stamps a run's group_id, that ListRuns
// round-trips it correctly, and that a run never stamped keeps the zero-value
// "" (an ungrouped run is unaffected — the default for every run outside a
// "Backup Everything" pass).
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

	// Best-effort bookkeeping: stamping an id that matches no row is not an error.
	if err := r.SetRunGroup("no-such-run", "some-group"); err != nil {
		t.Fatalf("SetRunGroup(unknown id) must not error: %v", err)
	}
}

// TestLastEverythingPass pins the "Backup Everything" everyN due-gate's input:
// no matching rows report a zero time; a run that has FINISHED sets it, success
// or failure; a run still in flight does not.
//
// The failure case is the point, and it is a deliberate reversal — this test
// used to assert that a failed pass leaves the gate at zero. That is what made
// the pass run every night instead of every N days: the parent run is
// all-or-nothing ("success" iff every domain step had zero item failures), so a
// single item that fails persistently — one broken container, or the flash step
// on a host with no /boot mount, which is a supported deployment — means the
// pass never records a success, the gate reads "never ran" on every daily
// trigger, and the whole five-domain pass plus the batched prune and off-site
// replication runs nightly, silently. Whether the pass may run is a question
// about the INTERVAL, not about the verdict.
func TestLastEverythingPass(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// No runs yet.
	ts, err := r.LastEverythingPass()
	if err != nil {
		t.Fatal(err)
	}
	if !ts.IsZero() {
		t.Fatalf("expected zero time before any everything pass, got %v", ts)
	}

	// A pass that is still running is not a completed pass.
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

	// It finishes "failed" because one item failed. The pass still RAN, so the
	// interval starts here.
	if err := r.FinishRun(runningID, "failed", "", 0, "flash: not mounted"); err != nil {
		t.Fatal(err)
	}
	failedAt, err := r.LastEverythingPass()
	if err != nil {
		t.Fatal(err)
	}
	if failedAt.IsZero() {
		t.Fatal("a completed pass must satisfy the gate even when an item failed — otherwise the pass runs every night")
	}

	// A later successful pass moves it forward.
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

// TestLastEverythingPassIgnoresAbandonedRuns is the other half of the gate, and
// the one the "a finished run counts, success or not" rule above opened up: a
// pass that was ABANDONED must not count, even though it carries a finished_at.
//
// The pass holds one parent run open across containers → vms → flash → files →
// config plus the batched prune and off-site replication, i.e. hours. Reboot the
// box or update the container in that window and ReapInterruptedRuns — global,
// unconditional, every startup — stamps the abandoned row finished_at = the
// restart instant. On `everyN 7 03:00` that stamp shuts the everyN gate for the
// next seven days and closes the anacron catch-up with it (the stamp lies after
// the missed fire, so nothing reads as missed): a whole interval of whole-server
// backups skipped, silently, because the box rebooted mid-pass. The panic path
// (FailRunningRun) writes the same shape.
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

			// The row is closed out — that part is correct, the dashboard must not
			// show a perpetual "running" chip.
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
				t.Fatalf("an abandoned pass must not satisfy the everyN gate, got %v — the next interval of whole-server backups would be skipped", ts)
			}
		})
	}
}

// TestLastEverythingPassCountsACompletedPassAfterAReap pins that the fix is
// narrow: excluding abandoned rows must not exclude the completed pass that
// follows one. A reboot mid-pass, then the catch-up pass that actually runs to
// its own end — the gate measures the second, not the first.
func TestLastEverythingPassCountsACompletedPassAfterAReap(t *testing.T) {
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

// TestLastSuccessfulConfigBackupAndCounts verifies the config self-backup domain
// helpers: a run tagged with the reserved ConfigTargetID satisfies the config
// everyN due-gate and is attributed to the "config" domain by RunCounts.
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
