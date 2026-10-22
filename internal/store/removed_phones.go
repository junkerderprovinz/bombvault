package store

import (
	"fmt"
	"time"
)

// NotePhoneRemoved remembers that a person removed a phone's card. The phone
// keeps the group's words, so it comes back as a member, and it then comes
// back switched off.
func (r *Repo) NotePhoneRemoved(memberID string) error {
	_, err := r.db.Exec(`INSERT INTO removed_phones (member_id, at) VALUES (?, ?)
		ON CONFLICT (member_id) DO UPDATE SET at = excluded.at`, memberID, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("NotePhoneRemoved: %w", err)
	}
	return nil
}

// RemovedPhones returns the member ids of the phones whose cards were removed.
func (r *Repo) RemovedPhones() (map[string]bool, error) {
	rows, err := r.db.Query(`SELECT member_id FROM removed_phones`)
	if err != nil {
		return nil, fmt.Errorf("RemovedPhones: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("RemovedPhones: %w", err)
		}
		out[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("RemovedPhones: %w", err)
	}
	return out, nil
}
