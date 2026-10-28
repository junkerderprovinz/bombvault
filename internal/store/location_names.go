package store

import (
	"fmt"
	"strings"
)

// LocationNames returns the names given to storage locations that have no row
// of their own, by location key.
func (r *Repo) LocationNames() (map[string]string, error) {
	rows, err := r.db.Query(`SELECT location, name FROM location_names`)
	if err != nil {
		return nil, fmt.Errorf("LocationNames: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only
	out := map[string]string{}
	for rows.Next() {
		var key, name string
		if err := rows.Scan(&key, &name); err != nil {
			return nil, fmt.Errorf("LocationNames: %w", err)
		}
		out[key] = name
	}
	return out, rows.Err()
}

// SetLocationName names the storage location with the given key. An empty
// name gives it back the one it is listed under by default.
func (r *Repo) SetLocationName(key, name string) error {
	name = strings.TrimSpace(name)
	var err error
	if name == "" {
		_, err = r.db.Exec(`DELETE FROM location_names WHERE location = ?`, key)
	} else {
		_, err = r.db.Exec(`INSERT INTO location_names (location, name) VALUES (?, ?)
			ON CONFLICT(location) DO UPDATE SET name = excluded.name`, key, name)
	}
	if err != nil {
		return fmt.Errorf("SetLocationName: %w", err)
	}
	return nil
}
