package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// FileSet is one named host folder the files domain backs up.
type FileSet struct {
	ID string
	// Name is the user-visible label and the restic tag (fileset:<Name>). Runs
	// reference the stable ID, so a rename keeps the run history.
	Name string
	// Path is a relative subpath under the host mount root (like
	// Settings.ContainersPath), resolved with paths.Resolve at backup time.
	Path string
	// Excludes are restic --exclude patterns applied to this set's backup.
	Excludes []string
	// Enabled gates the set's participation in scheduled and whole-domain runs.
	Enabled bool
	// ScheduleCadence is the set's optional per-item schedule. Empty follows the
	// Folders domain schedule. A cadence gives the set its own entry and takes
	// it out of the domain run and Backup Everything; "off" excludes it from
	// scheduling. It only applies while per-item schedules are switched on.
	// Only SetFileSetScheduleCadence writes it, so editing the name or path
	// cannot drop it.
	ScheduleCadence string
	// Repo is the ID of the set's own named repository (a RoleRepo row in
	// offsite_targets), not a location. Empty means the Folders domain
	// repository (Settings.FilesPath). The API tier turns the ID into a
	// location (itemRepoPath). CreateFileSet writes it with the row and
	// WritePlacement afterwards, never UpdateFileSet, so renaming a set cannot
	// move its backups to another repository.
	Repo string
	// RepoChosen says whether Repo is settled. An open row has an empty Repo and
	// takes the default's location at its first backup.
	RepoChosen RepoChoice
	// SelectedPaths is the set's optional tree selection, in the same flat
	// encoding as the containers' backupPaths: absolute paths under the mount
	// root are included roots, "!"-prefixed entries are deselected branches
	// (internal/api/selection.go defines the meaning and the web's
	// selectionTree.ts mirrors it). nil, a NULL column, means the tree was
	// never used and the backup takes the whole resolved Path. The store does
	// not interpret the entries. Only SetFileSetSelectedPaths and
	// UpdateFileSetClearingSelection write it.
	SelectedPaths []string
	CreatedAt     int64
}

