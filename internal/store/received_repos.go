package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrEmptyReceivedRepo is returned when a received repo has no repo location.
var ErrEmptyReceivedRepo = errors.New("received repo location must not be empty")

// ReceivedRepo is an immutable off-site repository that another BombVault
// replicates into and this box monitors read-only. It is unrelated to Target,
// which is a backup source.
type ReceivedRepo struct {
	ID   string
	Name string
	Repo string
	// AppKeyEnc is the sending instance's APP_KEY, encrypted with this
	// instance's key (internal/secret). Only the read-only engine decrypts it,
	// and it is never logged or returned in the clear.
	AppKeyEnc []byte
	// DeadManHours is how long without a new snapshot before the dead man's
	// switch alert fires. Default 26.
	DeadManHours int
	// CheckCadence schedules the independent restic check in the off-site
	// schedule grammar; "off" disables it.
	CheckCadence string
	// ReadDataPercent is the --read-data-subset percentage of the deep check;
	// 0 checks structure only.
	ReadDataPercent int
	// LastCheckAt is the Unix time of the last check, 0 if none.
	LastCheckAt int64
	// LastCheckOK is not Valid until the first check.
	LastCheckOK sql.NullBool
	// LastCheckError is the last check's scrubbed error.
	LastCheckError string
	// LastCheckReadData reports whether the last check read back pack data
	// rather than only structure.
	LastCheckReadData bool
	Enabled           bool
	CreatedAt         int64
	SortOrder         int
}

const receivedRepoCols = `id, name, repo, app_key_enc, dead_man_hours, check_cadence, read_data_percent,
	last_check_at, last_check_ok, last_check_error, last_check_read_data, enabled, created_at, sort_order`

// CreateReceivedRepo inserts a new received repo, assigning an ID and CreatedAt
// when they are unset, and returns the stored row.
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
		rr.AppKeyEnc = []byte{} // the column rejects NULL
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

// UpdateReceivedRepo updates the received repo identified by rr.ID. Updating a
// missing id is not an error.
func (r *Repo) UpdateReceivedRepo(rr ReceivedRepo) error {
	if strings.TrimSpace(rr.Repo) == "" {
		return ErrEmptyReceivedRepo
	}
	if rr.AppKeyEnc == nil {
		rr.AppKeyEnc = []byte{} // the column rejects NULL
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

// UpdateReceivedRepoCheckResult writes just the last-check columns of repo id,
// so recording a verdict cannot clobber a concurrent edit of the rest of the
// row. Updating a missing id is not an error.
func (r *Repo) UpdateReceivedRepoCheckResult(id string, at int64, ok sql.NullBool, checkErr string, ranReadData bool) error {
	_, err := r.db.Exec(`
		UPDATE received_repos SET
		  last_check_at        = ?,
		  last_check_ok        = ?,
		  last_check_error     = ?,
		  last_check_read_data = ?
		WHERE id = ?`,
		at, nullBool(ok), checkErr, boolInt(ranReadData), id,
	)
	if err != nil {
		return fmt.Errorf("UpdateReceivedRepoCheckResult: %w", err)
	}
	return nil
}

// ListReceivedRepos returns all received repos in display order.
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

// GetReceivedRepo returns the received repo with the given id, or false when
// there is none.
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

// DeleteReceivedRepo removes the received repo with the given id. A missing id
// is not an error.
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

func nullBool(b sql.NullBool) any {
	if !b.Valid {
		return nil
	}
	return boolInt(b.Bool)
}
