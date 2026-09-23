package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrEmptyPullRepo is returned when a pull source has no repo location.
var ErrEmptyPullRepo = errors.New("pull source location must not be empty")

// PullSource is another BombVault's repository whose snapshots this box copies
// into its own repository on its own schedule: off-site replication in reverse.
// Like a ReceivedRepo it keeps the other instance's APP_KEY encrypted, because
// the source repository's password derives from it.
type PullSource struct {
	ID   string
	Name string
	// Repo is the source instance's repository location (rest:, s3:, sftp:, b2:,
	// or a path under the host mount). The API refuses `rclone:`, as foreign.go
	// does: rclone reads its remotes from our own config and would authenticate
	// to a caller-chosen endpoint with our secrets.
	Repo string
	// AppKeyEnc is the source instance's APP_KEY, encrypted with this
	// instance's key (internal/secret). It is never logged or returned in the
	// clear, and like received_repos it stays out of the settings export.
	AppKeyEnc []byte
	// CredsRef names a set in the encrypted cloud-credentials blob, so a remote
	// source uses its own backend credentials rather than this box's.
	CredsRef string
	// Domain is the repository of this box the pulled snapshots land in. Empty
	// pulls every domain the source holds.
	Domain string
	// Cadence is the pull schedule in the off-site schedule grammar; "off"
	// pulls only on demand.
	Cadence string
	// LimitDownload and LimitUpload cap restic's bandwidth in KiB/s.
	LimitDownload int
	LimitUpload   int
	// LastPullAt is the Unix time of the last pull attempt, 0 if none.
	LastPullAt int64
	// LastPullOK is not Valid until the first pull.
	LastPullOK sql.NullBool
	// LastPullError is the last pull's scrubbed error.
	LastPullError string
	// SnapshotsPulled is how many snapshots the last successful pull copied.
	SnapshotsPulled int
	Enabled         bool
	CreatedAt       int64
	SortOrder       int
}

const pullSourceCols = `id, name, repo, app_key_enc, creds_ref, domain, cadence,
	limit_download, limit_upload, last_pull_at, last_pull_ok, last_pull_error,
	snapshots_pulled, enabled, created_at, sort_order`

// CreatePullSource inserts a new pull source, assigning an ID and CreatedAt
// when they are unset, and returns the stored row.
func (r *Repo) CreatePullSource(ps PullSource) (PullSource, error) {
	if strings.TrimSpace(ps.Repo) == "" {
		return PullSource{}, ErrEmptyPullRepo
	}
	if ps.ID == "" {
		ps.ID = newID()
	}
	if ps.CreatedAt == 0 {
		ps.CreatedAt = time.Now().Unix()
	}
	if ps.AppKeyEnc == nil {
		ps.AppKeyEnc = []byte{} // the column rejects NULL
	}
	_, err := r.db.Exec(`
		INSERT INTO pull_sources (`+pullSourceCols+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ps.ID, ps.Name, ps.Repo, ps.AppKeyEnc, ps.CredsRef, ps.Domain, ps.Cadence,
		ps.LimitDownload, ps.LimitUpload, ps.LastPullAt, nullBool(ps.LastPullOK), ps.LastPullError,
		ps.SnapshotsPulled, boolInt(ps.Enabled), ps.CreatedAt, ps.SortOrder,
	)
	if err != nil {
		return PullSource{}, fmt.Errorf("CreatePullSource: %w", err)
	}
	return ps, nil
}

// UpdatePullSource updates the pull source identified by ps.ID. Updating a
// missing id is not an error.
func (r *Repo) UpdatePullSource(ps PullSource) error {
	if strings.TrimSpace(ps.Repo) == "" {
		return ErrEmptyPullRepo
	}
	if ps.AppKeyEnc == nil {
		ps.AppKeyEnc = []byte{} // the column rejects NULL
	}
	_, err := r.db.Exec(`
		UPDATE pull_sources SET
		  name             = ?,
		  repo             = ?,
		  app_key_enc      = ?,
		  creds_ref        = ?,
		  domain           = ?,
		  cadence          = ?,
		  limit_download   = ?,
		  limit_upload     = ?,
		  last_pull_at     = ?,
		  last_pull_ok     = ?,
		  last_pull_error  = ?,
		  snapshots_pulled = ?,
		  enabled          = ?,
		  sort_order       = ?
		WHERE id = ?`,
		ps.Name, ps.Repo, ps.AppKeyEnc, ps.CredsRef, ps.Domain, ps.Cadence,
		ps.LimitDownload, ps.LimitUpload, ps.LastPullAt, nullBool(ps.LastPullOK), ps.LastPullError,
		ps.SnapshotsPulled, boolInt(ps.Enabled), ps.SortOrder, ps.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdatePullSource: %w", err)
	}
	return nil
}

// UpdatePullSourceResult writes just the last-pull columns of source id, so
// recording a verdict cannot clobber a concurrent edit of the rest of the row.
// Updating a missing id is not an error.
func (r *Repo) UpdatePullSourceResult(id string, at int64, ok sql.NullBool, pullErr string, snapshots int) error {
	_, err := r.db.Exec(`
		UPDATE pull_sources SET
		  last_pull_at     = ?,
		  last_pull_ok     = ?,
		  last_pull_error  = ?,
		  snapshots_pulled = ?
		WHERE id = ?`,
		at, nullBool(ok), pullErr, snapshots, id,
	)
	if err != nil {
		return fmt.Errorf("UpdatePullSourceResult: %w", err)
	}
	return nil
}

// ListPullSources returns all pull sources in display order.
func (r *Repo) ListPullSources() ([]PullSource, error) {
	rows, err := r.db.Query(`SELECT ` + pullSourceCols + ` FROM pull_sources ORDER BY sort_order, created_at`)
	if err != nil {
		return nil, fmt.Errorf("ListPullSources: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []PullSource
	for rows.Next() {
		ps, err := scanPullSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ps)
	}
	return out, rows.Err()
}

// GetPullSource returns the pull source with the given id, or false when there
// is none.
func (r *Repo) GetPullSource(id string) (PullSource, bool, error) {
	row := r.db.QueryRow(`SELECT `+pullSourceCols+` FROM pull_sources WHERE id = ?`, id)
	ps, err := scanPullSource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return PullSource{}, false, nil
	}
	if err != nil {
		return PullSource{}, false, err
	}
	return ps, true, nil
}

// DeletePullSource removes the pull source with the given id; a missing id is
// not an error. Snapshots already pulled stay.
func (r *Repo) DeletePullSource(id string) error {
	if _, err := r.db.Exec(`DELETE FROM pull_sources WHERE id = ?`, id); err != nil {
		return fmt.Errorf("DeletePullSource: %w", err)
	}
	return nil
}

func scanPullSource(s scanner) (PullSource, error) {
	var ps PullSource
	var enabled int
	err := s.Scan(
		&ps.ID, &ps.Name, &ps.Repo, &ps.AppKeyEnc, &ps.CredsRef, &ps.Domain, &ps.Cadence,
		&ps.LimitDownload, &ps.LimitUpload, &ps.LastPullAt, &ps.LastPullOK, &ps.LastPullError,
		&ps.SnapshotsPulled, &enabled, &ps.CreatedAt, &ps.SortOrder,
	)
	if err != nil {
		return PullSource{}, fmt.Errorf("scanPullSource: %w", err)
	}
	ps.Enabled = enabled != 0
	return ps, nil
}
