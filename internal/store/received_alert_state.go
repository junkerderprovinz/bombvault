package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// ReceivedAlertState is WatchdogState for one stale source on a received repo:
// when the dead man's switch alert fired and the snapshot time it was based
// on. While BasedOn is unchanged no new alert is sent; a newer snapshot, or the
// recovery path deleting the row, re-arms it.
type ReceivedAlertState struct {
	ReceivedRepoID string
	Source         string
	NotifiedAt     int64
	BasedOn        int64
}

// GetReceivedAlertState returns the alert episode of a source on a received
// repo, or false when the source has none.
func (r *Repo) GetReceivedAlertState(receivedRepoID, source string) (ReceivedAlertState, bool, error) {
	row := r.db.QueryRow(`
		SELECT received_repo_id, source, notified_at, based_on
		FROM received_alert_state WHERE received_repo_id = ? AND source = ?`, receivedRepoID, source)
	var st ReceivedAlertState
	err := row.Scan(&st.ReceivedRepoID, &st.Source, &st.NotifiedAt, &st.BasedOn)
	if errors.Is(err, sql.ErrNoRows) {
		return ReceivedAlertState{}, false, nil
	}
	if err != nil {
		return ReceivedAlertState{}, false, fmt.Errorf("GetReceivedAlertState: %w", err)
	}
	return st, true, nil
}

// UpsertReceivedAlertState records (or refreshes) a source's stale-episode state.
func (r *Repo) UpsertReceivedAlertState(st ReceivedAlertState) error {
	_, err := r.db.Exec(`
		INSERT INTO received_alert_state (received_repo_id, source, notified_at, based_on)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(received_repo_id, source) DO UPDATE SET notified_at = excluded.notified_at,
		                                                    based_on    = excluded.based_on`,
		st.ReceivedRepoID, st.Source, st.NotifiedAt, st.BasedOn,
	)
	if err != nil {
		return fmt.Errorf("UpsertReceivedAlertState: %w", err)
	}
	return nil
}

// DeleteReceivedAlertState removes one source's episode once a newer snapshot
// has arrived. Deleting a missing row is a no-op.
func (r *Repo) DeleteReceivedAlertState(receivedRepoID, source string) error {
	if _, err := r.db.Exec(`DELETE FROM received_alert_state WHERE received_repo_id = ? AND source = ?`, receivedRepoID, source); err != nil {
		return fmt.Errorf("DeleteReceivedAlertState: %w", err)
	}
	return nil
}

// DeleteReceivedAlertStatesForRepo removes every episode recorded for a received
// repo, so deleting the repo leaves no orphaned alert state behind.
func (r *Repo) DeleteReceivedAlertStatesForRepo(receivedRepoID string) error {
	if _, err := r.db.Exec(`DELETE FROM received_alert_state WHERE received_repo_id = ?`, receivedRepoID); err != nil {
		return fmt.Errorf("DeleteReceivedAlertStatesForRepo: %w", err)
	}
	return nil
}
