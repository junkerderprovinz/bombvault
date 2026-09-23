package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// MarkRepoEstablished records that a repo was successfully created or opened at
// this destination. A later failure to open it (its `config` gone) then means
// the backing store vanished, for example a remote share that mounts late at
// boot, rather than a fresh location to initialise. Idempotent.
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
// It is called when the destination is mounted and writable but holds no repo
// yet, so a stale marker from an init that ran before the mount does not block
// creating the repo on the real disk. Idempotent.
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
