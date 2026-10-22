package store

import (
	"fmt"
	"time"
)

// NoteReceiverLocation remembers that url, a rest-server address that ends in
// this instance's login, lies on a member's receiving server, so a storage
// location there can still be found while the member is out of reach.
func (r *Repo) NoteReceiverLocation(memberID, url string) error {
	_, err := r.db.Exec(`INSERT INTO receiver_locations (member_id, url, at) VALUES (?, ?, ?)
		ON CONFLICT (member_id, url) DO UPDATE SET at = excluded.at`, memberID, url, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("NoteReceiverLocation: %w", err)
	}
	return nil
}

// ReceiverLocations returns the remembered addresses on each member's
// receiving server, keyed by member id.
func (r *Repo) ReceiverLocations() (map[string][]string, error) {
	rows, err := r.db.Query(`SELECT member_id, url FROM receiver_locations ORDER BY member_id, at DESC`)
	if err != nil {
		return nil, fmt.Errorf("ReceiverLocations: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := map[string][]string{}
	for rows.Next() {
		var member, url string
		if err := rows.Scan(&member, &url); err != nil {
			return nil, fmt.Errorf("ReceiverLocations: %w", err)
		}
		out[member] = append(out[member], url)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ReceiverLocations: %w", err)
	}
	return out, nil
}

// ForgetReceiverLocations drops what was remembered about a member's
// receiving server.
func (r *Repo) ForgetReceiverLocations(memberID string) error {
	if _, err := r.db.Exec(`DELETE FROM receiver_locations WHERE member_id = ?`, memberID); err != nil {
		return fmt.Errorf("ForgetReceiverLocations: %w", err)
	}
	return nil
}
