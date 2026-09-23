package store

import "fmt"

// VacuumInto writes a consistent single-file snapshot of the live database to
// dst with VACUUM INTO. Unlike copying the files of a WAL-mode database, it
// includes the WAL and cannot catch a half-written page. SQLite refuses to
// overwrite, so dst must not exist yet.
func (r *Repo) VacuumInto(dst string) error {
	if _, err := r.db.Exec("VACUUM INTO ?", dst); err != nil {
		return fmt.Errorf("VacuumInto %q: %w", dst, err)
	}
	return nil
}
