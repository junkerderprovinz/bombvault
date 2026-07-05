package backup

import (
	"context"
	"fmt"
)

// FlashRestic is the restic surface the flash domain needs for backup. Flash is
// a plain directory backup of the Unraid USB (/boot). Restore is NOT handled
// here: it is a non-destructive zip download (restic dump) streamed straight to
// the browser by the service layer, never an in-place or to-folder restore over
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

// BackupFlash backs up the whole Unraid flash directory via restic. Unlike the
// container/VM domains there is no lifecycle to manage (no stop/start) — the
// flash is just a directory tree — so this is a thin record-around-restic.
func BackupFlash(ctx context.Context, d FlashBackupDeps) (Summary, error) {
	runID, err := d.Runs.Start(d.TargetID, kindBackup)
	if err != nil {
		return Summary{}, fmt.Errorf("flash backup: start run: %w", err)
	}
	// Exclude .git so a user/plugin-created /boot/.git never flows into the flash
	// snapshot (or the download/export zips) — matching Unraid's own flash backup,
	// which omits it (#31). restic matches the bare ".git" by basename at any depth.
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
