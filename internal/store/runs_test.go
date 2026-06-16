package store_test

import (
	"testing"

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
