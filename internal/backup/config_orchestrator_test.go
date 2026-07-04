package backup_test

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

// TestBackupConfigRecordsRunAndTags reuses the shared fakeRestic / fakeRuns test
// doubles (defined in orchestrator_test.go) — fakeRestic satisfies ConfigRestic
// via its Backup method and records the repo/paths/tags in its log; fakeRuns
// records run start/finish. The real doubles differ from the plan's snippet:
// fakeRestic has no `tags` field (paths+tags land in .log) and fakeRuns exposes
// `finishes []string` rather than a `finishedStatus` scalar — assertions adapt to
// the real code.
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
	// fakeRestic.Backup logs "backup:<repo>:<paths>:<tags>" — assert the staged
	// snapshot dir was the only path and the snapshot was tagged "config".
	wantBackup := "backup:/repo:/config/.snapshot:config"
	if len(fr.log) != 1 || fr.log[0] != wantBackup {
		t.Fatalf("restic log: got %v, want [%q]", fr.log, wantBackup)
	}
	// Run recorded: started for the config target as a backup, then finished success.
	if len(runs.log) < 1 || runs.log[0] != "runStart:config:backup" {
		t.Fatalf("run start not recorded: %v", runs.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("expected one success run, got %v", runs.finishes)
	}
}

// TestBackupConfigRecordsFailure ensures a restic failure is recorded as a failed
// run and surfaced to the caller (parity with the flash orchestrator).
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
