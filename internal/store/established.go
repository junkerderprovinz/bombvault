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
// boot, rather than a fresh location to initialise. It also clears a gone mark,
// since the repo opened. Idempotent.
func (r *Repo) MarkRepoEstablished(repo string) error {
	if _, err := r.db.Exec(
		`INSERT INTO established_repos (repo, created_at) VALUES (?, ?) ON CONFLICT(repo) DO UPDATE SET gone_at = 0 WHERE gone_at != 0`,
		repo, time.Now().Unix(),
	); err != nil {
		return fmt.Errorf("MarkRepoEstablished: %w", err)
	}
	return nil
}

// MarkRepoGone records that the established repo was found missing where it
// was. A repo never established is left alone.
func (r *Repo) MarkRepoGone(repo string) error {
	if _, err := r.db.Exec(`UPDATE established_repos SET gone_at = ? WHERE repo = ?`, time.Now().Unix(), repo); err != nil {
		return fmt.Errorf("MarkRepoGone: %w", err)
	}
	return nil
}

// IsRepoGone reports whether repo was found missing and has not opened since.
func (r *Repo) IsRepoGone(repo string) (bool, error) {
	var gone int64
	err := r.db.QueryRow(`SELECT gone_at FROM established_repos WHERE repo = ?`, repo).Scan(&gone)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("IsRepoGone: %w", err)
	}
	return gone != 0, nil
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
