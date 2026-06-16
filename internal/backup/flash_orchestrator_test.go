package backup_test

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

// fakeFlashRestic implements backup.FlashRestic (Backup + RestoreTo).
type fakeFlashRestic struct {
	backedUpPaths []string
	restoreTarget string
	backupErr     error
	restoreErr    error
}

func (f *fakeFlashRestic) Backup(_ context.Context, _ string, paths, _ []string) (backup.Summary, error) {
	f.backedUpPaths = paths
	if f.backupErr != nil {
		return backup.Summary{}, f.backupErr
	}
	return backup.Summary{SnapshotID: "abcd1234ef567890", Bytes: 4096}, nil
}

func (f *fakeFlashRestic) RestoreTo(_ context.Context, _, _, target string) error {
	f.restoreTarget = target
	return f.restoreErr
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

func TestRestoreFlashRequiresConfirm(t *testing.T) {
	err := backup.RestoreFlash(context.Background(), backup.FlashRestoreDeps{
		Confirmed: false, SnapshotID: "abcd1234ef567890", Restic: &fakeFlashRestic{}, Runs: &fakeRuns{},
	})
	if !errors.Is(err, backup.ErrNotConfirmed) {
		t.Fatalf("expected ErrNotConfirmed, got %v", err)
	}
}

func TestRestoreFlashRejectsBadSnapshotID(t *testing.T) {
	err := backup.RestoreFlash(context.Background(), backup.FlashRestoreDeps{
		Confirmed: true, SnapshotID: "../etc", Restic: &fakeFlashRestic{}, Runs: &fakeRuns{},
	})
	if !errors.Is(err, backup.ErrInvalidSnapshotID) {
		t.Fatalf("expected ErrInvalidSnapshotID, got %v", err)
	}
}

func TestRestoreFlashExtractsToTarget(t *testing.T) {
	rc := &fakeFlashRestic{}
	runs := &fakeRuns{}
	err := backup.RestoreFlash(context.Background(), backup.FlashRestoreDeps{
		Confirmed:  true,
		SnapshotID: "abcd1234ef567890",
		Repo:       "/repo/flash",
		Target:     "/host/user/user/bombvault/flash-restore",
		TargetID:   "flash",
		Restic:     rc,
		Runs:       runs,
	})
	if err != nil {
		t.Fatalf("RestoreFlash: %v", err)
	}
	if rc.restoreTarget != "/host/user/user/bombvault/flash-restore" {
		t.Fatalf("expected extract to target folder, got %q", rc.restoreTarget)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("expected one success run, got %v", runs.finishes)
	}
}
