package store

import (
	"database/sql"
	"encoding/base64"
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
	// EncryptionBefore is the encryption setting the adoption replaced, nil
	// when it replaced none.
	EncryptionBefore *bool
	// Lost says which sealed values of the old install a settings restore
	// could not open, as the JSON the restore wrote. Empty for none.
	Lost string
}

// AdoptedKey returns the adopted backup key, found false for an install that
// backs up under its own key.
func (r *Repo) AdoptedKey() (key AdoptedKey, found bool, err error) {
	var before sql.NullBool
	err = r.db.QueryRow(`SELECT key_enc, source, adopted_at, encryption_before, lost FROM backup_key WHERE id = 1`).
		Scan(&key.KeyEnc, &key.Source, &key.AdoptedAt, &before, &key.Lost)
	if errors.Is(err, sql.ErrNoRows) {
		return AdoptedKey{}, false, nil
	}
	if err != nil {
		return AdoptedKey{}, false, fmt.Errorf("AdoptedKey: %w", err)
	}
	if before.Valid {
		key.EncryptionBefore = &before.Bool
	}
	return key, true, nil
}

// SetAdoptedKey stores the sealed key of the backups this install takes over,
// with the encryption setting the adoption is about to replace. A key adopted
// over another one keeps the setting from before the first.
func (r *Repo) SetAdoptedKey(enc []byte, source string, encryptionBefore bool) error {
	_, err := r.db.Exec(`
		INSERT INTO backup_key (id, key_enc, source, adopted_at, encryption_before) VALUES (1, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET key_enc = excluded.key_enc, source = excluded.source, adopted_at = excluded.adopted_at,
			encryption_before = coalesce(backup_key.encryption_before, excluded.encryption_before), lost = ''`,
		enc, source, time.Now().Unix(), encryptionBefore)
	if err != nil {
		return fmt.Errorf("SetAdoptedKey: %w", err)
	}
	return nil
}

// ClearAdoptedLoss forgets which sealed values a settings restore could not
// open, once somebody has seen the list.
func (r *Repo) ClearAdoptedLoss() error {
	if _, err := r.db.Exec(`UPDATE backup_key SET lost = ''`); err != nil {
		return fmt.Errorf("ClearAdoptedLoss: %w", err)
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

// RepoEstablishedSince lists the repositories this install created or
// opened for a backup at or after since. They outlive the runs of an entry
// that was deleted.
func (r *Repo) RepoEstablishedSince(since int64) ([]string, error) {
	rows, err := r.db.Query(`SELECT repo FROM established_repos WHERE created_at >= ? ORDER BY repo`, since)
	if err != nil {
		return nil, fmt.Errorf("RepoEstablishedSince: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	var out []string
	for rows.Next() {
		var repo string
		if err := rows.Scan(&repo); err != nil {
			return nil, fmt.Errorf("RepoEstablishedSince: %w", err)
		}
		out = append(out, repo)
	}
	return out, rows.Err()
}

// ReplacedSignIns are the keys of the default cloud sign-in and the rclone
// config an install had before an emergency stored a kit's over them, sealed
// as the settings hold them. An empty one is a sign-in the install did not
// have.
type ReplacedSignIns struct {
	CloudConf  string
	RcloneConf string
}

func sealedOrNull(stored string) (any, error) {
	if stored == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(stored)
}

// KeepReplacedSignIns remembers what the install had, unless it remembers
// already: a second store during the same emergency would otherwise record
// the kit's sign-ins as the install's own.
func (r *Repo) KeepReplacedSignIns(before ReplacedSignIns) error {
	cloud, err := sealedOrNull(before.CloudConf)
	if err != nil {
		return fmt.Errorf("KeepReplacedSignIns: %w", err)
	}
	rclone, err := sealedOrNull(before.RcloneConf)
	if err != nil {
		return fmt.Errorf("KeepReplacedSignIns: %w", err)
	}
	_, err = r.db.Exec(`INSERT INTO replaced_sign_ins (id, cloud_conf_enc, rclone_conf_enc, replaced_at) VALUES (1, ?, ?, ?)
		ON CONFLICT (id) DO NOTHING`, cloud, rclone, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("KeepReplacedSignIns: %w", err)
	}
	return nil
}

// ReplacedSignIns returns what KeepReplacedSignIns remembered, found false
// when no emergency replaced anything.
func (r *Repo) ReplacedSignIns() (ReplacedSignIns, bool, error) {
	var cloud, rclone []byte
	err := r.db.QueryRow(`SELECT cloud_conf_enc, rclone_conf_enc FROM replaced_sign_ins WHERE id = 1`).Scan(&cloud, &rclone)
	if errors.Is(err, sql.ErrNoRows) {
		return ReplacedSignIns{}, false, nil
	}
	if err != nil {
		return ReplacedSignIns{}, false, fmt.Errorf("ReplacedSignIns: %w", err)
	}
	out := ReplacedSignIns{}
	if len(cloud) > 0 {
		out.CloudConf = base64.StdEncoding.EncodeToString(cloud)
	}
	if len(rclone) > 0 {
		out.RcloneConf = base64.StdEncoding.EncodeToString(rclone)
	}
	return out, true, nil
}

// ClearReplacedSignIns forgets them, once the emergency is finished or undone.
func (r *Repo) ClearReplacedSignIns() error {
	if _, err := r.db.Exec(`DELETE FROM replaced_sign_ins`); err != nil {
		return fmt.Errorf("ClearReplacedSignIns: %w", err)
	}
	return nil
}
