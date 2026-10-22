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

// The places a section can be switched off at by hand. placeReplicaOnly is
// all of the ZFS section's places at once, while its items only replicate;
// the places keep their own switches under it.
const (
	placeLocal       = "local"
	placePrimary     = "primary"
	placeReplicaOnly = "replica-only"
)

// ZFSReplicaOnly reports whether the ZFS section takes no backups, so that a
// run of an item with a replica only sends the replica.
func (r *Repo) ZFSReplicaOnly() (bool, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM places_off WHERE domain = 'zfs' AND place = ?`, placeReplicaOnly).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("ZFSReplicaOnly: %w", err)
	}
	return n > 0, nil
}

// SetZFSReplicaOnly switches the ZFS section's backups off or on again,
// leaving the switches of its own folder and its off-site targets alone.
func (r *Repo) SetZFSReplicaOnly(on bool) error {
	if err := r.setPlaceOff("zfs", placeReplicaOnly, "", on); err != nil {
		return fmt.Errorf("SetZFSReplicaOnly: %w", err)
	}
	return nil
}

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
