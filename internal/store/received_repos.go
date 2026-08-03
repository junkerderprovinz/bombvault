package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrEmptyReceivedRepo is returned when a received repo has no repo location. A
// received repo with an empty location addresses nowhere and could never be
// opened, so it is rejected at the store boundary (mirrors ErrEmptyOffsiteRepo).
var ErrEmptyReceivedRepo = errors.New("received repo location must not be empty")

// ReceivedRepo is one immutable off-site repository this box RECEIVES copies into
// (from another BombVault instance's off-site replication) and monitors READ-ONLY.
// It carries the location plus the SENDING instance's APP_KEY — stored ENCRYPTED
// at rest in AppKeyEnc (internal/secret) and only ever decrypted in-engine, never
// by the store and never logged — and the dead-mans-switch / integrity-check
// settings the receiver dashboard drives.
//
// NOTE the naming trap that already applies to OffsiteTarget: store.Target is a
// backup SOURCE (a container). This type (table received_repos) is a monitored
// off-site DESTINATION on the RECEIVING side and is unrelated to Target.
type ReceivedRepo struct {
	ID   string
	Name string
	Repo string
	// AppKeyEnc is the SENDING instance's 64-hex APP_KEY, AES-256-GCM encrypted at
	// rest via internal/secret. The store only ever persists/returns the ciphertext;
	// only the read-only engine decrypts it (with this instance's APP_KEY) to open
	// the received repo. Never logged, never returned in the clear.
	AppKeyEnc []byte
	// DeadManHours is how many hours without a newly received snapshot before the
	// dead-mans-switch alert fires. Default 26.
	DeadManHours int
	// CheckCadence is the schedule cadence for the independent restic check (same
	// grammar as the off-site schedules). 'off' = no scheduled check.
	CheckCadence string
	// ReadDataPercent drives the periodic deep check: 0 = off (structural check
	// only); 1..100 = `restic check --read-data-subset=<pct>%`.
	ReadDataPercent int
	// LastCheckAt is the Unix time of the last check (0 = never checked).
	LastCheckAt int64
	// LastCheckOK is the last check's verdict. Valid=false means never checked yet
	// (the column is nullable).
	LastCheckOK sql.NullBool
	// LastCheckError is the last check's scrubbed error ('' on success/never).
	LastCheckError string
	// LastCheckReadData records whether the last check read back real pack data
	// (a deep --read-data-subset check) rather than only structure/metadata.
	LastCheckReadData bool
	Enabled           bool
	CreatedAt         int64
	SortOrder         int
}

const receivedRepoCols = `id, name, repo, app_key_enc, dead_man_hours, check_cadence, read_data_percent,
	last_check_at, last_check_ok, last_check_error, last_check_read_data, enabled, created_at, sort_order`

// CreateReceivedRepo inserts a new received repo. An empty ID is assigned via
// newID(); CreatedAt is stamped now when 0. Returns the stored row (with the
// assigned id/timestamp). The repo location must not be empty.
func (r *Repo) CreateReceivedRepo(rr ReceivedRepo) (ReceivedRepo, error) {
	if strings.TrimSpace(rr.Repo) == "" {
		return ReceivedRepo{}, ErrEmptyReceivedRepo
	}
	if rr.ID == "" {
		rr.ID = newID()
	}
	if rr.CreatedAt == 0 {
		rr.CreatedAt = time.Now().Unix()
	}
	if rr.AppKeyEnc == nil {
		rr.AppKeyEnc = []byte{} // NOT NULL blob: bind an empty blob, never SQL NULL
	}
	_, err := r.db.Exec(`
		INSERT INTO received_repos (`+receivedRepoCols+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rr.ID, rr.Name, rr.Repo, rr.AppKeyEnc, rr.DeadManHours, rr.CheckCadence, rr.ReadDataPercent,
		rr.LastCheckAt, nullBool(rr.LastCheckOK), rr.LastCheckError, boolInt(rr.LastCheckReadData),
		boolInt(rr.Enabled), rr.CreatedAt, rr.SortOrder,
	)
	if err != nil {
		return ReceivedRepo{}, fmt.Errorf("CreateReceivedRepo: %w", err)
	}
	return rr, nil
}

