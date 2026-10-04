package store

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// RoleDestination is a destination set up once and offered to every domain.
// Its Repo is the location the domains' repositories sit under, and Domain is
// empty. A domain that ticks it gets a target of its own, derived from it and
// carrying its id in DestinationID, so replication only ever sees ordinary
// targets.
const RoleDestination = "destination"

// ErrNotDestination is returned for an id that names no destination.
var ErrNotDestination = errors.New("no such destination")

// ErrDestinationInUse is returned when a destination still has domain targets.
var ErrDestinationInUse = errors.New("destination is in use")

// destinationMirroredCols are the fields a domain target takes from its
// destination on every save of the destination.
var destinationMirroredCols = []string{"name", "creds_ref", "storage_class", "immutable", "provider"}

func destinationMirroredValues(d OffsiteTarget) []any {
	return []any{d.Name, d.CredsRef, d.StorageClass, boolInt(d.Immutable), d.Provider}
}

// ListDestinations returns every destination in the order they were created.
func (r *Repo) ListDestinations() ([]OffsiteTarget, error) {
	rows, err := r.db.Query(`SELECT `+offsiteTargetCols+` FROM offsite_targets
		WHERE role = ? ORDER BY created_at, id`, RoleDestination)
	if err != nil {
		return nil, fmt.Errorf("ListDestinations: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only rows
	var out []OffsiteTarget
	for rows.Next() {
		t, err := scanOffsiteTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetDestination returns the destination with the given id.
func (r *Repo) GetDestination(id string) (OffsiteTarget, bool, error) {
	t, err := destinationTx(r.db, id)
	if errors.Is(err, ErrNotDestination) {
		return OffsiteTarget{}, false, nil
	}
	if err != nil {
		return OffsiteTarget{}, false, err
	}
	return t, true, nil
}

func destinationTx(q queryer, id string) (OffsiteTarget, error) {
	t, err := scanOffsiteTarget(q.QueryRow(`SELECT `+offsiteTargetCols+`
		FROM offsite_targets WHERE id = ? AND role = ?`, id, RoleDestination))
	if errors.Is(err, sql.ErrNoRows) {
		return OffsiteTarget{}, ErrNotDestination
	}
	return t, err
}

// SaveDestination creates d, or updates the destination with its id and
// carries name, credentials, storage class, append-only flag and provider into
// every domain target derived from it. The location of a destination that has
// domain targets cannot change: their repositories are already there.
func (r *Repo) SaveDestination(d OffsiteTarget) (OffsiteTarget, error) {
	if strings.TrimSpace(d.Repo) == "" {
		return OffsiteTarget{}, ErrEmptyOffsiteRepo
	}
	d.Role, d.Domain, d.Enabled = RoleDestination, "", true
	if d.CreatedAt == 0 {
		d.CreatedAt = time.Now().Unix()
	}
	var saved OffsiteTarget
	err := r.inTx(func(tx *sql.Tx) error {
		if d.ID != "" {
			old, err := destinationTx(tx, d.ID)
			if err != nil {
				return err
			}
			if old.Repo != d.Repo {
				n, err := derivedCountTx(tx, d.ID)
				if err != nil {
					return err
				}
				if n > 0 {
					return fmt.Errorf("%w: its location holds the repositories of %d domain(s)", ErrDestinationInUse, n)
				}
			}
			d.CreatedAt = old.CreatedAt
		} else {
			d.ID = newID()
		}
		_, err := tx.Exec(`
			INSERT INTO offsite_targets (id, domain, name, repo, role, creds_ref, storage_class, immutable,
			  enabled, created_at, sort_order, provider)
			VALUES (?, '', ?, ?, ?, ?, ?, ?, 1, ?, 0, ?)
			ON CONFLICT(id) DO UPDATE SET
			  name = excluded.name, repo = excluded.repo, creds_ref = excluded.creds_ref,
			  storage_class = excluded.storage_class, immutable = excluded.immutable, provider = excluded.provider
			WHERE offsite_targets.role = excluded.role`,
			d.ID, d.Name, d.Repo, RoleDestination, d.CredsRef, d.StorageClass, boolInt(d.Immutable), d.CreatedAt, d.Provider)
		if err != nil {
			return err
		}
		set := make([]string, len(destinationMirroredCols))
		for i, c := range destinationMirroredCols {
			set[i] = c + " = ?"
		}
		args := slices.Concat(destinationMirroredValues(d), []any{RoleOffsite, d.ID})
		//nolint:gosec // G202: set is built from the fixed destinationMirroredCols names; every value travels in args.
		if _, err := tx.Exec(`UPDATE offsite_targets SET `+strings.Join(set, ", ")+`
			WHERE role = ? AND destination_id = ?`, args...); err != nil {
			return err
		}
		saved, err = destinationTx(tx, d.ID)
		return err
	})
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("SaveDestination: %w", err)
	}
	return saved, nil
}

// DestinationTargets returns the domain targets derived from a destination.
func (r *Repo) DestinationTargets(id string) ([]OffsiteTarget, error) {
	rows, err := r.db.Query(`SELECT `+offsiteTargetCols+` FROM offsite_targets
		WHERE role = ? AND destination_id = ? ORDER BY domain`, RoleOffsite, id)
	if err != nil {
		return nil, fmt.Errorf("DestinationTargets: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only rows
	var out []OffsiteTarget
	for rows.Next() {
		t, err := scanOffsiteTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func derivedCountTx(q queryer, id string) (int, error) {
	var n int
	err := q.QueryRow(`SELECT count(*) FROM offsite_targets WHERE role = ? AND destination_id = ?`, RoleOffsite, id).Scan(&n)
	return n, err
}

// EnsureDestinationTarget returns the domain's target derived from the
// destination, creating it at location when there is none. A new target is
// added to the domain default's skip list and to every copy rule of the domain
// that copies at all, in the same transaction, so ticking a destination for
// one item does not start copies for the others. created reports whether the
// target is new.
//
// A new target never takes sort order 0: that slot belongs to the target the
// domain's off-site settings field edits, and syncing that field would switch
// it off.
func (r *Repo) EnsureDestinationTarget(id, domain, location string) (t OffsiteTarget, created bool, err error) {
	if strings.TrimSpace(location) == "" {
		return OffsiteTarget{}, false, ErrEmptyOffsiteRepo
	}
	err = r.inTx(func(tx *sql.Tx) error {
		d, err := destinationTx(tx, id)
		if err != nil {
			return err
		}
		existing, err := scanOffsiteTarget(tx.QueryRow(`SELECT `+offsiteTargetCols+` FROM offsite_targets
			WHERE role = ? AND destination_id = ? AND domain = ?`, RoleOffsite, id, domain))
		if err == nil {
			t = existing
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		now := time.Now().Unix()
		tid := newID()
		_, err = tx.Exec(`
			INSERT INTO offsite_targets (id, domain, name, repo, role, creds_ref, storage_class, immutable,
			  enabled, created_at, sort_order, provider, destination_id)
			SELECT ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, MAX(COALESCE(MAX(sort_order), 0) + 1, 1), ?, ?
			  FROM offsite_targets WHERE role = ? AND domain = ?`,
			tid, domain, d.Name, location, RoleOffsite, d.CredsRef, d.StorageClass, boolInt(d.Immutable),
			now, d.Provider, id, RoleOffsite, domain)
		if err != nil {
			return err
		}
		if slices.Contains(PlacementDomains, domain) {
			if err := skipNewTargetTx(tx, domain, tid, now); err != nil {
				return err
			}
		}
		t, err = offsiteTargetTx(tx, tid)
		created = err == nil
		return err
	})
	if err != nil {
		return OffsiteTarget{}, false, fmt.Errorf("EnsureDestinationTarget: %w", err)
	}
	return t, created, nil
}

// skipNewTargetTx adds a target to the domain default's skip list and to every
// copy rule of the domain that does not already skip everything.
func skipNewTargetTx(tx *sql.Tx, domain, targetID string, now int64) error {
	def, ok, err := placementDefaultQ(tx, domain)
	if err != nil {
		return err
	}
	if ok && !slices.Contains(def.Skip, SkipAll) {
		raw, err := encodeSkip(append(slices.Clone(def.Skip), targetID))
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE placement_defaults SET skip = ?, updated_at = ? WHERE domain = ?`, raw, now, domain); err != nil {
			return err
		}
	} else if !ok {
		raw, err := encodeSkip([]string{targetID})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO placement_defaults (domain, home, skip, confirmed_at, updated_at) VALUES (?, '', ?, ?, ?)`,
			domain, raw, now, now); err != nil {
			return err
		}
	}
	rules, err := copyRulesQ(tx, domain)
	if err != nil {
		return err
	}
	for identity, rule := range rules {
		if slices.Contains(rule.Skip, SkipAll) || slices.Contains(rule.Skip, targetID) {
			continue
		}
		if err := setCopyRuleTx(tx, domain, identity, append(slices.Clone(rule.Skip), targetID), now); err != nil {
			return err
		}
	}
	return nil
}

// DeleteDestinationIfUnused removes a destination that no domain target is
// derived from any more.
func (r *Repo) DeleteDestinationIfUnused(id string) error {
	err := r.inTx(func(tx *sql.Tx) error {
		if _, err := destinationTx(tx, id); err != nil {
			return err
		}
		n, err := derivedCountTx(tx, id)
		if err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("%w: %d domain(s) still copy to it", ErrDestinationInUse, n)
		}
		_, err = tx.Exec(`DELETE FROM offsite_targets WHERE id = ? AND role = ?`, id, RoleDestination)
		return err
	})
	if err != nil {
		return fmt.Errorf("DeleteDestinationIfUnused: %w", err)
	}
	return nil
}
