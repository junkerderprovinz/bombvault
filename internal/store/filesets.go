package store

import (
	"encoding/json"
	"fmt"
	"time"
)

// FileSet represents one named host folder the files domain backs up (#62).
type FileSet struct {
	ID string
	// Name is the user-visible label and the restic tag/item key
	// (fileset:<Name>); ID is stable so renames never orphan run history
	// (runs.target_id = file_sets.id).
	Name string
	// Path is a relative subpath under the host mount root (like
	// Settings.ContainersPath), resolved with paths.Resolve at backup time.
	Path string
	// Excludes are restic --exclude patterns applied to this set's backup.
	Excludes []string
	// Enabled gates the set's participation in scheduled and whole-domain runs.
	Enabled   bool
	CreatedAt int64
}

// CreateFileSet inserts a new file set. An empty ID is assigned via newID();
// a duplicate name fails (name is UNIQUE). Returns the stored FileSet.
func (r *Repo) CreateFileSet(fs FileSet) (FileSet, error) {
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

	_, err = r.db.Exec(`
		INSERT INTO file_sets (id, name, path, excludes, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		fs.ID, fs.Name, fs.Path, string(exJSON), boolInt(fs.Enabled), fs.CreatedAt,
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

// ListFileSets returns all file sets ordered by name.
func (r *Repo) ListFileSets() ([]FileSet, error) {
	rows, err := r.db.Query(`
		SELECT id, name, path, excludes, enabled, created_at
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
		SELECT id, name, path, excludes, enabled, created_at
		FROM file_sets WHERE id = ?`, id)
	return scanFileSet(row)
}

// GetFileSetByName returns the file set with the given (unique) name.
func (r *Repo) GetFileSetByName(name string) (FileSet, error) {
	row := r.db.QueryRow(`
		SELECT id, name, path, excludes, enabled, created_at
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

// DeleteFileSet removes a file set and ALL its run history by id, in a single
// transaction. It is a no-op (no error) if the set does not exist.
func (r *Repo) DeleteFileSet(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("DeleteFileSet begin: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM runs WHERE target_id = ?`, id); err != nil {
		tx.Rollback() //nolint:errcheck,gosec // best-effort rollback; original error takes priority
		return fmt.Errorf("DeleteFileSet runs: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM file_sets WHERE id = ?`, id); err != nil {
		tx.Rollback() //nolint:errcheck,gosec // best-effort rollback; original error takes priority
		return fmt.Errorf("DeleteFileSet: %w", err)
	}
	return tx.Commit()
}

func scanFileSet(s scanner) (FileSet, error) {
	var fs FileSet
	var exJSON string
	var enabled int
	err := s.Scan(&fs.ID, &fs.Name, &fs.Path, &exJSON, &enabled, &fs.CreatedAt)
	if err != nil {
		return FileSet{}, fmt.Errorf("scanFileSet: %w", err)
	}
	if err := json.Unmarshal([]byte(exJSON), &fs.Excludes); err != nil {
		return FileSet{}, fmt.Errorf("scanFileSet unmarshal excludes: %w", err)
	}
	fs.Enabled = enabled != 0
	return fs, nil
}