// UpdateReceivedRepo updates the received repo identified by rr.ID in place. The
// repo location must not be empty. Updating a missing id affects no rows and is
// not an error (mirrors the offsite/no-op conventions).
func (r *Repo) UpdateReceivedRepo(rr ReceivedRepo) error {
	if strings.TrimSpace(rr.Repo) == "" {
		return ErrEmptyReceivedRepo
	}
	if rr.AppKeyEnc == nil {
		rr.AppKeyEnc = []byte{} // NOT NULL blob: bind an empty blob, never SQL NULL
	}
	_, err := r.db.Exec(`
		UPDATE received_repos SET
		  name                 = ?,
		  repo                 = ?,
		  app_key_enc          = ?,
		  dead_man_hours       = ?,
		  check_cadence        = ?,
		  read_data_percent    = ?,
		  last_check_at        = ?,
		  last_check_ok        = ?,
		  last_check_error     = ?,
		  last_check_read_data = ?,
		  enabled              = ?,
		  sort_order           = ?
		WHERE id = ?`,
		rr.Name, rr.Repo, rr.AppKeyEnc, rr.DeadManHours, rr.CheckCadence, rr.ReadDataPercent,
		rr.LastCheckAt, nullBool(rr.LastCheckOK), rr.LastCheckError, boolInt(rr.LastCheckReadData),
		boolInt(rr.Enabled), rr.SortOrder, rr.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateReceivedRepo: %w", err)
	}
	return nil
}

// ListReceivedRepos returns all received repos ordered by sort_order then
// created_at (a stable display order).
func (r *Repo) ListReceivedRepos() ([]ReceivedRepo, error) {
	rows, err := r.db.Query(`SELECT ` + receivedRepoCols + ` FROM received_repos ORDER BY sort_order, created_at`)
	if err != nil {
		return nil, fmt.Errorf("ListReceivedRepos: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []ReceivedRepo
	for rows.Next() {
		rr, err := scanReceivedRepo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}

// GetReceivedRepo returns the received repo with the given id. The bool is false
// (with a zero ReceivedRepo) when no such row exists.
func (r *Repo) GetReceivedRepo(id string) (ReceivedRepo, bool, error) {
	row := r.db.QueryRow(`SELECT `+receivedRepoCols+` FROM received_repos WHERE id = ?`, id)
	rr, err := scanReceivedRepo(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ReceivedRepo{}, false, nil
	}
	if err != nil {
		return ReceivedRepo{}, false, err
	}
	return rr, true, nil
}

// DeleteReceivedRepo removes the received repo with the given id. It is a no-op
// (no error) if the row does not exist.
func (r *Repo) DeleteReceivedRepo(id string) error {
	if _, err := r.db.Exec(`DELETE FROM received_repos WHERE id = ?`, id); err != nil {
		return fmt.Errorf("DeleteReceivedRepo: %w", err)
	}
	return nil
}

func scanReceivedRepo(s scanner) (ReceivedRepo, error) {
	var rr ReceivedRepo
	var readData, enabled int
	err := s.Scan(
		&rr.ID, &rr.Name, &rr.Repo, &rr.AppKeyEnc, &rr.DeadManHours, &rr.CheckCadence, &rr.ReadDataPercent,
		&rr.LastCheckAt, &rr.LastCheckOK, &rr.LastCheckError, &readData, &enabled, &rr.CreatedAt, &rr.SortOrder,
	)
	if err != nil {
		return ReceivedRepo{}, fmt.Errorf("scanReceivedRepo: %w", err)
	}
	rr.LastCheckReadData = readData != 0
	rr.Enabled = enabled != 0
	return rr, nil
}

// nullBool renders a sql.NullBool as the value SQLite stores: nil (SQL NULL) when
// invalid (never set), otherwise 0/1. Kept local to the received-repo store so the
// nullable last_check_ok column round-trips cleanly.
func nullBool(b sql.NullBool) any {
	if !b.Valid {
		return nil
	}
	return boolInt(b.Bool)
}
