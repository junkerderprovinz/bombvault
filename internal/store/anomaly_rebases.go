package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// AnomalyScopeKey names one series the anomaly rules judge.
type AnomalyScopeKey struct{ Kind, ID string }

// RebaseAnomalyScopes records that BombVault itself changed what the given
// series back up at the time at, so their rules start learning again from the
// first backup that sees the change. An earlier time never replaces a later one.
func (r *Repo) RebaseAnomalyScopes(scopes []AnomalyScopeKey, at int64) error {
	if len(scopes) == 0 {
		return nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("RebaseAnomalyScopes begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit
	for _, sc := range scopes {
		if _, err := tx.Exec(`
			INSERT INTO anomaly_rebases (scope_kind, scope_id, at) VALUES (?, ?, ?)
			ON CONFLICT(scope_kind, scope_id) DO UPDATE SET at = MAX(at, excluded.at)`,
			sc.Kind, sc.ID, at); err != nil {
			return fmt.Errorf("RebaseAnomalyScopes: %w", err)
		}
	}
	return tx.Commit()
}

// AnomalyRebaseAt is when BombVault last changed what one series backs up, 0
// when it never did.
func (r *Repo) AnomalyRebaseAt(scopeKind, scopeID string) (int64, error) {
	var at int64
	err := r.db.QueryRow(`SELECT at FROM anomaly_rebases WHERE scope_kind = ? AND scope_id = ?`,
		scopeKind, scopeID).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("AnomalyRebaseAt: %w", err)
	}
	return at, nil
}

// ZFSMountpoints maps every dataset of every ZFS item's tree to where the host
// mounted it when it was last looked at. A dataset without a mountpoint is
// left out.
func (r *Repo) ZFSMountpoints() (map[string]string, error) {
	rows, err := r.db.Query(`SELECT dataset, host_mountpoint FROM zfs_members WHERE host_mountpoint <> ''`)
	if err != nil {
		return nil, fmt.Errorf("ZFSMountpoints: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	out := map[string]string{}
	for rows.Next() {
		var dataset, mountpoint string
		if err := rows.Scan(&dataset, &mountpoint); err != nil {
			return nil, fmt.Errorf("ZFSMountpoints: %w", err)
		}
		out[dataset] = mountpoint
	}
	return out, rows.Err()
}
