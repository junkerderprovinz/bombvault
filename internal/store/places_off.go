package store

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"
)

// LocalSwitchDomains are the sections without placement whose own folder can
// be switched off, so that their backups go straight to their off-site
// targets.
var LocalSwitchDomains = []string{"flash", "config", "zfs"}

// The places a section can be switched off at by hand.
const (
	placeLocal   = "local"
	placePrimary = "primary"
	// The prefix of the places a replica-only switch turned off, one row each.
	placeReplicaOnly = "replica-only:"
)

// LocalOff returns the sections whose own folder takes no new backups.
func (r *Repo) LocalOff() (map[string]bool, error) {
	rows, err := r.db.Query(`SELECT domain FROM places_off WHERE place = ?`, placeLocal)
	if err != nil {
		return nil, fmt.Errorf("LocalOff: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := map[string]bool{}
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			return nil, fmt.Errorf("LocalOff: %w", err)
		}
		out[domain] = true
	}
	return out, rows.Err()
}

// SetLocal switches a section's own folder on or off for new backups. The
// backups already in it stay where they are.
func (r *Repo) SetLocal(domain string, on bool) error {
	if !slices.Contains(LocalSwitchDomains, domain) {
		return fmt.Errorf("SetLocal: %q has no switch for its own folder", domain)
	}
	if err := r.setPlaceOff(domain, placeLocal, "", !on); err != nil {
		return fmt.Errorf("SetLocal %s: %w", domain, err)
	}
	return nil
}

// PrimaryOffAt returns the location at which the target a domain's off-site
// field describes was switched off by hand, "" when it was not.
func (r *Repo) PrimaryOffAt(domain string) (string, error) {
	var repo string
	err := r.db.QueryRow(`SELECT repo FROM places_off WHERE domain = ? AND place = ?`, domain, placePrimary).Scan(&repo)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("PrimaryOffAt %s: %w", domain, err)
	}
	return repo, nil
}

// SetPrimaryOff remembers that the target a domain's off-site field describes
// was switched off by hand while it pointed at repo, or forgets it. A field
// that names another location brings the target back on.
func (r *Repo) SetPrimaryOff(domain, repo string, off bool) error {
	if err := r.setPlaceOff(domain, placePrimary, repo, off); err != nil {
		return fmt.Errorf("SetPrimaryOff %s: %w", domain, err)
	}
	return nil
}

// ReplicaOnlyOff returns the places of a section that its replica-only switch
// turned off, so that switching it off again brings back those and no others:
// "" for the own folder, a target's id for a target.
func (r *Repo) ReplicaOnlyOff(domain string) ([]string, error) {
	rows, err := r.db.Query(`SELECT repo FROM places_off WHERE domain = ? AND place LIKE ? ORDER BY place`, domain, placeReplicaOnly+"%")
	if err != nil {
		return nil, fmt.Errorf("ReplicaOnlyOff %s: %w", domain, err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("ReplicaOnlyOff %s: %w", domain, err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SetReplicaOnlyOff remembers that a section's replica-only switch turned off
// the place id ("" for the own folder), or forgets it.
func (r *Repo) SetReplicaOnlyOff(domain, id string, off bool) error {
	if err := r.setPlaceOff(domain, placeReplicaOnly+id, id, off); err != nil {
		return fmt.Errorf("SetReplicaOnlyOff %s: %w", domain, err)
	}
	return nil
}

func (r *Repo) setPlaceOff(domain, place, repo string, off bool) error {
	if !off {
		_, err := r.db.Exec(`DELETE FROM places_off WHERE domain = ? AND place = ?`, domain, place)
		return err
	}
	_, err := r.db.Exec(`INSERT INTO places_off (domain, place, repo, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (domain, place) DO UPDATE SET repo = excluded.repo, updated_at = excluded.updated_at`,
		domain, place, repo, time.Now().Unix())
	return err
}
