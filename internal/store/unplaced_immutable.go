package store

import (
	"database/sql"
	"fmt"
)

// SetUnplacedImmutable switches the append-only flag of a target or named
// repository without a place, and of the target's direct repository with it.
// A place writes the flag of its own rows and a direct repository takes it
// from its target, so neither is found here. The row the domain's off-site
// field edits carries the flag into the field, since every settings save
// writes the field back onto that row.
func (r *Repo) SetUnplacedImmutable(id string, on bool) (OffsiteTarget, error) {
	var row OffsiteTarget
	_, err := r.mutateSettings(func(tx *sql.Tx, _ Settings, after *Settings) error {
		res, err := tx.Exec(`UPDATE offsite_targets SET immutable = ?
			WHERE id = ? AND role IN (?, ?) AND place_id = '' AND companion_of = ''`,
			boolInt(on), id, RoleOffsite, RoleRepo)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("row %s: %w", id, sql.ErrNoRows)
		}
		if row, err = scanOffsiteTarget(tx.QueryRow(`SELECT `+offsiteTargetCols+` FROM offsite_targets WHERE id = ?`, id)); err != nil {
			return err
		}
		if row.Role != RoleOffsite {
			return nil
		}
		if err := mirrorTx(tx, row, false); err != nil {
			return err
		}
		field, ok, err := fieldRowQ(tx, row.Domain)
		if err != nil {
			return err
		}
		if _, _, immutable := domainColumns(after, row.Domain); ok && field.ID == id && immutable != nil {
			*immutable = on
		}
		return nil
	})
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("SetUnplacedImmutable: %w", err)
	}
	return row, nil
}
