package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrEmptyPullRepo is returned when a restic pull source has no repo location.
var ErrEmptyPullRepo = errors.New("pull source location must not be empty")

// What a pull source fetches: restic snapshots out of the source's
// repository, or a ZFS replica of the source's ZFS items.
const (
	PullSourceRestic = "restic"
	PullSourceZFS    = "zfs"
)

// PullSource is another BombVault's repository whose snapshots this box copies
// into its own repository on its own schedule: off-site replication in reverse.
// Like a ReceivedRepo it keeps the other instance's restic password sealed.
type PullSource struct {
	ID string
	// MemberID is the source instance's id in the pairing group. It is empty
	// on a row from before pairing, which has to be paired again.
	MemberID string
	Name     string
	// Repo is the source instance's repository location (rest:, s3:, sftp:, b2:,
	// or a path under the host mount). The API refuses `rclone:`, as foreign.go
	// does: rclone reads its remotes from our own config and would authenticate
	// to a caller-chosen endpoint with our secrets.
	Repo string
	// ResticPasswordEnc is the source instance's restic password, sealed with
	// this instance's key (internal/secret). It is never logged or returned
	// in the clear, and like received_repos it stays out of the settings
	// export.
	ResticPasswordEnc []byte
	// LegacyAppKeyEnc is a source APP_KEY stored before pairing existed. It
	// is only read to convert it to ResticPasswordEnc and then cleared.
	LegacyAppKeyEnc []byte
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
	// Kind is PullSourceRestic or PullSourceZFS; an unset Kind is stored as
	// restic. Repo, the password, CredsRef, Domain and the limits belong to
	// a restic source, the ZFS fields and GrantState to a zfs one.
	Kind string
	// ZFSDatasets are the ids of the source's ZFS items, and ZFSPool and
	// ZFSRoot where on this host they land. An unset ZFSKeep is stored as
	// DefaultZFSReplicaKeep.
	ZFSDatasets []string
	ZFSPool     string
	ZFSRoot     string
	ZFSKeep     ZFSReplicaKeep
	// GrantState is the source's answer to the pull request, one of the
	// ZFSGrant states. Only SetPullSourceGrantState writes it.
	GrantState string
}

const pullSourceCols = `id, member_id, name, repo, restic_password_enc, app_key_enc, creds_ref, domain, cadence,
	limit_download, limit_upload, last_pull_at, last_pull_ok, last_pull_error,
	snapshots_pulled, enabled, created_at, sort_order,
	kind, zfs_datasets, zfs_pool, zfs_root, zfs_keep, grant_state`

// CreatePullSource inserts a new pull source, assigning an ID and CreatedAt
// when they are unset, and returns the stored row.
func (r *Repo) CreatePullSource(ps PullSource) (PullSource, error) {
	datasets, keep, err := pullSourceZFSColumns(&ps)
	if err != nil {
		return PullSource{}, fmt.Errorf("CreatePullSource: %w", err)
	}
	if ps.ID == "" {
		ps.ID = newID()
	}
	if ps.CreatedAt == 0 {
		ps.CreatedAt = time.Now().Unix()
	}
	ps.ResticPasswordEnc = notNullBlob(ps.ResticPasswordEnc)
	ps.LegacyAppKeyEnc = notNullBlob(ps.LegacyAppKeyEnc)
	_, err = r.db.Exec(`
		INSERT INTO pull_sources (`+pullSourceCols+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ps.ID, ps.MemberID, ps.Name, ps.Repo, ps.ResticPasswordEnc, ps.LegacyAppKeyEnc, ps.CredsRef, ps.Domain, ps.Cadence,
		ps.LimitDownload, ps.LimitUpload, ps.LastPullAt, nullBool(ps.LastPullOK), ps.LastPullError,
		ps.SnapshotsPulled, boolInt(ps.Enabled), ps.CreatedAt, ps.SortOrder,
		ps.Kind, datasets, ps.ZFSPool, ps.ZFSRoot, keep, ps.GrantState,
	)
	if err != nil {
		return PullSource{}, fmt.Errorf("CreatePullSource: %w", err)
	}
	return ps, nil
}

