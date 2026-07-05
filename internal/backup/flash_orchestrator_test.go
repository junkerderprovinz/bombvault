package backup_test

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

// fakeFlashRestic implements backup.FlashRestic (Backup only — flash restore is
// a zip download handled in the service layer, not the orchestrator).
type fakeFlashRestic struct {
	backedUpPaths []string
	excludes      []string
	backupErr     error
}

func (f *fakeFlashRestic) Backup(_ context.Context, _ string, paths, _ []string, excludes ...string) (backup.Summary, error) {
	f.backedUpPaths = paths
	f.excludes = excludes
	if f.backupErr != nil {
		return backup.Summary{}, f.backupErr
	}
	return backup.Summary{SnapshotID: "abcd1234ef567890", Bytes: 4096}, nil
}

func TestBackupFlash(t *testing.T) {
	rc := &fakeFlashRestic{}
	runs := &fakeRuns{}
	sum, err := backup.BackupFlash(context.Background(), backup.FlashBackupDeps{
		SourceDir: "/host/boot",
		Repo:      "/repo/flash",
		TargetID:  "flash",
		Restic:    rc,
		Runs:      runs,
	})
	if err != nil {
		t.Fatalf("BackupFlash: %v", err)
	}
	if sum.SnapshotID != "abcd1234ef567890" {
		t.Fatalf("snapshot id: %q", sum.SnapshotID)
	}
	if len(rc.backedUpPaths) != 1 || rc.backedUpPaths[0] != "/host/boot" {
		t.Fatalf("expected to back up /host/boot, got %v", rc.backedUpPaths)
	}
	// The flash backup must exclude .git so a /boot/.git never enters the snapshot
	// or the download/export zips — matching Unraid's own flash backup (#31).
	if len(rc.excludes) != 1 || rc.excludes[0] != ".git" {
		t.Fatalf("expected flash backup to exclude .git, got %v", rc.excludes)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("expected one success run, got %v", runs.finishes)
	}
}

func TestBackupFlashRecordsFailure(t *testing.T) {
	rc := &fakeFlashRestic{backupErr: errors.New("restic boom")}
	runs := &fakeRuns{}
	if _, err := backup.BackupFlash(context.Background(), backup.FlashBackupDeps{
		SourceDir: "/host/boot", Repo: "/repo/flash", TargetID: "flash", Restic: rc, Runs: runs,
	}); err == nil {
		t.Fatal("expected error")
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("expected one failed run, got %v", runs.finishes)
	}
}
