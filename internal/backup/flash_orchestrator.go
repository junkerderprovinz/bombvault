package backup

import (
	"context"
	"fmt"
)

// FlashRestic is the restic surface the flash domain needs. Flash restore is a
// zip download (restic dump) served by the service layer, never a restore over
// the live flash.
type FlashRestic interface {
	Backup(ctx context.Context, repo string, paths, tags []string, excludes ...string) (Summary, error)
}

// FlashBackupDeps bundles everything BackupFlash needs.
type FlashBackupDeps struct {
	// SourceDir is the container-visible path of the mounted flash (e.g.
	// /host/boot) to back up.
	SourceDir string
	Repo      string
	TargetID  string // store.FlashTargetID
	Restic    FlashRestic
	Runs      Runs
}

// BackupFlash backs up the Unraid flash directory and records the run.
func BackupFlash(ctx context.Context, d FlashBackupDeps) (Summary, error) {
	runID, err := d.Runs.Start(d.TargetID, kindBackup)
	if err != nil {
		return Summary{}, fmt.Errorf("flash backup: start run: %w", err)
	}
	// Unraid's own flash backup leaves out .git, and so do the snapshot and the
	// zips built from it. restic matches ".git" by basename at any depth.
	summary, err := d.Restic.Backup(ctx, d.Repo, []string{d.SourceDir}, []string{"flash"}, ".git")
	if err != nil {
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(err))
		return Summary{}, err
	}
	if err := d.Runs.Finish(runID, statusSuccess, summary.SnapshotID, summary.Bytes, ""); err != nil {
		return summary, fmt.Errorf("flash backup: record run: %w", err)
	}
	return summary, nil
}
