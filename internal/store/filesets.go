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
	Enabled bool
	// ScheduleCadence is this set's OPTIONAL per-item schedule override (#199,
	// the same mechanism containers and VMs got in #121). Empty means "follow the
	// Folders domain schedule", which is what every set did before this existed.
	// A concrete cadence takes the set OUT of the domain run and out of Backup
	// Everything and gives it its own entry; the literal "off" excludes it from
	// scheduling altogether. Only honoured while the per-item-schedules feature
	// toggle is on, so an install that never turns it on behaves exactly as it
	// did. Owned by SetFileSetScheduleCadence, never by UpdateFileSet, so an
	// ordinary edit of the name or path cannot silently drop a cadence.
	ScheduleCadence string
	// SelectedPaths is the set's OPTIONAL tree selection (Phase 4 file-sets
	// parity, D-03/D-05): the same flat encoding as the containers' flat
	// backupPaths set — bare mount-root-space absolute paths are included
	// roots, "!"-prefixed entries are deselected branches (internal/api/
	// selection.go owns the meaning of "!"; web selectionTree.ts mirrors it).
	// nil means the column is NULL: "never touched by the tree" — the legacy
	// switch that keeps the backup compiling to the single positional
	// [resolved Path] byte-identically. Opaque to this package: the store
	// marshals and scans the JSON blob and interprets nothing; normalization
	// and backup-time compilation live in the API tier. Owned by
	// SetFileSetSelectedPaths, never by UpdateFileSet (same rationale as the
	// cadence above, verbatim: an edit that does not know about the selection
	// must not be able to clear one by omitting it).
	SelectedPaths []string
	CreatedAt     int64
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
		SELECT id, name, path, excludes, enabled, schedule_cadence, selected_paths, created_at
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
		SELECT id, name, path, excludes, enabled, schedule_cadence, selected_paths, created_at
		FROM file_sets WHERE id = ?`, id)
	return scanFileSet(row)
}

// GetFileSetByName returns the file set with the given (unique) name.
func (r *Repo) GetFileSetByName(name string) (FileSet, error) {
	row := r.db.QueryRow(`
		SELECT id, name, path, excludes, enabled, schedule_cadence, selected_paths, created_at
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
// SetFileSetScheduleCadence writes a file set's per-item schedule override (#199).
// An empty string clears it, putting the set back on the Folders domain schedule.
//
// Deliberately its own statement rather than a field on UpdateFileSet: the cadence
// is owned by the schedule editor, and folding it into the general update would
// mean every rename or path edit carries a cadence with it, so a form that did not
// know about the field would silently clear one. The same split targets.go makes
// for SetScheduleCadence, and for the same reason.
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

// SetFileSetSelectedPaths writes a file set's tree selection (Phase 4, D-03)
// as its JSON encoding — the same flat set the container backupPaths column
// stores, but in its own column. A nil slice stores SQL NULL, never the JSON
// literal '[]': the NULL/nil state IS the "never touched by the tree" legacy
// switch the backup compile reads (a stored '[]' would decode to a non-nil
// empty slice and mean something no caller can express). This package does
// not interpret the entries — opaque blob in, opaque blob out; normalization
// and boundary validation live in the API tier.
//
// Deliberately its own statement rather than a field on UpdateFileSet, for
// exactly the reason SetFileSetScheduleCadence is (comment above): a save that
// does not know about the selection must not be able to clear one by omitting
// it. The tree editor is the only writer.
func (r *Repo) SetFileSetSelectedPaths(id string, selected []string) error {
	var encoded any // nil interface binds as SQL NULL for the legacy state
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
	// selected_paths is the table's only nullable column (v101): NULL is the
	// COMMON state (every row created before the tree existed, and every
	// CreateFileSet INSERT omits the column), so it scans into *string —
	// scanning a plain string would fail every legacy row and take down
	// ListFileSets/GetFileSet at runtime. NULL ⇒ the field stays nil; any
	// written value decodes as JSON below. Same nullable-scan precedent as
	// received_repos.last_check_ok (migrate.go v76).
	var selJSON *string
	err := s.Scan(&fs.ID, &fs.Name, &fs.Path, &exJSON, &enabled, &fs.ScheduleCadence, &selJSON, &fs.CreatedAt)
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
