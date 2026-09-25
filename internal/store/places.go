package store

import (
	"database/sql"
	"fmt"
)

// AttachRowTx puts a row at a place. Its repo must already be the place's
// address for domain plus suffix; WritePlace keeps it so from then on. An
// empty domain is the place's base itself.
func AttachRowTx(tx *sql.Tx, rowID, placeID, domain, suffix string) error {
	return setRowPlaceTx(tx, rowID, placeID, domain, suffix)
}

// DetachRowTx takes a row off its place. The row keeps its address and its
// settings and goes on working as a row without a place.
func DetachRowTx(tx *sql.Tx, rowID string) error {
	return setRowPlaceTx(tx, rowID, "", "", "")
}

func setRowPlaceTx(tx *sql.Tx, rowID, placeID, domain, suffix string) error {
	res, err := tx.Exec(`UPDATE offsite_targets SET place_id = ?, place_domain = ?, place_suffix = ? WHERE id = ?`,
		placeID, domain, suffix, rowID)
	if err != nil {
		return fmt.Errorf("place of row %s: %w", rowID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("place of row %s: %w", rowID, err)
	}
	if n == 0 {
		return fmt.Errorf("place of row %s: %w", rowID, sql.ErrNoRows)
	}
	return nil
}

// PlaceRows returns the offsite_targets rows at a place, any role.
func (r *Repo) PlaceRows(placeID string) ([]OffsiteTarget, error) {
	rows, err := placeRowsQ(r.db, placeID)
	if err != nil {
		return nil, fmt.Errorf("PlaceRows: %w", err)
	}
	return rows, nil
}

func placeRowsQ(q queryer, placeID string) ([]OffsiteTarget, error) {
	rows, err := q.Query(`SELECT `+offsiteTargetCols+` FROM offsite_targets
		WHERE place_id = ? AND place_id <> '' ORDER BY place_domain, place_suffix, role, created_at, id`, placeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := []OffsiteTarget{}
	for rows.Next() {
		t, err := scanOffsiteTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
