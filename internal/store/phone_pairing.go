package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// PhonePairing reports whether a phone of the group may pair with this
// install.
func (r *Repo) PhonePairing() (bool, error) {
	var off bool
	err := r.db.QueryRow(`SELECT off FROM phone_pairing WHERE id = 1`).Scan(&off)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("PhonePairing: %w", err)
	}
	return !off, nil
}

// SetPhonePairing switches pairing of phones on or off.
func (r *Repo) SetPhonePairing(on bool) error {
	if _, err := r.db.Exec(`INSERT INTO phone_pairing (id, off) VALUES (1, ?) ON CONFLICT (id) DO UPDATE SET off = excluded.off`, !on); err != nil {
		return fmt.Errorf("SetPhonePairing: %w", err)
	}
	return nil
}
