package backup

import (
	"context"
	"fmt"
)

// ConfigRestic is the restic surface the config self-backup domain needs. Like
// flash it is a plain directory backup (of a staged snapshot of /config), so
// there is no lifecycle to manage.
type ConfigRestic interface {
	Backup(ctx context.Context, repo string, paths, tags []string, excludes ...string) (Summary, error)
}

// ConfigBackupDeps bundles everything BackupConfig needs. SourceDir is the staged
// snapshot directory (VACUUM-INTO DB + rclone.conf + ssh/), NOT the live /config.
type ConfigBackupDeps struct {
	SourceDir string
	Repo      string
	TargetID  string // store.ConfigTargetID
	Restic    ConfigRestic
	Runs      Runs
}

// BackupConfig backs up BombVault's own staged /config snapshot via restic. A
// thin record-around-restic (no stop/start): the DB was made consistent upstream
// by VACUUM INTO, so this just snapshots the staging directory.
func BackupConfig(ctx context.Context, d ConfigBackupDeps) (Summary, error) {
	runID, err := d.Runs.Start(d.TargetID, kindBackup)
	if err != nil {
		return Summary{}, fmt.Errorf("config backup: start run: %w", err)
	}
	summary, err := d.Restic.Backup(ctx, d.Repo, []string{d.SourceDir}, []string{"config"})
	if err != nil {
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(err))
		return Summary{}, err
	}
	if err := d.Runs.Finish(runID, statusSuccess, summary.SnapshotID, summary.Bytes, ""); err != nil {
		return summary, fmt.Errorf("config backup: record run: %w", err)
	}
	return summary, nil
}
