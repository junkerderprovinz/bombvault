package store

import "fmt"

// RestoreRemoval is a container a restore or an update removed and has not
// created again.
type RestoreRemoval struct {
	Name string
	// Definition is the JSON of the model.Inspect the container is created
	// from, OldID the ID the containers sharing its namespaces are bound to.
	Definition string
	OldID      string
	At         int64
	// Update marks a removal by an update. Its data is untouched, so the
	// container starts again, where one a restore removed stays stopped.
	Update bool
}

// MarkRestoreRemoval remembers a container about to be removed.
func (r *Repo) MarkRestoreRemoval(m RestoreRemoval) error {
	if _, err := r.db.Exec(`INSERT INTO container_restore_removals (name, definition, old_id, at, by_update) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (name) DO UPDATE SET definition = excluded.definition, old_id = excluded.old_id, at = excluded.at, by_update = excluded.by_update`,
		m.Name, m.Definition, m.OldID, m.At, m.Update); err != nil {
		return fmt.Errorf("MarkRestoreRemoval: %w", err)
	}
	return nil
}

// ClearRestoreRemoval forgets a container that exists again.
func (r *Repo) ClearRestoreRemoval(name string) error {
	if _, err := r.db.Exec(`DELETE FROM container_restore_removals WHERE name = ?`, name); err != nil {
		return fmt.Errorf("ClearRestoreRemoval: %w", err)
	}
	return nil
}

// ListRestoreRemovals returns the containers restores removed, oldest first.
func (r *Repo) ListRestoreRemovals() ([]RestoreRemoval, error) {
	rows, err := r.db.Query(`SELECT name, definition, old_id, at, by_update FROM container_restore_removals ORDER BY at, name`)
	if err != nil {
		return nil, fmt.Errorf("ListRestoreRemovals: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []RestoreRemoval
	for rows.Next() {
		var m RestoreRemoval
		if err := rows.Scan(&m.Name, &m.Definition, &m.OldID, &m.At, &m.Update); err != nil {
			return nil, fmt.Errorf("ListRestoreRemovals: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
