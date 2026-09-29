package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// ItemProbe is one restore probe of one item: a sample of its files restored
// into a sandbox and compared with what the backup recorded.
type ItemProbe struct {
	TargetID   string `json:"targetId"`
	Domain     string `json:"domain"`
	At         int64  `json:"at"`
	OK         bool   `json:"ok"`
	Detail     string `json:"detail"`
	SnapshotID string `json:"snapshotId"`
	Files      int    `json:"files"`
	Bytes      int64  `json:"bytes"`
	// Trigger is "first" for the probe after an item's first backup and
	// "manual" for one somebody asked for.
	Trigger string `json:"trigger"`
}

// AddItemProbe records a probe result.
func (r *Repo) AddItemProbe(p ItemProbe) error {
	if p.Trigger == "" {
		p.Trigger = "first"
	}
	_, err := r.db.Exec(`
		INSERT INTO item_probes (target_id, domain, at, ok, detail, snapshot_id, files, bytes, trigger)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.TargetID, p.Domain, p.At, boolInt(p.OK), p.Detail, p.SnapshotID, p.Files, p.Bytes, p.Trigger,
	)
	if err != nil {
		return fmt.Errorf("AddItemProbe: %w", err)
	}
	return nil
}

// LatestItemProbe returns the newest probe of one item. The bool is false when
// the item was never probed.
func (r *Repo) LatestItemProbe(targetID string) (ItemProbe, bool, error) {
	row := r.db.QueryRow(`
		SELECT target_id, domain, at, ok, detail, snapshot_id, files, bytes, trigger
		FROM item_probes
		WHERE target_id = ?
		ORDER BY at DESC, id DESC
		LIMIT 1`, targetID)
	p, err := scanItemProbe(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ItemProbe{}, false, nil
	}
	if err != nil {
		return ItemProbe{}, false, fmt.Errorf("LatestItemProbe: %w", err)
	}
	return p, true, nil
}

// LatestItemProbes returns the newest probe of every item that has one, keyed
// by target id.
func (r *Repo) LatestItemProbes() (map[string]ItemProbe, error) {
	rows, err := r.db.Query(`
		SELECT target_id, domain, at, ok, detail, snapshot_id, files, bytes, trigger
		FROM item_probes
		ORDER BY at, id`)
	if err != nil {
		return nil, fmt.Errorf("LatestItemProbes: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	out := map[string]ItemProbe{}
	for rows.Next() {
		p, sErr := scanItemProbe(rows)
		if sErr != nil {
			return nil, fmt.Errorf("LatestItemProbes: %w", sErr)
		}
		out[p.TargetID] = p
	}
	return out, rows.Err()
}

// FirstSuccessfulBackupAt returns when an item's first successful backup
// finished, or 0 when it has none.
func (r *Repo) FirstSuccessfulBackupAt(targetID string) (int64, error) {
	var at sql.NullInt64
	err := r.db.QueryRow(`
		SELECT MIN(finished_at) FROM runs
		WHERE target_id = ? AND kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL`,
		targetID).Scan(&at)
	if err != nil {
		return 0, fmt.Errorf("FirstSuccessfulBackupAt: %w", err)
	}
	return at.Int64, nil
}

// FirstProbesSince is when first-backup probes started: the time the
// migration that created item_probes ran on this database.
func (r *Repo) FirstProbesSince() (int64, error) {
	var at int64
	err := r.db.QueryRow(`SELECT applied_at FROM schema_migrations WHERE version = ?`, verifyMigrationBase).Scan(&at)
	if err != nil {
		return 0, fmt.Errorf("FirstProbesSince: %w", err)
	}
	return at, nil
}

func scanItemProbe(s scanner) (ItemProbe, error) {
	var p ItemProbe
	var ok int
	if err := s.Scan(&p.TargetID, &p.Domain, &p.At, &ok, &p.Detail, &p.SnapshotID, &p.Files, &p.Bytes, &p.Trigger); err != nil {
		return ItemProbe{}, err
	}
	p.OK = ok != 0
	return p, nil
}
