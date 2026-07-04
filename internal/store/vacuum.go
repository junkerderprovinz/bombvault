package store

import "fmt"

// VacuumInto writes a fully-consistent single-file snapshot of the live database
// to dst using SQLite's `VACUUM INTO`. Because the DB runs in WAL mode with a
// single pooled connection, this is the safe way to snapshot it: it folds the WAL
// in and cannot capture a torn/partial write. dst must not already exist (SQLite
// refuses to overwrite), so callers stage into a freshly-created directory.
func (r *Repo) VacuumInto(dst string) error {
	if _, err := r.db.Exec("VACUUM INTO ?", dst); err != nil {
		return fmt.Errorf("VacuumInto %q: %w", dst, err)
	}
	return nil
}
