package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// SizeBreakdown is the folder sizes of one item's snapshot, with Tree as the
// api package writes it. ParentID is the snapshot the added sizes compare
// with, empty for a first backup. Partial means the comparison had more
// changed files than it keeps, so the added sizes are a lower bound.
type SizeBreakdown struct {
	TargetID, SnapshotID, Domain, ParentID string
	CreatedAt                              int64
	Partial                                bool
	Tree                                   string
}

// sizeBreakdownsKept is how many snapshots of an item keep their breakdown.
const sizeBreakdownsKept = 3

// PutSizeBreakdown stores a breakdown and drops the item's older ones.
func (r *Repo) PutSizeBreakdown(b SizeBreakdown) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("PutSizeBreakdown: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // a no-op after Commit
	_, err = tx.Exec(`INSERT OR REPLACE INTO size_breakdowns
		(target_id, snapshot_id, domain, parent_id, created_at, partial, tree)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		b.TargetID, b.SnapshotID, b.Domain, b.ParentID, b.CreatedAt, boolToInt(b.Partial), b.Tree)
	if err != nil {
		return fmt.Errorf("PutSizeBreakdown: %w", err)
	}
	_, err = tx.Exec(`DELETE FROM size_breakdowns WHERE target_id = ? AND snapshot_id NOT IN (
		SELECT snapshot_id FROM size_breakdowns WHERE target_id = ? ORDER BY created_at DESC, rowid DESC LIMIT ?)`,
		b.TargetID, b.TargetID, sizeBreakdownsKept)
	if err != nil {
		return fmt.Errorf("PutSizeBreakdown: %w", err)
	}
	return tx.Commit()
}

// GetSizeBreakdown returns the breakdown of one snapshot of an item.
func (r *Repo) GetSizeBreakdown(targetID, snapshotID string) (SizeBreakdown, bool, error) {
	b := SizeBreakdown{TargetID: targetID, SnapshotID: snapshotID}
	var partial int64
	err := r.db.QueryRow(`SELECT domain, parent_id, created_at, partial, tree FROM size_breakdowns
		WHERE target_id = ? AND snapshot_id = ?`, targetID, snapshotID).
		Scan(&b.Domain, &b.ParentID, &b.CreatedAt, &partial, &b.Tree)
	if errors.Is(err, sql.ErrNoRows) {
		return SizeBreakdown{}, false, nil
	}
	if err != nil {
		return SizeBreakdown{}, false, fmt.Errorf("GetSizeBreakdown: %w", err)
	}
	b.Partial = partial != 0
	return b, true, nil
}

// SizeBreakdownDomain says whether an item has any breakdown and of which
// domain, which is how a finished backup knows to refresh it.
func (r *Repo) SizeBreakdownDomain(targetID string) (string, bool, error) {
	var domain string
	err := r.db.QueryRow(`SELECT domain FROM size_breakdowns WHERE target_id = ? LIMIT 1`, targetID).Scan(&domain)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("SizeBreakdownDomain: %w", err)
	}
	return domain, true, nil
}
