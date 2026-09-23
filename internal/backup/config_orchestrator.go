package backup

import (
	"context"
	"fmt"
)

// ConfigRestic is the restic surface BombVault's own config backup needs.
type ConfigRestic interface {
	Backup(ctx context.Context, repo string, paths, tags []string, excludes ...string) (Summary, error)
}

// ConfigBackupDeps holds what BackupConfig needs. SourceDir is the staged copy
// of /config (the database written with VACUUM INTO, rclone.conf and ssh/),
// not the live directory.
type ConfigBackupDeps struct {
	SourceDir string
	Repo      string
	TargetID  string // store.ConfigTargetID
	Restic    ConfigRestic
	Runs      Runs
}

// BackupConfig backs up the staged /config copy and records the run. VACUUM
// INTO already made the database consistent, so nothing has to be stopped.
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
