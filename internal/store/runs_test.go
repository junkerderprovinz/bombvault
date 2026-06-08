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
