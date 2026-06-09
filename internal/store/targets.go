package store

import (
	"encoding/json"
	"fmt"
	"time"
)

// Target represents a container that BombVault can back up.
type Target struct {
	ID                string
	ContainerName     string
	AppdataPaths      []string
	IncludeInSchedule bool
	CreatedAt         int64
	// Definition is an opaque JSON blob persisted at backup time. It carries the
	// container's recreate recipe (inspect + template XML) so restore works even
	// after the container has been deleted from the host.
	Definition string
}

// UpsertTarget inserts or updates the target by container name.
// On conflict (container already exists), appdata_paths and definition are
// refreshed via the ON CONFLICT … DO UPDATE SET clause. id, created_at, and
// include_in_schedule are preserved from the original row — include_in_schedule
// is owned exclusively by SetInclude and must never be reset here.
// Returns the authoritative Target (with the original ID when a conflict fires).
func (r *Repo) UpsertTarget(t Target) (Target, error) {
	if t.ID == "" {
		t.ID = newID()
	}
	if t.CreatedAt == 0 {
		t.CreatedAt = time.Now().Unix()
	}

	pathsJSON, err := json.Marshal(t.AppdataPaths)
	if err != nil {
		return Target{}, fmt.Errorf("UpsertTarget marshal paths: %w", err)
	}

	_, err = r.db.Exec(`
		INSERT INTO targets (id, container_name, appdata_paths, include_in_schedule, created_at, definition)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(container_name) DO UPDATE SET
		  appdata_paths = excluded.appdata_paths,
		  definition    = excluded.definition`,
		t.ID, t.ContainerName, string(pathsJSON),
		boolInt(t.IncludeInSchedule), t.CreatedAt, t.Definition,
	)
	if err != nil {
		return Target{}, fmt.Errorf("UpsertTarget: %w", err)
	}

	// Re-read to get the authoritative record (conflict may have preserved the original ID).
	return r.GetTargetByContainer(t.ContainerName)
}

// GetTargetByContainer returns the target for the named container.
func (r *Repo) GetTargetByContainer(name string) (Target, error) {
	row := r.db.QueryRow(`
		SELECT id, container_name, appdata_paths, include_in_schedule, created_at, definition
		FROM targets WHERE container_name = ?`, name)
	return scanTarget(row)
}

// ListTargets returns all known targets.
func (r *Repo) ListTargets() ([]Target, error) {
	rows, err := r.db.Query(`
		SELECT id, container_name, appdata_paths, include_in_schedule, created_at, definition
		FROM targets ORDER BY container_name`)
	if err != nil {
		return nil, fmt.Errorf("ListTargets: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []Target
	for rows.Next() {
		t, err := scanTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SetInclude updates the include_in_schedule flag for a container.
func (r *Repo) SetInclude(containerName string, include bool) error {
	res, err := r.db.Exec(`
		UPDATE targets SET include_in_schedule = ? WHERE container_name = ?`,
		boolInt(include), containerName)
	if err != nil {
		return fmt.Errorf("SetInclude: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("SetInclude: container %q not found", containerName)
	}
	return nil
}

// DeleteTarget removes a target and ALL its run history by container name, in a
// single transaction. It is a no-op (no error) if the target does not exist.
// Used to forget a container that is no longer installed once its backups have
// been deleted from the restic repo.
func (r *Repo) DeleteTarget(name string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("DeleteTarget begin: %w", err)
	}
	// Child runs first (runs.target_id references targets.id).
	if _, err := tx.Exec(
		`DELETE FROM runs WHERE target_id IN (SELECT id FROM targets WHERE container_name = ?)`, name,
	); err != nil {
		tx.Rollback() //nolint:errcheck,gosec // best-effort rollback; original error takes priority
		return fmt.Errorf("DeleteTarget runs: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM targets WHERE container_name = ?`, name); err != nil {
		tx.Rollback() //nolint:errcheck,gosec // best-effort rollback; original error takes priority
		return fmt.Errorf("DeleteTarget: %w", err)
	}
	return tx.Commit()
}

// scanner abstracts *sql.Row and *sql.Rows so scanTarget works for both.
type scanner interface {
	Scan(dest ...any) error
}

func scanTarget(s scanner) (Target, error) {
	var t Target
	var pathsJSON string
	var include int
	err := s.Scan(&t.ID, &t.ContainerName, &pathsJSON, &include, &t.CreatedAt, &t.Definition)
	if err != nil {
		return Target{}, fmt.Errorf("scanTarget: %w", err)
	}
	if err := json.Unmarshal([]byte(pathsJSON), &t.AppdataPaths); err != nil {
		return Target{}, fmt.Errorf("scanTarget unmarshal paths: %w", err)
	}
	t.IncludeInSchedule = include != 0
	return t, nil
}
