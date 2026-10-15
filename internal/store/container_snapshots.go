package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// SetRunSnapshotRead records how a container backup read its data. A window
// of nil means the live data was read; fallback says why, when the container
// could not be read from a snapshot.
func (r *Repo) SetRunSnapshotRead(runID string, windowMS *int64, fallback string) error {
	if _, err := r.db.Exec(`UPDATE runs SET snapshot_window_ms = ?, snapshot_fallback = ? WHERE id = ?`,
		windowMS, fallback, runID); err != nil {
		return fmt.Errorf("SetRunSnapshotRead: %w", err)
	}
	return nil
}

// LastBackupReadSnapshot reports whether the target's newest successful backup
// was read from a snapshot. found is false for a target that has none.
func (r *Repo) LastBackupReadSnapshot(targetID string) (fromSnapshot, found bool, err error) {
	var window sql.NullInt64
	err = r.db.QueryRow(`SELECT snapshot_window_ms FROM runs
		WHERE target_id = ? AND kind = 'backup' AND status = 'success'
		ORDER BY started_at DESC, rowid DESC LIMIT 1`, targetID).Scan(&window)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("LastBackupReadSnapshot: %w", err)
	}
	return window.Valid, true, nil
}

// AddContainerSnapshotDataset remembers a dataset a container backup is about
// to snapshot, so a sweep finds what a crash leaves on it.
func (r *Repo) AddContainerSnapshotDataset(dataset string, at int64) error {
	if _, err := r.db.Exec(`INSERT INTO container_snapshot_datasets (dataset, added_at) VALUES (?, ?)
		ON CONFLICT (dataset) DO NOTHING`, dataset, at); err != nil {
		return fmt.Errorf("AddContainerSnapshotDataset: %w", err)
	}
	return nil
}

// RemoveContainerSnapshotDataset forgets a dataset a sweep found clean.
func (r *Repo) RemoveContainerSnapshotDataset(dataset string) error {
	if _, err := r.db.Exec(`DELETE FROM container_snapshot_datasets WHERE dataset = ?`, dataset); err != nil {
		return fmt.Errorf("RemoveContainerSnapshotDataset: %w", err)
	}
	return nil
}

// ListContainerSnapshotDatasets returns the datasets a sweep still has to look
// at, oldest first.
func (r *Repo) ListContainerSnapshotDatasets() ([]string, error) {
	rows, err := r.db.Query(`SELECT dataset FROM container_snapshot_datasets ORDER BY added_at, dataset`)
	if err != nil {
		return nil, fmt.Errorf("ListContainerSnapshotDatasets: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("ListContainerSnapshotDatasets: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
