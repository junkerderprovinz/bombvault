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

// LastSuccessfulContainerBackup returns the time of the most recent successful
// backup run across ALL container targets, or a zero time when there has been
// none. This is used by the scheduler's everyN due-gate to decide whether the
// containers domain is due for a run. It is scoped to container targets
// (target_id in the `targets` table) so a VM backup never satisfies the gate.
func (r *Repo) LastSuccessfulContainerBackup() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success'
		  AND target_id IN (SELECT id FROM targets)
		ORDER BY started_at DESC
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
		WHERE kind = 'backup' AND status = 'success'
		  AND target_id IN (SELECT id FROM vms)
		ORDER BY started_at DESC
		LIMIT 1`)
	return scanLastBackupTime(row, "LastSuccessfulVMBackup")
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

func scanRun(s scanner) (Run, error) {
	var run Run
	var finishedAt sql.NullInt64
	var snapID, errCol sql.NullString
	err := s.Scan(
		&run.ID, &run.TargetID, &run.Kind, &run.Status,
		&run.StartedAt, &finishedAt, &snapID, &run.Bytes, &errCol,
	)
	if err != nil {
		return Run{}, err
	}
	if finishedAt.Valid {
		run.FinishedAt = &finishedAt.Int64
	}
	if snapID.Valid {
		run.SnapshotID = snapID.String
	}
	if errCol.Valid {
		run.Error = errCol.String
	}
	return run, nil
}
