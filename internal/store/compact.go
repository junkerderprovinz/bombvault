package store

import (
	"errors"
	"fmt"
)

// Compact rebuilds the database from its live rows and empties the
// write-ahead log, so the bytes of rows that were rewritten or deleted are
// gone from the files and not only from the tables. It rewrites the whole
// file, so it suits a one-time step such as a migration.
func (r *Repo) Compact() error {
	if _, err := r.db.Exec("VACUUM"); err != nil {
		return fmt.Errorf("Compact: %w", err)
	}
	var busy, logFrames, moved int
	if err := r.db.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &moved); err != nil {
		return fmt.Errorf("Compact checkpoint: %w", err)
	}
	if busy != 0 {
		return errors.New("Compact checkpoint: the log is still in use")
	}
	return nil
}
