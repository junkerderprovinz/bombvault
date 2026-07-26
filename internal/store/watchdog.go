package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// WatchdogState is the overdue-backup watchdog's once-per-episode memory for
// one domain: when the notification fired and which last-success timestamp the
// overdue verdict was based on. An episode is identified by that timestamp —
// while it is unchanged the domain is still in the SAME overdue episode and the
// watchdog stays quiet; a new success changes it (or removes the row via the
// recovery path), re-arming the watchdog.
type WatchdogState struct {
	Domain        string
	NotifiedAt    int64
	LastSuccessAt int64
}

// GetWatchdogState returns the recorded watchdog state for a domain. The bool
// is false (with a zero WatchdogState) when the domain has no active episode.
func (r *Repo) GetWatchdogState(domain string) (WatchdogState, bool, error) {
	row := r.db.QueryRow(`
		SELECT domain, notified_at, last_success_at
		FROM watchdog_state WHERE domain = ?`, domain)
	var ws WatchdogState
	err := row.Scan(&ws.Domain, &ws.NotifiedAt, &ws.LastSuccessAt)
	if errors.Is(err, sql.ErrNoRows) {
		return WatchdogState{}, false, nil
	}
	if err != nil {
		return WatchdogState{}, false, fmt.Errorf("GetWatchdogState: %w", err)
	}
	return ws, true, nil
}

// UpsertWatchdogState records (or refreshes) a domain's overdue-episode state.
func (r *Repo) UpsertWatchdogState(ws WatchdogState) error {
	_, err := r.db.Exec(`
		INSERT INTO watchdog_state (domain, notified_at, last_success_at)
		VALUES (?, ?, ?)
		ON CONFLICT(domain) DO UPDATE SET notified_at = excluded.notified_at,
		                                  last_success_at = excluded.last_success_at`,
		ws.Domain, ws.NotifiedAt, ws.LastSuccessAt,
	)
	if err != nil {
		return fmt.Errorf("UpsertWatchdogState: %w", err)
	}
	return nil
}

// DeleteWatchdogState removes a domain's episode state (the domain recovered —
// its backups are current again). Deleting a missing row is a no-op.
func (r *Repo) DeleteWatchdogState(domain string) error {
	if _, err := r.db.Exec(`DELETE FROM watchdog_state WHERE domain = ?`, domain); err != nil {
		return fmt.Errorf("DeleteWatchdogState: %w", err)
	}
	return nil
}