// CreateFileSet inserts a new file set. An empty ID is assigned via newID();
// a duplicate name fails (name is UNIQUE). Returns the stored FileSet.
func (r *Repo) CreateFileSet(fs FileSet) (FileSet, error) {
	if err := checkRepoChoice(fs.Repo, fs.RepoChosen); err != nil {
		return FileSet{}, fmt.Errorf("CreateFileSet: %w", err)
	}
	if fs.ID == "" {
		fs.ID = newID()
	}
	if fs.CreatedAt == 0 {
		fs.CreatedAt = time.Now().Unix()
	}
	if fs.Excludes == nil {
		fs.Excludes = []string{}
	}
	exJSON, err := json.Marshal(fs.Excludes)
	if err != nil {
		return FileSet{}, fmt.Errorf("CreateFileSet marshal excludes: %w", err)
	}

	// Unlike UpdateFileSet, the INSERT includes repo: a new set has nothing to
	// overwrite, and a separate setter call would leave a window in which the
	// set sits on the domain repository while the caller believes it is on the
	// chosen one.
	_, err = r.db.Exec(`
		INSERT INTO file_sets (id, name, path, excludes, enabled, created_at, repo, repo_chosen)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		fs.ID, fs.Name, fs.Path, string(exJSON), boolInt(fs.Enabled), fs.CreatedAt, fs.Repo, fs.RepoChosen,
	)
	if err != nil {
		return FileSet{}, fmt.Errorf("CreateFileSet: %w", err)
	}
	return fs, nil
}

// UpdateFileSet updates name, path, excludes, and enabled for the set with
// fs.ID. ID and created_at are immutable.
func (r *Repo) UpdateFileSet(fs FileSet) error {
	if fs.Excludes == nil {
		fs.Excludes = []string{}
	}
	exJSON, err := json.Marshal(fs.Excludes)
	if err != nil {
		return fmt.Errorf("UpdateFileSet marshal excludes: %w", err)
	}
	res, err := r.db.Exec(`
		UPDATE file_sets SET name = ?, path = ?, excludes = ?, enabled = ? WHERE id = ?`,
		fs.Name, fs.Path, string(exJSON), boolInt(fs.Enabled), fs.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateFileSet: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("UpdateFileSet: file set %q not found", fs.ID)
	}
	return nil
}

// UpdateFileSetClearingSelection is UpdateFileSet plus a reset of
// selected_paths to NULL, in one statement. A path change moves the anchor the
// stored selection was validated against, so the selection has to go, and a
// single statement cannot leave the new path saved next to the old selection.
func (r *Repo) UpdateFileSetClearingSelection(fs FileSet) error {
	if fs.Excludes == nil {
		fs.Excludes = []string{}
	}
	exJSON, err := json.Marshal(fs.Excludes)
	if err != nil {
		return fmt.Errorf("UpdateFileSetClearingSelection marshal excludes: %w", err)
	}
	res, err := r.db.Exec(`
		UPDATE file_sets SET name = ?, path = ?, excludes = ?, enabled = ?, selected_paths = NULL
		WHERE id = ?`,
		fs.Name, fs.Path, string(exJSON), boolInt(fs.Enabled), fs.ID,
	)
	if err != nil {
		return fmt.Errorf("UpdateFileSetClearingSelection: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("UpdateFileSetClearingSelection: file set %q not found", fs.ID)
	}
	return nil
}

// ListFileSets returns all file sets ordered by name.
func (r *Repo) ListFileSets() ([]FileSet, error) {
	rows, err := r.db.Query(`
		SELECT id, name, path, excludes, enabled, schedule_cadence, selected_paths, repo, repo_chosen, created_at
		FROM file_sets ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("ListFileSets: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []FileSet
	for rows.Next() {
		fs, err := scanFileSet(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, fs)
	}
	return out, rows.Err()
}

// GetFileSet returns the file set with the given id.
func (r *Repo) GetFileSet(id string) (FileSet, error) {
	row := r.db.QueryRow(`
		SELECT id, name, path, excludes, enabled, schedule_cadence, selected_paths, repo, repo_chosen, created_at
		FROM file_sets WHERE id = ?`, id)
	return scanFileSet(row)
}

// GetFileSetByName returns the file set with the given (unique) name.
func (r *Repo) GetFileSetByName(name string) (FileSet, error) {
	row := r.db.QueryRow(`
		SELECT id, name, path, excludes, enabled, schedule_cadence, selected_paths, repo, repo_chosen, created_at
		FROM file_sets WHERE name = ?`, name)
	return scanFileSet(row)
}

// SetFileSetEnabled updates the enabled flag for the set with the given id.
func (r *Repo) SetFileSetEnabled(id string, enabled bool) error {
	res, err := r.db.Exec(`UPDATE file_sets SET enabled = ? WHERE id = ?`,
		boolInt(enabled), id)
	if err != nil {
		return fmt.Errorf("SetFileSetEnabled: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("SetFileSetEnabled: file set %q not found", id)
	}
	return nil
}

// SetFileSetScheduleCadence writes a file set's per-item schedule override. An
// empty string puts the set back on the Folders domain schedule. It is
// separate from UpdateFileSet so a form that does not know about the cadence
// cannot clear it, the same split SetScheduleCadence makes for containers.
func (r *Repo) SetFileSetScheduleCadence(id, cadence string) error {
	res, err := r.db.Exec(
		`UPDATE file_sets SET schedule_cadence = ? WHERE id = ?`, cadence, id)
	if err != nil {
		return fmt.Errorf("SetFileSetScheduleCadence: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("SetFileSetScheduleCadence: no file set %q", id)
	}
	return nil
}

// SetFileSetSelectedPaths writes a file set's tree selection as JSON. A nil
// slice stores SQL NULL rather than '[]': NULL means the tree was never used,
// while '[]' would read back as an empty non-nil slice. The entries are not
// interpreted here. It is separate from UpdateFileSet so an edit that does not
// know about the selection cannot clear it.
func (r *Repo) SetFileSetSelectedPaths(id string, selected []string) error {
	var encoded any // nil binds as SQL NULL
	if selected != nil {
		b, err := json.Marshal(selected)
		if err != nil {
			return fmt.Errorf("SetFileSetSelectedPaths marshal: %w", err)
		}
		encoded = string(b)
	}
	res, err := r.db.Exec(
		`UPDATE file_sets SET selected_paths = ? WHERE id = ?`, encoded, id)
	if err != nil {
		return fmt.Errorf("SetFileSetSelectedPaths: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("SetFileSetSelectedPaths: no file set %q", id)
	}
	return nil
}

// DeleteFileSet removes a file set and all its run history in one
// transaction. Deleting a set that does not exist is not an error.
func (r *Repo) DeleteFileSet(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("DeleteFileSet begin: %w", err)
	}
	// The name, read before the row goes: its copy rule is keyed by identity
	// (fileset:<Name>), not by id, and a rule surviving the set would block a
	// later set from taking the freed name.
	var name string
	if err := tx.QueryRow(`SELECT name FROM file_sets WHERE id = ?`, id).Scan(&name); err != nil && !errors.Is(err, sql.ErrNoRows) {
		tx.Rollback() //nolint:errcheck,gosec // best-effort rollback; original error takes priority
		return fmt.Errorf("DeleteFileSet name: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM runs WHERE target_id = ?`, id); err != nil {
		tx.Rollback() //nolint:errcheck,gosec // best-effort rollback; original error takes priority
		return fmt.Errorf("DeleteFileSet runs: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM file_sets WHERE id = ?`, id); err != nil {
		tx.Rollback() //nolint:errcheck,gosec // best-effort rollback; original error takes priority
		return fmt.Errorf("DeleteFileSet: %w", err)
	}
	if name != "" {
		if _, err := tx.Exec(`DELETE FROM offsite_copy_rules WHERE domain = 'files' AND identity = ?`, "fileset:"+name); err != nil {
			tx.Rollback() //nolint:errcheck,gosec // best-effort rollback; original error takes priority
			return fmt.Errorf("DeleteFileSet copy rule: %w", err)
		}
	}
	return tx.Commit()
}

func scanFileSet(s scanner) (FileSet, error) {
	var fs FileSet
	var exJSON string
	var enabled int
	// selected_paths is nullable and NULL for most rows; scanning it into a
	// plain string would fail on every one of them.
	var selJSON *string
	err := s.Scan(&fs.ID, &fs.Name, &fs.Path, &exJSON, &enabled, &fs.ScheduleCadence, &selJSON, &fs.Repo, &fs.RepoChosen, &fs.CreatedAt)
	if err != nil {
		return FileSet{}, fmt.Errorf("scanFileSet: %w", err)
	}
	if err := json.Unmarshal([]byte(exJSON), &fs.Excludes); err != nil {
		return FileSet{}, fmt.Errorf("scanFileSet unmarshal excludes: %w", err)
	}
	if selJSON != nil {
		if err := json.Unmarshal([]byte(*selJSON), &fs.SelectedPaths); err != nil {
			return FileSet{}, fmt.Errorf("scanFileSet unmarshal selected_paths: %w", err)
		}
	}
	fs.Enabled = enabled != 0
	return fs, nil
}
