package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// MDNSEnabled reports whether BombVault announces itself on the local network.
// It is on until somebody switches it off.
func (r *Repo) MDNSEnabled() (bool, error) {
	var on bool
	err := r.db.QueryRow(`SELECT enabled FROM mdns_settings WHERE id = 1`).Scan(&on)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("MDNSEnabled: %w", err)
	}
	return on, nil
}

// SetMDNSEnabled stores the switch.
func (r *Repo) SetMDNSEnabled(on bool) error {
	_, err := r.db.Exec(`INSERT INTO mdns_settings (id, enabled) VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET enabled = excluded.enabled`, on)
	if err != nil {
		return fmt.Errorf("SetMDNSEnabled: %w", err)
	}
	return nil
}
