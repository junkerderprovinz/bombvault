package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Where an adopted backup key came from.
const (
	KeyFromKit   = "kit"
	KeyFromTyped = "typed"
	KeyFromGroup = "group"
)

// KeyTargetID is the reserved runs.target_id of what happens to keys: a kit
// read, a key taken over, a deposit made, renewed, released or deleted.
const KeyTargetID = "key"

// AdoptedKey is the key of another install's backups that this install took
// over. KeyEnc is that key sealed under this install's own.
type AdoptedKey struct {
	KeyEnc    []byte
	Source    string
	AdoptedAt int64
}

// AdoptedKey returns the adopted backup key, found false for an install that
// backs up under its own key.
func (r *Repo) AdoptedKey() (key AdoptedKey, found bool, err error) {
	err = r.db.QueryRow(`SELECT key_enc, source, adopted_at FROM backup_key WHERE id = 1`).
		Scan(&key.KeyEnc, &key.Source, &key.AdoptedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AdoptedKey{}, false, nil
	}
	if err != nil {
		return AdoptedKey{}, false, fmt.Errorf("AdoptedKey: %w", err)
	}
	return key, true, nil
}

// SetAdoptedKey stores the sealed key of the backups this install takes over.
func (r *Repo) SetAdoptedKey(enc []byte, source string) error {
	_, err := r.db.Exec(`
		INSERT INTO backup_key (id, key_enc, source, adopted_at) VALUES (1, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET key_enc = excluded.key_enc, source = excluded.source, adopted_at = excluded.adopted_at`,
		enc, source, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("SetAdoptedKey: %w", err)
	}
	return nil
}

// ClearAdoptedKey puts the install back under its own key.
func (r *Repo) ClearAdoptedKey() error {
	if _, err := r.db.Exec(`DELETE FROM backup_key`); err != nil {
		return fmt.Errorf("ClearAdoptedKey: %w", err)
	}
	return nil
}

// BackupsWrittenSince reports whether a backup or an import succeeded at or
// after since.
func (r *Repo) BackupsWrittenSince(since int64) (bool, error) {
	var n int
	err := r.db.QueryRow(`
		SELECT count(*) FROM runs
		WHERE kind IN ('backup', 'import') AND status = 'success' AND finished_at >= ?`, since).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("BackupsWrittenSince: %w", err)
	}
	return n > 0, nil
}
