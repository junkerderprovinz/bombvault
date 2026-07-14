package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Run represents a single backup or restore operation.
type Run struct {
	ID         string `json:"id"`
	TargetID   string `json:"targetId"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	StartedAt  int64  `json:"startedAt"`
	FinishedAt *int64 `json:"finishedAt"`
	SnapshotID string `json:"snapshotId"`
	Bytes      int64  `json:"bytes"`
	Error      string `json:"error"`
}

// StartRun records the beginning of a run and returns its ID.
func (r *Repo) StartRun(targetID, kind string) (string, error) {
	id := newID()
	_, err := r.db.Exec(`
		INSERT INTO runs (id, target_id, kind, status, started_at)
		VALUES (?, ?, ?, 'running', ?)`,
		id, targetID, kind, time.Now().Unix(),
	)
	if err != nil {
		return "", fmt.Errorf("StartRun: %w", err)
	}
	return id, nil
}

// FinishRun updates a run with its final status, snapshot ID, bytes, and optional error.
func (r *Repo) FinishRun(id, status, snapshotID string, bytes int64, errMsg string) error {
	now := time.Now().Unix()
	var snap, errCol any
	if snapshotID != "" {
		snap = snapshotID
	}
	if errMsg != "" {
		errCol = errMsg
	}
	res, err := r.db.Exec(`
		UPDATE runs SET status = ?, finished_at = ?, snapshot_id = ?, bytes = ?, error = ?
		WHERE id = ?`,
		status, now, snap, bytes, errCol, id,
	)
	if err != nil {
		return fmt.Errorf("FinishRun: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("FinishRun: run %s not found", id)
	}
	return nil
}

// ReapInterruptedRuns marks any run still in 'running' as failed. It is meant to
// be called once at startup: BombVault is a single process, so a run left in
// 'running' is necessarily an orphan from a previous lifetime (the process
// crashed or was updated mid-backup) and can never still be in progress. Without
// this, such a run keeps a NULL bytes/finished_at and shows a perpetual "running"
// chip on the dashboard. Returns how many runs were reaped.
func (r *Repo) ReapInterruptedRuns() (int64, error) {
	res, err := r.db.Exec(`
		UPDATE runs
		SET status = 'failed', finished_at = ?, error = 'interrupted (BombVault restarted mid-run)'
		WHERE status = 'running'`, time.Now().Unix())
	if err != nil {
		return 0, fmt.Errorf("ReapInterruptedRuns: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// LastSuccessfulBackup returns the most recent successful backup run for targetID, or nil.
func (r *Repo) LastSuccessfulBackup(targetID string) (*Run, error) {
	row := r.db.QueryRow(`
		SELECT id, target_id, kind, status, started_at, finished_at, snapshot_id, bytes, error
		FROM runs
		WHERE target_id = ? AND kind = 'backup' AND status = 'success'
		ORDER BY started_at DESC
		LIMIT 1`, targetID)
	run, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("LastSuccessfulBackup: %w", err)
	}
	return &run, nil
}

// LastRunForTarget returns the most recent backup run for targetID regardless of
// status (success, failed, skipped, running), or nil when there is none. Used to
// debounce repeated "container missing" skip warnings: warn on the first miss,
// stay quiet while the target keeps being skipped.
func (r *Repo) LastRunForTarget(targetID string) (*Run, error) {
	row := r.db.QueryRow(`
		SELECT id, target_id, kind, status, started_at, finished_at, snapshot_id, bytes, error
		FROM runs
		WHERE target_id = ? AND kind = 'backup'
		ORDER BY started_at DESC
		LIMIT 1`, targetID)
	run, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("LastRunForTarget: %w", err)
	}
	return &run, nil
}

// LastSuccessfulContainerBackup returns the time of the most recent successful
// backup run across ALL container targets, or a zero time when there has been
// none. This is used by the scheduler's everyN due-gate to decide whether the
// containers domain is due for a run. It is scoped to container targets
// (target_id in the `targets` table) so a VM backup never satisfies the gate.
func (r *Repo) LastSuccessfulContainerBackup() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL
		  AND target_id IN (SELECT id FROM targets)
		ORDER BY finished_at DESC
		LIMIT 1`)
	return scanLastBackupTime(row, "LastSuccessfulContainerBackup")
}

// LastSuccessfulVMBackup is the VM-domain counterpart of
// LastSuccessfulContainerBackup, scoped to VM targets (target_id in the `vms`
// table). Drives the VMs domain everyN due-gate.
func (r *Repo) LastSuccessfulVMBackup() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL
		  AND target_id IN (SELECT id FROM vms)
		ORDER BY finished_at DESC
		LIMIT 1`)
	return scanLastBackupTime(row, "LastSuccessfulVMBackup")
}

// LastSuccessfulFilesBackup is the files-domain counterpart of
// LastSuccessfulContainerBackup, scoped to file-set targets (target_id in the
// `file_sets` table). Drives the files domain everyN due-gate.
func (r *Repo) LastSuccessfulFilesBackup() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL
		  AND target_id IN (SELECT id FROM file_sets)
		ORDER BY finished_at DESC
		LIMIT 1`)
	return scanLastBackupTime(row, "LastSuccessfulFilesBackup")
}

