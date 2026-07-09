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
