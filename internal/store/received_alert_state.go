package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// ReceivedAlertState is the receiver dead-mans-switch's once-per-episode memory
// for one stale SOURCE on a received repo: when the alert fired and which newest
// snapshot time (unix) the stale verdict was based on. An episode is identified by
// that BasedOn timestamp — while it is unchanged the source is still in the SAME
// stale episode and the receiver stays quiet; a newer received snapshot changes it
// (or the recovery path removes the row), re-arming the alert. It mirrors
// WatchdogState, scoped by (ReceivedRepoID, Source) instead of a single domain.
type ReceivedAlertState struct {
	ReceivedRepoID string
	Source         string
	NotifiedAt     int64
	BasedOn        int64
}

// GetReceivedAlertState returns the recorded dead-mans-switch episode for a
// (received repo, source) pair. The bool is false (with a zero value) when the
// source has no active episode.
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

// DeleteReceivedAlertState removes a single source's episode (the source
// recovered — a newer snapshot arrived). Deleting a missing row is a no-op.
func (r *Repo) DeleteReceivedAlertState(receivedRepoID, source string) error {
	if _, err := r.db.Exec(`DELETE FROM received_alert_state WHERE received_repo_id = ? AND source = ?`, receivedRepoID, source); err != nil {
		return fmt.Errorf("DeleteReceivedAlertState: %w", err)
	}
	return nil
}

// DeleteReceivedAlertStatesForRepo removes every episode recorded for a received
// repo, so deleting the repo leaves no orphaned alert state behind. A no-op when
// the repo has none.
func (r *Repo) DeleteReceivedAlertStatesForRepo(receivedRepoID string) error {
	if _, err := r.db.Exec(`DELETE FROM received_alert_state WHERE received_repo_id = ?`, receivedRepoID); err != nil {
		return fmt.Errorf("DeleteReceivedAlertStatesForRepo: %w", err)
	}
	return nil
}
