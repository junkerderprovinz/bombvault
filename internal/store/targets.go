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
}

// UpsertTarget inserts or updates the target by container name.
// On conflict (container already exists), only appdata_paths is refreshed via
// the ON CONFLICT … DO UPDATE SET clause. id, created_at, and
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
		INSERT INTO targets (id, container_name, appdata_paths, include_in_schedule, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(container_name) DO UPDATE SET
		  appdata_paths = excluded.appdata_paths`,
		t.ID, t.ContainerName, string(pathsJSON),
		boolInt(t.IncludeInSchedule), t.CreatedAt,
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
		SELECT id, container_name, appdata_paths, include_in_schedule, created_at
		FROM targets WHERE container_name = ?`, name)
	return scanTarget(row)
}

// ListTargets returns all known targets.
func (r *Repo) ListTargets() ([]Target, error) {
	rows, err := r.db.Query(`
		SELECT id, container_name, appdata_paths, include_in_schedule, created_at
		FROM targets ORDER BY container_name`)
	if err != nil {
		return nil, fmt.Errorf("ListTargets: %w", err)
	}
	defer rows.Close()

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

// scanner abstracts *sql.Row and *sql.Rows so scanTarget works for both.
type scanner interface {
	Scan(dest ...any) error
}

func scanTarget(s scanner) (Target, error) {
	var t Target
	var pathsJSON string
	var include int
	err := s.Scan(&t.ID, &t.ContainerName, &pathsJSON, &include, &t.CreatedAt)
	if err != nil {
		return Target{}, fmt.Errorf("scanTarget: %w", err)
	}
	if err := json.Unmarshal([]byte(pathsJSON), &t.AppdataPaths); err != nil {
		return Target{}, fmt.Errorf("scanTarget unmarshal paths: %w", err)
	}
	t.IncludeInSchedule = include != 0
	return t, nil
}
