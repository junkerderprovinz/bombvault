package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// OwnRecords is what a node keeps that its repositories do not: its
// containers with their recreate definitions, its folder sets and its newest
// finished runs. Home holds a copy and gives it back to the node set up anew.
type OwnRecords struct {
	Targets  []Target  `json:"targets"`
	FileSets []FileSet `json:"fileSets"`
	Runs     []Run     `json:"runs"`
}

// OwnRecords returns this node's records with at most runs of its newest
// finished runs.
func (r *Repo) OwnRecords(runs int) (OwnRecords, error) {
	var out OwnRecords
	var err error
	if out.Targets, err = r.ListTargets(); err != nil {
		return OwnRecords{}, err
	}
	if out.FileSets, err = r.ListFileSets(); err != nil {
		return OwnRecords{}, err
	}
	rows, err := r.db.Query(`SELECT `+runCols+` FROM runs WHERE finished_at IS NOT NULL ORDER BY started_at DESC, id DESC LIMIT ?`, runs)
	if err != nil {
		return OwnRecords{}, fmt.Errorf("OwnRecords: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only cursor
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return OwnRecords{}, fmt.Errorf("OwnRecords: %w", err)
		}
		out.Runs = append(out.Runs, run)
	}
	return out, rows.Err()
}

// RestoreOwnRecords puts back the containers, folder sets and runs of rec
// this node does not have. One it has already stays as it is.
func (r *Repo) RestoreOwnRecords(rec OwnRecords) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("RestoreOwnRecords: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // a no-op once committed
	for _, t := range rec.Targets {
		if err := restoreTarget(tx, t); err != nil {
			return fmt.Errorf("RestoreOwnRecords: %s: %w", t.ContainerName, err)
		}
	}
	for _, fs := range rec.FileSets {
		if err := restoreFileSet(tx, fs); err != nil {
			return fmt.Errorf("RestoreOwnRecords: %s: %w", fs.Name, err)
		}
	}
	for _, run := range rec.Runs {
		if run.FinishedAt == nil {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO runs (`+runCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT DO NOTHING`,
			run.ID, run.TargetID, run.Kind, run.Status, run.StartedAt, *run.FinishedAt, run.SnapshotID, run.Bytes, run.Error, run.Acknowledged, run.GroupID,
			run.StartedVia, run.StartedViaKey, run.Load, run.SourceBytes, run.Subject, run.SnapshotWindowMS, run.SnapshotFallback, run.RetryAt, run.Copied); err != nil {
			return fmt.Errorf("RestoreOwnRecords: run %s: %w", run.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("RestoreOwnRecords: %w", err)
	}
	return nil
}

func restoreTarget(tx *sql.Tx, t Target) error {
	if err := checkRepoChoice(t.Repo, t.RepoChosen); err != nil {
		return err
	}
	var lists [5][]byte
	for i, v := range []any{t.AppdataPaths, t.SelectedPaths, t.StopContainers, t.Excludes, t.ExcludeCaches} {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		lists[i] = b
	}
	_, err := tx.Exec(`INSERT INTO targets (id, container_name, appdata_paths, include_in_schedule, created_at, definition, pre_hook, post_hook,
		selected_paths, stop_containers, excludes, exclude_caches, update_after_backup, update_after_backup_own, last_update_check, last_update_result,
		backup_order, schedule_cadence, repo, repo_chosen, db_dump_off, db_dump_engine, zfs_snapshot, zfs_snapshot_own)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
		t.ID, t.ContainerName, string(lists[0]), boolInt(t.IncludeInSchedule), t.CreatedAt, t.Definition, t.PreHook, t.PostHook,
		string(lists[1]), string(lists[2]), string(lists[3]), string(lists[4]), boolInt(t.UpdateAfterBackup), boolInt(t.UpdateAfterBackupOwn),
		t.LastUpdateCheck, t.LastUpdateResult, t.BackupOrder, t.ScheduleCadence, t.Repo, t.RepoChosen, boolInt(t.DBDumpOff), t.DBDumpEngine,
		boolInt(t.ZFSSnapshot), boolInt(t.ZFSSnapshotOwn))
	return err
}

func restoreFileSet(tx *sql.Tx, fs FileSet) error {
	if err := checkRepoChoice(fs.Repo, fs.RepoChosen); err != nil {
		return err
	}
	if fs.Excludes == nil {
		fs.Excludes = []string{}
	}
	excludes, err := json.Marshal(fs.Excludes)
	if err != nil {
		return err
	}
	// A NULL selection means the set takes its whole folder.
	var selected *string
	if fs.SelectedPaths != nil {
		b, err := json.Marshal(fs.SelectedPaths)
		if err != nil {
			return err
		}
		selected = new(string(b))
	}
	_, err = tx.Exec(`INSERT INTO file_sets (id, name, path, excludes, enabled, schedule_cadence, selected_paths, repo, repo_chosen, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
		fs.ID, fs.Name, fs.Path, string(excludes), boolInt(fs.Enabled), fs.ScheduleCadence, selected, fs.Repo, fs.RepoChosen, fs.CreatedAt)
	return err
}