// FlashTargetID is the reserved runs.target_id for the singleton flash domain
// (the Unraid USB). Flash has no per-item table, so its runs are tagged with
// this fixed id — distinct from the hex/UUID ids of container and VM targets,
// so it never collides with or pollutes the other domains' gates.
const FlashTargetID = "flash"

// LastSuccessfulFlashBackup drives the flash domain everyN due-gate, scoped to
// the reserved flash target id.
func (r *Repo) LastSuccessfulFlashBackup() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL AND target_id = ?
		ORDER BY finished_at DESC
		LIMIT 1`, FlashTargetID)
	return scanLastBackupTime(row, "LastSuccessfulFlashBackup")
}

// ConfigTargetID is the reserved runs.target_id for the singleton config self-
// backup domain (BombVault's own /config). Like FlashTargetID it is a fixed
// literal, distinct from the hex/UUID ids of container and VM targets.
const ConfigTargetID = "config"

// LastSuccessfulConfigBackup drives the config domain everyN due-gate, scoped to
// the reserved config target id.
func (r *Repo) LastSuccessfulConfigBackup() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL AND target_id = ?
		ORDER BY finished_at DESC
		LIMIT 1`, ConfigTargetID)
	return scanLastBackupTime(row, "LastSuccessfulConfigBackup")
}

// scanLastBackupTime reads the single nullable finished_at column from a
// last-successful-backup query, mapping no-rows / NULL to a zero time.
func scanLastBackupTime(row *sql.Row, label string) (time.Time, error) {
	var ts sql.NullInt64
	if err := row.Scan(&ts); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("%s: %w", label, err)
	}
	if !ts.Valid {
		return time.Time{}, nil
	}
	return time.Unix(ts.Int64, 0), nil
}

// ListRuns returns up to limit recent runs across all targets, newest first.
func (r *Repo) ListRuns(limit int) ([]Run, error) {
	rows, err := r.db.Query(`
		SELECT id, target_id, kind, status, started_at, finished_at, snapshot_id, bytes, error
		FROM runs
		ORDER BY started_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("ListRuns: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// RunsSince returns all runs with started_at >= since (unix seconds), newest
// first. Used by the dashboard's backup-health heatmap to bucket a window of
// runs by day and domain.
func (r *Repo) RunsSince(since int64) ([]Run, error) {
	rows, err := r.db.Query(`
		SELECT id, target_id, kind, status, started_at, finished_at, snapshot_id, bytes, error
		FROM runs
		WHERE started_at >= ?
		ORDER BY started_at DESC`, since)
	if err != nil {
		return nil, fmt.Errorf("RunsSince: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// RunCounts returns the total number of backup runs per domain ("containers" |
// "vms" | "flash" | "config" | "files") and status ("success" | "failed"), keyed
// [domain][status]. Domain is attributed the same way as the last-successful
// helpers: container targets live in `targets`, VM targets in `vms`, file-set
// targets in `file_sets`, and the singleton flash and config domains use the
// reserved FlashTargetID and ConfigTargetID. Only finished backup runs
// (success/failed) are counted; "running" runs are skipped. A domain/status
// with no runs is absent from the map (the caller defaults it to 0). Drives
// the Prometheus `bombvault_runs_total` counter.
func (r *Repo) RunCounts() (map[string]map[string]int, error) {
	rows, err := r.db.Query(`
		SELECT
		  CASE
		    WHEN target_id = ?                              THEN 'config'
		    WHEN target_id = ?                              THEN 'flash'
		    WHEN target_id IN (SELECT id FROM vms)          THEN 'vms'
		    WHEN target_id IN (SELECT id FROM file_sets)    THEN 'files'
		    WHEN target_id IN (SELECT id FROM targets)      THEN 'containers'
		    ELSE ''
		  END AS domain,
		  status,
		  count(*) AS n
		FROM runs
		WHERE kind = 'backup' AND status IN ('success', 'failed')
		GROUP BY domain, status`, ConfigTargetID, FlashTargetID)
	if err != nil {
		return nil, fmt.Errorf("RunCounts: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	out := map[string]map[string]int{}
	for rows.Next() {
		var domain, status string
		var n int
		if sErr := rows.Scan(&domain, &status, &n); sErr != nil {
			return nil, fmt.Errorf("RunCounts: %w", sErr)
		}
		if domain == "" {
			continue // run for a deleted/unknown target — not attributable to a domain
		}
		if out[domain] == nil {
			out[domain] = map[string]int{}
		}
		out[domain][status] = n
	}
	return out, rows.Err()
}

func scanRun(s scanner) (Run, error) {
	var run Run
	var finishedAt, bytes sql.NullInt64
	var snapID, errCol sql.NullString
	err := s.Scan(
		&run.ID, &run.TargetID, &run.Kind, &run.Status,
		&run.StartedAt, &finishedAt, &snapID, &bytes, &errCol,
	)
	if err != nil {
		return Run{}, err
	}
	if finishedAt.Valid {
		run.FinishedAt = &finishedAt.Int64
	}
	if bytes.Valid {
		run.Bytes = bytes.Int64
	}
	if snapID.Valid {
		run.SnapshotID = snapID.String
	}
	if errCol.Valid {
		run.Error = errCol.String
	}
	return run, nil
}
