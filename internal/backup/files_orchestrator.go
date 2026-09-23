package backup

import (
	"context"
	"fmt"
)

// FilesRestic is the restic surface the files domain needs. The fileset:<Name>
// tag is the only link between a snapshot and its file set; restore happens in
// the service layer.
type FilesRestic interface {
	Backup(ctx context.Context, repo string, paths, tags []string, excludes ...string) (Summary, error)
}

// FileSetBackupDeps bundles everything BackupFileSetDir needs.
type FileSetBackupDeps struct {
	// SourceDir is the container-visible path of the set's folder, e.g.
	// /host/user/data/docs.
	SourceDir string
	// SourcePaths are the backup roots compiled from the set's tree selection
	// by service.BackupFileSet. When empty, SourceDir alone is backed up, so
	// restic never gets an empty path list.
	SourcePaths []string
	Repo        string
	// TargetID is the set's file_sets.id, so renaming a set keeps its run
	// history.
	TargetID string
	// SetName is the set's name; the snapshot is tagged fileset:<SetName>.
	SetName string
	// Excludes are the set's restic --exclude patterns, passed through verbatim.
	Excludes []string
	Restic   FilesRestic
	Runs     Runs
}

// BackupFileSetDir backs up one file set and records the run.
func BackupFileSetDir(ctx context.Context, d FileSetBackupDeps) (Summary, error) {
	runID, err := d.Runs.Start(d.TargetID, kindBackup)
	if err != nil {
		return Summary{}, fmt.Errorf("files backup: start run: %w", err)
	}
	paths := []string{d.SourceDir}
	if len(d.SourcePaths) > 0 {
		paths = d.SourcePaths
	}
	summary, err := d.Restic.Backup(ctx, d.Repo, paths, []string{"fileset:" + d.SetName}, d.Excludes...)
	if err != nil {
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(err))
		return Summary{}, err
	}
	if err := d.Runs.Finish(runID, statusSuccess, summary.SnapshotID, summary.Bytes, ""); err != nil {
		return summary, fmt.Errorf("files backup: record run: %w", err)
	}
	return summary, nil
}
