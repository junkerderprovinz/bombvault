package backup_test

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

// fakeFilesRestic implements backup.FilesRestic (Backup only — file-set restore
// lives in the service layer, not the orchestrator), recording the paths, tags,
// and excludes it was asked to back up.
type fakeFilesRestic struct {
	backedUpPaths []string
	tags          []string
	excludes      []string
	backupErr     error
}

func (f *fakeFilesRestic) Backup(_ context.Context, _ string, paths, tags []string, excludes ...string) (backup.Summary, error) {
	f.backedUpPaths = paths
	f.tags = tags
	f.excludes = excludes
	if f.backupErr != nil {
		return backup.Summary{}, f.backupErr
	}
	return backup.Summary{SnapshotID: "abcd1234ef567890", Bytes: 4096}, nil
}

func TestBackupFileSetDir(t *testing.T) {
	rc := &fakeFilesRestic{}
	runs := &fakeRuns{}
	sum, err := backup.BackupFileSetDir(context.Background(), backup.FileSetBackupDeps{
		SourceDir: "/host/user/data/docs",
		Repo:      "/repo/files",
		TargetID:  "set-1",
		SetName:   "docs",
		Excludes:  []string{"*.tmp", "cache/**"},
		Restic:    rc,
		Runs:      runs,
	})
	if err != nil {
		t.Fatalf("BackupFileSetDir: %v", err)
	}
	if sum.SnapshotID != "abcd1234ef567890" {
		t.Fatalf("snapshot id: %q", sum.SnapshotID)
	}
	if len(rc.backedUpPaths) != 1 || rc.backedUpPaths[0] != "/host/user/data/docs" {
		t.Fatalf("expected to back up the set's source dir, got %v", rc.backedUpPaths)
	}
	// The snapshot must carry the fileset:<Name> tag — it is the ONLY link
	// between a snapshot and its file set (the files domain has no defs dir).
	if len(rc.tags) != 1 || rc.tags[0] != "fileset:docs" {
		t.Fatalf("expected tag fileset:docs, got %v", rc.tags)
	}
	// The set's excludes pass through to restic verbatim, in order.
	if len(rc.excludes) != 2 || rc.excludes[0] != "*.tmp" || rc.excludes[1] != "cache/**" {
		t.Fatalf("expected the set's excludes passed through, got %v", rc.excludes)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("expected one success run, got %v", runs.finishes)
	}
	// The run is attributed to the set's stable id, not its name — a rename must
	// never orphan run history (runs.target_id = file_sets.id).
	if len(runs.log) == 0 || runs.log[0] != "runStart:set-1:backup" {
		t.Fatalf("expected the run recorded against the set id, got %v", runs.log)
	}
}

func TestBackupFileSetDirRecordsFailure(t *testing.T) {
	rc := &fakeFilesRestic{backupErr: errors.New("restic boom")}
	runs := &fakeRuns{}
	if _, err := backup.BackupFileSetDir(context.Background(), backup.FileSetBackupDeps{
		SourceDir: "/host/user/data/docs", Repo: "/repo/files", TargetID: "set-1", SetName: "docs", Restic: rc, Runs: runs,
	}); err == nil {
		t.Fatal("expected error")
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("expected one failed run, got %v", runs.finishes)
	}
}
