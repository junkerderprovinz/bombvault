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

// LocationKeeps returns the How long to keep cards saved for storage locations
// that have no row to carry one, keyed like LocationNames.
func (r *Repo) LocationKeeps() (map[string]RetentionKeep, error) {
	rows, err := r.db.Query(`SELECT location, keep_last, keep_daily, keep_weekly, keep_monthly, keep_yearly FROM location_keep`)
	if err != nil {
		return nil, fmt.Errorf("LocationKeeps: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := map[string]RetentionKeep{}
	for rows.Next() {
		var key string
		var k RetentionKeep
		if err := rows.Scan(&key, &k.KeepLast, &k.KeepDaily, &k.KeepWeekly, &k.KeepMonthly, &k.KeepYearly); err != nil {
			return nil, fmt.Errorf("LocationKeeps: %w", err)
		}
		out[key] = k
	}
	return out, rows.Err()
}

// SetLocationKeep saves the card of the storage location with the given key.
func (r *Repo) SetLocationKeep(key string, k RetentionKeep) error {
	_, err := r.db.Exec(`INSERT INTO location_keep (location, keep_last, keep_daily, keep_weekly, keep_monthly, keep_yearly) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(location) DO UPDATE SET keep_last = excluded.keep_last, keep_daily = excluded.keep_daily,
			keep_weekly = excluded.keep_weekly, keep_monthly = excluded.keep_monthly, keep_yearly = excluded.keep_yearly`,
		key, k.KeepLast, k.KeepDaily, k.KeepWeekly, k.KeepMonthly, k.KeepYearly)
	if err != nil {
		return fmt.Errorf("SetLocationKeep: %w", err)
	}
	return nil
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
