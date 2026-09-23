package backup_test

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

func TestBackupConfigRecordsRunAndTags(t *testing.T) {
	fr := &fakeRestic{summary: backup.Summary{SnapshotID: "s1", Bytes: 42}}
	runs := &fakeRuns{}
	sum, err := backup.BackupConfig(context.Background(), backup.ConfigBackupDeps{
		SourceDir: "/config/.snapshot",
		Repo:      "/repo",
		TargetID:  "config",
		Restic:    fr,
		Runs:      runs,
	})
	if err != nil {
		t.Fatalf("BackupConfig: %v", err)
	}
	if sum.SnapshotID != "s1" || sum.Bytes != 42 {
		t.Fatalf("summary: %+v", sum)
	}
	// fakeRestic logs "backup:<repo>:<paths>:<tags>".
	wantBackup := "backup:/repo:/config/.snapshot:config"
	if len(fr.log) != 1 || fr.log[0] != wantBackup {
		t.Fatalf("restic log: got %v, want [%q]", fr.log, wantBackup)
	}
	if len(runs.log) < 1 || runs.log[0] != "runStart:config:backup" {
		t.Fatalf("run start not recorded: %v", runs.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("expected one success run, got %v", runs.finishes)
	}
}

func TestBackupConfigRecordsFailure(t *testing.T) {
	fr := &fakeRestic{backupErr: errors.New("restic boom")}
	runs := &fakeRuns{}
	if _, err := backup.BackupConfig(context.Background(), backup.ConfigBackupDeps{
		SourceDir: "/config/.snapshot", Repo: "/repo", TargetID: "config", Restic: fr, Runs: runs,
	}); err == nil {
		t.Fatal("expected error")
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("expected one failed run, got %v", runs.finishes)
	}
}
