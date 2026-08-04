package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// MarkRepoEstablished records that a repo was successfully created or opened at
// this destination. A later failure to open it (its `config` gone) then means the
// backing store vanished — e.g. a remote backup share that mounts late at boot
// (#55) — rather than a fresh location to (re-)initialise. Idempotent.
func (r *Repo) MarkRepoEstablished(repo string) error {
	if _, err := r.db.Exec(
		`INSERT INTO established_repos (repo, created_at) VALUES (?, ?) ON CONFLICT(repo) DO NOTHING`,
		repo, time.Now().Unix(),
	); err != nil {
		return fmt.Errorf("MarkRepoEstablished: %w", err)
	}
	return nil
}

// ClearRepoEstablished removes the established marker for a repo destination.
// Called when the destination is confirmed present (mounted + writable) but the
// repo is legitimately not there yet — a stale/phantom marker from a pre-mount
// init that must not block re-establishing the repo on the live disk (#120).
// Idempotent: deleting a missing row is not an error.
func (r *Repo) ClearRepoEstablished(repo string) error {
	if _, err := r.db.Exec(`DELETE FROM established_repos WHERE repo = ?`, repo); err != nil {
		return fmt.Errorf("ClearRepoEstablished: %w", err)
	}
	return nil
}

// IsRepoEstablished reports whether MarkRepoEstablished has ever recorded this
// repo destination.
func (r *Repo) IsRepoEstablished(repo string) (bool, error) {
	var one int
	err := r.db.QueryRow(`SELECT 1 FROM established_repos WHERE repo = ?`, repo).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("IsRepoEstablished: %w", err)
	}
	return true, nil
}
