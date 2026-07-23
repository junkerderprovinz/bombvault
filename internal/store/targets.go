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
	// PreHook / PostHook are optional shell commands run inside the container via
	// `sh -c` before/after a backup. Owned by SetHooks (never reset by Upsert).
	PreHook  string
	PostHook string
	// SelectedPaths is the user's explicit set of folders to back up
	// (container-translated paths). Empty means "use the automatic appdata
	// detection". Owned by SetBackupPaths (never reset by Upsert).
	SelectedPaths []string
	// StopContainers are other container names to stop for the duration of this
	// container's backup (e.g. a database) and start again afterwards. Owned by
	// SetStopContainers (never reset by Upsert).
	StopContainers []string
	// Excludes are restic --exclude patterns applied to this container's backup.
	// Owned by SetExcludes (never reset by Upsert).
	Excludes []string
	// UpdateAfterBackup opts this container into a post-backup image update (#52):
	// after a successful backup, pull the image and recreate the container if a
	// newer image is available. Owned by SetUpdateAfterBackup (never reset by
	// Upsert). Off by default.
	UpdateAfterBackup bool
	// LastUpdateCheck / LastUpdateResult record the outcome of the most recent
	// post-backup update check, so "checked and current" is distinguishable from
	// "never reached" without a noise run row per night. LastUpdateCheck is the
	// unix time the check completed (0 = never); LastUpdateResult is '' |
	// 'up-to-date' | 'updated' | 'failed'. Owned by SetUpdateCheck (never reset
	// by Upsert).
	LastUpdateCheck  int64
	LastUpdateResult string
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
	selJSON, err := json.Marshal(t.SelectedPaths)
	if err != nil {
		return Target{}, fmt.Errorf("UpsertTarget marshal selected: %w", err)
	}
	stopJSON, err := json.Marshal(t.StopContainers)
	if err != nil {
		return Target{}, fmt.Errorf("UpsertTarget marshal stop: %w", err)
	}
	exJSON, err := json.Marshal(t.Excludes)
	if err != nil {
		return Target{}, fmt.Errorf("UpsertTarget marshal excludes: %w", err)
	}

	// selected_paths, stop_containers and excludes are owned by their setters and
	// intentionally NOT in the ON CONFLICT update set, so a backup's UpsertTarget
	// never clobbers the user's choices (same pattern as include_in_schedule/hooks).
	_, err = r.db.Exec(`
		INSERT INTO targets (id, container_name, appdata_paths, include_in_schedule, created_at, definition, pre_hook, post_hook, selected_paths, stop_containers, excludes, update_after_backup)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(container_name) DO UPDATE SET
		  appdata_paths = excluded.appdata_paths,
		  definition    = excluded.definition`,
		t.ID, t.ContainerName, string(pathsJSON),
		boolInt(t.IncludeInSchedule), t.CreatedAt, t.Definition, t.PreHook, t.PostHook, string(selJSON), string(stopJSON), string(exJSON), boolInt(t.UpdateAfterBackup),
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
		SELECT id, container_name, appdata_paths, include_in_schedule, created_at, definition, pre_hook, post_hook, selected_paths, stop_containers, excludes, update_after_backup, last_update_check, last_update_result
		FROM targets WHERE container_name = ?`, name)
	return scanTarget(row)
}

// ListTargets returns all known targets.
func (r *Repo) ListTargets() ([]Target, error) {
	rows, err := r.db.Query(`
		SELECT id, container_name, appdata_paths, include_in_schedule, created_at, definition, pre_hook, post_hook, selected_paths, stop_containers, excludes, update_after_backup, last_update_check, last_update_result
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

// ListTargetsScheduleOrder returns the same targets as ListTargets but ordered for
// a SCHEDULED run: never-backed-up targets first, then by oldest successful backup,
// then alphabetically. This keeps a slow or interrupted nightly run from
// perpetually starving the same alphabetical tail — a newly added container, or one
// that missed last night, leads the next run instead of always coming last (#95).
// The UI keeps using ListTargets (alphabetical) for a stable display order.
func (r *Repo) ListTargetsScheduleOrder() ([]Target, error) {
	rows, err := r.db.Query(`
		SELECT t.id, t.container_name, t.appdata_paths, t.include_in_schedule, t.created_at, t.definition, t.pre_hook, t.post_hook, t.selected_paths, t.stop_containers, t.excludes, t.update_after_backup, t.last_update_check, t.last_update_result
		FROM targets t
		LEFT JOIN (
			SELECT target_id, MAX(finished_at) AS last_ok
			FROM runs
			WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL
			GROUP BY target_id
		) r ON r.target_id = t.id
		ORDER BY (r.last_ok IS NULL) DESC, r.last_ok ASC, t.container_name ASC`)
	if err != nil {
		return nil, fmt.Errorf("ListTargetsScheduleOrder: %w", err)
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

// SetHooks updates the pre/post-backup hook commands for a container, creating
// the target row if it does not exist yet (so hooks can be set before the first
// backup).
func (r *Repo) SetHooks(containerName, preHook, postHook string) error {
	res, err := r.db.Exec(
		`UPDATE targets SET pre_hook = ?, post_hook = ? WHERE container_name = ?`,
		preHook, postHook, containerName)
	if err != nil {
		return fmt.Errorf("SetHooks: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := r.UpsertTarget(Target{ContainerName: containerName, PreHook: preHook, PostHook: postHook}); err != nil {
			return fmt.Errorf("SetHooks create target: %w", err)
		}
	}
	return nil
}

// SetUpdateAfterBackup toggles the post-backup image update for a container (#52),
// creating the target row if it does not exist yet (so it can be set before the
// first backup). Never reset by UpsertTarget.
func (r *Repo) SetUpdateAfterBackup(containerName string, updateAfterBackup bool) error {
	res, err := r.db.Exec(
		`UPDATE targets SET update_after_backup = ? WHERE container_name = ?`,
		boolInt(updateAfterBackup), containerName)
	if err != nil {
		return fmt.Errorf("SetUpdateAfterBackup: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := r.UpsertTarget(Target{ContainerName: containerName, UpdateAfterBackup: updateAfterBackup}); err != nil {
			return fmt.Errorf("SetUpdateAfterBackup create target: %w", err)
		}
	}
	return nil
}

// SetUpdateCheck records the outcome of a completed post-backup update check
// for a container: at is the unix time it finished, result is 'up-to-date' |
// 'updated' | 'failed'. Plain UPDATE (no create-on-miss): the check only ever
// runs right after a backup, which has already upserted the target row. Owned
// by this setter; never reset by UpsertTarget.
func (r *Repo) SetUpdateCheck(containerName string, at int64, result string) error {
	res, err := r.db.Exec(
		`UPDATE targets SET last_update_check = ?, last_update_result = ? WHERE container_name = ?`,
		at, result, containerName)
	if err != nil {
		return fmt.Errorf("SetUpdateCheck: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("SetUpdateCheck: container %q not found", containerName)
	}
	return nil
}

// SetBackupPaths sets the explicit backup-folder selection (container-translated
// paths) for a container, creating the target row if it does not exist yet. An
// empty slice clears the selection so backups fall back to automatic appdata
// detection. Owned by this setter; never reset by UpsertTarget.
func (r *Repo) SetBackupPaths(containerName string, selected []string) error {
	if selected == nil {
		selected = []string{}
	}
	selJSON, err := json.Marshal(selected)
	if err != nil {
		return fmt.Errorf("SetBackupPaths marshal: %w", err)
	}
	res, err := r.db.Exec(
		`UPDATE targets SET selected_paths = ? WHERE container_name = ?`,
		string(selJSON), containerName)
	if err != nil {
		return fmt.Errorf("SetBackupPaths: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := r.UpsertTarget(Target{ContainerName: containerName, SelectedPaths: selected}); err != nil {
			return fmt.Errorf("SetBackupPaths create target: %w", err)
		}
	}
	return nil
}

// SetStopContainers sets the list of other container names to stop during this
// container's backup, creating the target row if it does not exist yet. An empty
// slice clears the list. Owned by this setter; never reset by UpsertTarget.
func (r *Repo) SetStopContainers(containerName string, stop []string) error {
	if stop == nil {
		stop = []string{}
	}
	stopJSON, err := json.Marshal(stop)
	if err != nil {
		return fmt.Errorf("SetStopContainers marshal: %w", err)
	}
	res, err := r.db.Exec(
		`UPDATE targets SET stop_containers = ? WHERE container_name = ?`,
		string(stopJSON), containerName)
	if err != nil {
		return fmt.Errorf("SetStopContainers: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := r.UpsertTarget(Target{ContainerName: containerName, StopContainers: stop}); err != nil {
			return fmt.Errorf("SetStopContainers create target: %w", err)
		}
	}
	return nil
}

// SetExcludes sets the restic --exclude patterns for a container's backup,
// creating the target row if it does not exist yet. An empty slice clears the
// patterns. Owned by this setter; never reset by UpsertTarget.
func (r *Repo) SetExcludes(containerName string, excludes []string) error {
	if excludes == nil {
		excludes = []string{}
	}
	exJSON, err := json.Marshal(excludes)
	if err != nil {
		return fmt.Errorf("SetExcludes marshal: %w", err)
	}
	res, err := r.db.Exec(
		`UPDATE targets SET excludes = ? WHERE container_name = ?`,
		string(exJSON), containerName)
	if err != nil {
		return fmt.Errorf("SetExcludes: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := r.UpsertTarget(Target{ContainerName: containerName, Excludes: excludes}); err != nil {
			return fmt.Errorf("SetExcludes create target: %w", err)
		}
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
	var pathsJSON, selJSON, stopJSON, exJSON string
	var include, updateAfter int
	err := s.Scan(&t.ID, &t.ContainerName, &pathsJSON, &include, &t.CreatedAt, &t.Definition, &t.PreHook, &t.PostHook, &selJSON, &stopJSON, &exJSON, &updateAfter, &t.LastUpdateCheck, &t.LastUpdateResult)
	if err != nil {
		return Target{}, fmt.Errorf("scanTarget: %w", err)
	}
	if err := json.Unmarshal([]byte(pathsJSON), &t.AppdataPaths); err != nil {
		return Target{}, fmt.Errorf("scanTarget unmarshal paths: %w", err)
	}
	if err := json.Unmarshal([]byte(selJSON), &t.SelectedPaths); err != nil {
		return Target{}, fmt.Errorf("scanTarget unmarshal selected: %w", err)
	}
	if err := json.Unmarshal([]byte(stopJSON), &t.StopContainers); err != nil {
		return Target{}, fmt.Errorf("scanTarget unmarshal stop: %w", err)
	}
	if err := json.Unmarshal([]byte(exJSON), &t.Excludes); err != nil {
		return Target{}, fmt.Errorf("scanTarget unmarshal excludes: %w", err)
	}
	t.IncludeInSchedule = include != 0
	t.UpdateAfterBackup = updateAfter != 0
	return t, nil
}
