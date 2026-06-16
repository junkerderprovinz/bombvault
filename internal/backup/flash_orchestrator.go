package backup

import (
	"context"
	"fmt"
)

// FlashRestic is the restic surface the flash domain needs. Flash is a plain
// directory backup of the Unraid USB (/boot), and its restore EXTRACTS the
// snapshot to a target folder — it never restores in-place over the live,
// running flash (that could leave the server unbootable). So it needs a
// to-target restore, not the in-place RestorePaths the other domains use.
type FlashRestic interface {
	Backup(ctx context.Context, repo string, paths, tags []string) (Summary, error)
	RestoreTo(ctx context.Context, repo, snapshotID, target string) error
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
	summary, err := d.Restic.Backup(ctx, d.Repo, []string{d.SourceDir}, []string{"flash"})
	if err != nil {
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(err))
		return Summary{}, err
	}
	if err := d.Runs.Finish(runID, statusSuccess, summary.SnapshotID, summary.Bytes, ""); err != nil {
		return summary, fmt.Errorf("flash backup: record run: %w", err)
	}
	return summary, nil
}

// FlashRestoreDeps bundles everything RestoreFlash needs.
type FlashRestoreDeps struct {
	Confirmed  bool
	SnapshotID string // validated hex
	Repo       string
	// Target is the container-visible folder the snapshot is extracted into. It
	// is NEVER the live /boot — the user copies the recovered files to a fresh
	// USB themselves.
	Target   string
	TargetID string // store.FlashTargetID
	Restic   FlashRestic
	Runs     Runs
}

// RestoreFlash extracts a flash snapshot to a target folder (safe restore — the
// live flash is never overwritten). Mirrors the other domains' confirm gate and
// strict hex snapshot-id validation.
func RestoreFlash(ctx context.Context, d FlashRestoreDeps) error {
	if !d.Confirmed {
		return ErrNotConfirmed
	}
	if !snapshotIDRe.MatchString(d.SnapshotID) {
		return ErrInvalidSnapshotID
	}
	runID, err := d.Runs.Start(d.TargetID, kindRestore)
	if err != nil {
		return fmt.Errorf("flash restore: start run: %w", err)
	}
	if err := d.Restic.RestoreTo(ctx, d.Repo, d.SnapshotID, d.Target); err != nil {
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(err))
		return err
	}
	if err := d.Runs.Finish(runID, statusSuccess, d.SnapshotID, 0, ""); err != nil {
		return fmt.Errorf("flash restore: record run: %w", err)
	}
	return nil
}