// UpdatePullSource updates the pull source identified by ps.ID, all but its
// grant state. Updating a missing id is not an error.
func (r *Repo) UpdatePullSource(ps PullSource) error {
	datasets, keep, err := pullSourceZFSColumns(&ps)
	if err != nil {
		return fmt.Errorf("UpdatePullSource: %w", err)
	}
	ps.ResticPasswordEnc = notNullBlob(ps.ResticPasswordEnc)
	ps.LegacyAppKeyEnc = notNullBlob(ps.LegacyAppKeyEnc)
	_, err = r.db.Exec(`
		UPDATE pull_sources SET
		  member_id           = ?,
		  name                = ?,
		  repo                = ?,
		  restic_password_enc = ?,
		  app_key_enc         = ?,
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
		  sort_order       = ?,
		  kind             = ?,
		  zfs_datasets     = ?,
		  zfs_pool         = ?,
		  zfs_root         = ?,
		  zfs_keep         = ?
		WHERE id = ?`,
		ps.MemberID, ps.Name, ps.Repo, ps.ResticPasswordEnc, ps.LegacyAppKeyEnc, ps.CredsRef, ps.Domain, ps.Cadence,
		ps.LimitDownload, ps.LimitUpload, ps.LastPullAt, nullBool(ps.LastPullOK), ps.LastPullError,
		ps.SnapshotsPulled, boolInt(ps.Enabled), ps.SortOrder,
		ps.Kind, datasets, ps.ZFSPool, ps.ZFSRoot, keep, ps.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdatePullSource: %w", err)
	}
	return nil
}

// pullSourceZFSColumns fills in the kind and keep of a row that leaves them
// unset, refuses a restic row without a location, and encodes the ZFS
// columns.
func pullSourceZFSColumns(ps *PullSource) (string, string, error) {
	if ps.Kind == "" {
		ps.Kind = PullSourceRestic
	}
	if ps.Kind == PullSourceRestic && strings.TrimSpace(ps.Repo) == "" {
		return "", "", ErrEmptyPullRepo
	}
	if ps.ZFSKeep.Preset == "" {
		ps.ZFSKeep = DefaultZFSReplicaKeep
	}
	datasets, err := marshalList(ps.ZFSDatasets)
	if err != nil {
		return "", "", err
	}
	keep, err := json.Marshal(ps.ZFSKeep)
	if err != nil {
		return "", "", err
	}
	return datasets, string(keep), nil
}

// SetPullSourceGrantState records the source's answer to a zfs pull's
// request. It touches nothing else, so an answer arriving during an edit
// cannot undo the edit.
func (r *Repo) SetPullSourceGrantState(id, state string) error {
	if _, err := r.db.Exec(`UPDATE pull_sources SET grant_state = ? WHERE id = ?`, state, id); err != nil {
		return fmt.Errorf("SetPullSourceGrantState: %w", err)
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

// DeletePullSource removes the pull source with the given id and the replica
// state kept under it; a missing id is not an error. Snapshots and datasets
// already pulled stay.
func (r *Repo) DeletePullSource(id string) error {
	return r.inTx(func(tx *sql.Tx) error {
		if err := deleteZFSReplicaRows(tx, id); err != nil {
			return fmt.Errorf("DeletePullSource replica: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM pull_sources WHERE id = ?`, id); err != nil {
			return fmt.Errorf("DeletePullSource: %w", err)
		}
		return nil
	})
}

func scanPullSource(s scanner) (PullSource, error) {
	var ps PullSource
	var enabled int
	var datasets, keep string
	err := s.Scan(
		&ps.ID, &ps.MemberID, &ps.Name, &ps.Repo, &ps.ResticPasswordEnc, &ps.LegacyAppKeyEnc, &ps.CredsRef, &ps.Domain, &ps.Cadence,
		&ps.LimitDownload, &ps.LimitUpload, &ps.LastPullAt, &ps.LastPullOK, &ps.LastPullError,
		&ps.SnapshotsPulled, &enabled, &ps.CreatedAt, &ps.SortOrder,
		&ps.Kind, &datasets, &ps.ZFSPool, &ps.ZFSRoot, &keep, &ps.GrantState,
	)
	if err != nil {
		return PullSource{}, fmt.Errorf("scanPullSource: %w", err)
	}
	if err := json.Unmarshal([]byte(datasets), &ps.ZFSDatasets); err != nil {
		return PullSource{}, fmt.Errorf("scanPullSource unmarshal zfs_datasets: %w", err)
	}
	if err := json.Unmarshal([]byte(keep), &ps.ZFSKeep); err != nil {
		return PullSource{}, fmt.Errorf("scanPullSource unmarshal zfs_keep: %w", err)
	}
	ps.Enabled = enabled != 0
	return ps, nil
}

// NeedsPairing reports whether the row predates pairing and names no member.
func (ps PullSource) NeedsPairing() bool { return ps.MemberID == "" }
