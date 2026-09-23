package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// WatchdogState records when the overdue-backup watchdog notified about a
// domain and which last-success timestamp that notice was based on. While the
// timestamp is unchanged the domain is in the same overdue episode and the
// watchdog stays quiet; a new success starts a new one.
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

// DeleteWatchdogState removes a domain's episode state once its backups are
// current again. Deleting a missing row is a no-op.
func (r *Repo) DeleteWatchdogState(domain string) error {
	if _, err := r.db.Exec(`DELETE FROM watchdog_state WHERE domain = ?`, domain); err != nil {
		return fmt.Errorf("DeleteWatchdogState: %w", err)
	}
	return nil
}
