package backup_test

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

// fakeFlashRestic records the paths and excludes it was asked to back up.
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
	return flashSummary, nil
}

var flashSummary = backup.Summary{
	SnapshotID:  "abcd1234ef567890",
	Bytes:       4096,
	Measured:    true,
	SourceBytes: 2097152,
	SourceFiles: 1204,
	FilesNew:    7,
	ResticMS:    5120,
}

func TestFlashBackupFinishCarriesSummary(t *testing.T) {
	t.Run("a successful backup records what restic measured", func(t *testing.T) {
		runs := &fakeRuns{}
		_, err := backup.BackupFlash(context.Background(), backup.FlashBackupDeps{
			SourceDir: "/host/boot", Repo: "/repo/flash", TargetID: "flash",
			Restic: &fakeFlashRestic{}, Runs: runs,
		})
		if err != nil {
			t.Fatalf("BackupFlash: %v", err)
		}
		got := runs.finishOf(t, "run-1")
		if got.status != "success" || got.sum != flashSummary {
			t.Fatalf("finish = %+v, want the restic summary on a success", got)
		}
	})

	t.Run("a failed backup records no metrics", func(t *testing.T) {
		runs := &fakeRuns{}
		_, err := backup.BackupFlash(context.Background(), backup.FlashBackupDeps{
			SourceDir: "/host/boot", Repo: "/repo/flash", TargetID: "flash",
			Restic: &fakeFlashRestic{backupErr: errors.New("restic boom")}, Runs: runs,
		})
		if err == nil {
			t.Fatal("expected the restic failure to surface")
		}
		got := runs.finishOf(t, "run-1")
		if got.status != "failed" || got.sum != (backup.Summary{}) {
			t.Fatalf("finish = %+v, want an empty summary on a failure", got)
		}
	})
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
