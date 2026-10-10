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

// destinationMirroredCols are the fields every domain target takes from its
// destination on each save of the destination. The settings FollowedSettings
// lists are carried too, into the targets that do not keep their own.
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
// every domain target derived from it, and its keep-policy, compression,
// limits and switch into those that do not keep their own. A new destination
// starts switched on and off the premises. The location of a destination that
// has domain targets cannot change: their repositories are already there.
func (r *Repo) SaveDestination(d OffsiteTarget) (OffsiteTarget, error) {
	if strings.TrimSpace(d.Repo) == "" {
		return OffsiteTarget{}, ErrEmptyOffsiteRepo
	}
	d.Role, d.Domain = RoleDestination, ""
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
			d.Enabled, d.OffPremises = true, true
		}
		if err := writeDestinationTx(tx, d); err != nil {
			return err
		}
		out, err := destinationTx(tx, d.ID)
		saved = out
		return err
	})
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("SaveDestination: %w", err)
	}
	return saved, nil
}

// writeDestinationTx inserts or updates d and mirrors it into its domain
// targets. A row of another role under the same id is left alone.
func writeDestinationTx(tx *sql.Tx, d OffsiteTarget) error {
	//nolint:gosec // G202: the column lists are built from the fixed followedCols names; every value travels in args.
	_, err := tx.Exec(`
		INSERT INTO offsite_targets (id, domain, name, repo, role, creds_ref, storage_class, immutable,
		  created_at, sort_order, provider, off_premises, `+strings.Join(followedCols, ", ")+`)
		VALUES (?, '', ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		  name = excluded.name, repo = excluded.repo, creds_ref = excluded.creds_ref,
		  storage_class = excluded.storage_class, immutable = excluded.immutable, provider = excluded.provider,
		  off_premises = excluded.off_premises, `+assignments(followedCols, "excluded.")+`
		WHERE offsite_targets.role = excluded.role`,
		slices.Concat([]any{d.ID, d.Name, d.Repo, RoleDestination, d.CredsRef, d.StorageClass, boolInt(d.Immutable),
			d.CreatedAt, d.Provider, boolInt(d.OffPremises)}, followedValues(d))...)
	if err != nil {
		return err
	}
	set := make([]string, len(destinationMirroredCols))
	for i, c := range destinationMirroredCols {
		set[i] = c + " = ?"
	}
	args := slices.Concat(destinationMirroredValues(d), []any{RoleOffsite, d.ID})
	//nolint:gosec // G202: set is built from the fixed destinationMirroredCols names; every value travels in args.
	if _, err = tx.Exec(`UPDATE offsite_targets SET `+strings.Join(set, ", ")+`
		WHERE role = ? AND destination_id = ?`, args...); err != nil {
		return err
	}
	derived, err := destinationTargetsQ(tx, d.ID)
	if err != nil {
		return err
	}
	for _, t := range derived {
		// A value that only spells the destination's differently stays as written.
		differs := t.differingFrom(d)
		for _, setting := range FollowedSettings {
			if differs&setting != 0 && !t.Keeps(setting) {
				t.TakeFrom(d, setting)
			}
		}
		//nolint:gosec // G202: the SET list is built from the fixed followedCols names; every value travels in args.
		if _, err := tx.Exec(`UPDATE offsite_targets SET `+assignments(followedCols, "?")+` WHERE id = ? AND role = ?`,
			slices.Concat(followedValues(t), []any{t.ID, RoleOffsite})...); err != nil {
			return err
		}
		if err := mirrorTx(tx, t, false); err != nil {
			return err
		}
	}
	return nil
}

// followedCols are the columns behind FollowedSettings, in the order
// followedValues returns them.
var followedCols = []string{
	"retention_keep_last", "retention_keep_daily", "retention_keep_weekly", "retention_keep_monthly",
	"retention_keep_yearly", "compression", "limit_upload", "limit_download", "enabled",
}

func followedValues(t OffsiteTarget) []any {
	return []any{
		t.RetentionKeepLast, t.RetentionKeepDaily, t.RetentionKeepWeekly, t.RetentionKeepMonthly,
		t.RetentionKeepYearly, t.Compression, t.LimitUpload, t.LimitDownload, boolInt(t.Enabled),
	}
}

// assignments writes "col = value" for each column. A value that ends in a
// dot is a table prefix and takes the column's name behind it.
func assignments(cols []string, value string) string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c + " = " + value
		if strings.HasSuffix(value, ".") {
			out[i] += c
		}
	}
	return strings.Join(out, ", ")
}

// ImportDestinations writes the destinations of a settings file, each under
// its own id. A destination whose location here already holds domain
// repositories keeps that location, and its name is returned in kept.
func (r *Repo) ImportDestinations(ds []OffsiteTarget) (kept []string, err error) {
	err = r.inTx(func(tx *sql.Tx) error {
		for _, d := range ds {
			if strings.TrimSpace(d.Repo) == "" {
				return ErrEmptyOffsiteRepo
			}
			if d.ID == "" {
				d.ID = newID()
			}
			d.Role, d.Domain = RoleDestination, ""
			old, err := destinationTx(tx, d.ID)
			switch {
			case err == nil:
				d.CreatedAt = old.CreatedAt
				if old.Repo != d.Repo {
					n, err := derivedCountTx(tx, d.ID)
					if err != nil {
						return err
					}
					if n > 0 {
						d.Repo = old.Repo
						kept = append(kept, d.Name)
					}
				}
			case !errors.Is(err, ErrNotDestination):
				return err
			case d.CreatedAt == 0:
				d.CreatedAt = time.Now().Unix()
			}
			if err := writeDestinationTx(tx, d); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ImportDestinations: %w", err)
	}
	return kept, nil
}

// DeleteUnusedDestinationsExcept removes every destination outside keep that
// no domain target is derived from.
func (r *Repo) DeleteUnusedDestinationsExcept(keep []string) error {
	ds, err := r.ListDestinations()
	if err != nil {
		return err
	}
	for _, d := range ds {
		if slices.Contains(keep, d.ID) {
			continue
		}
		if _, err := r.db.Exec(`DELETE FROM offsite_targets WHERE id = ? AND role = ?
			AND NOT EXISTS (SELECT 1 FROM offsite_targets t WHERE t.role = ? AND t.destination_id = ?)`,
			d.ID, RoleDestination, RoleOffsite, d.ID); err != nil {
			return fmt.Errorf("DeleteUnusedDestinationsExcept: %w", err)
		}
	}
	return nil
}

// DestinationTargets returns the domain targets derived from a destination.
func (r *Repo) DestinationTargets(id string) ([]OffsiteTarget, error) {
	return destinationTargetsQ(r.db, id)
}

func destinationTargetsQ(q queryer, id string) ([]OffsiteTarget, error) {
	rows, err := q.Query(`SELECT `+offsiteTargetCols+` FROM offsite_targets
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
// destination, creating it at location when there is none. A new target
// starts with the destination's keep-policy, compression, limits and switch
// and follows it for all of them. It is added to the domain default's skip list and to every copy rule of the domain
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
		//nolint:gosec // G202: the column list is built from the fixed followedCols names; every value travels in args.
		_, err = tx.Exec(`
			INSERT INTO offsite_targets (id, domain, name, repo, role, creds_ref, storage_class, immutable,
			  created_at, sort_order, provider, destination_id, `+strings.Join(followedCols, ", ")+`)
			SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, MAX(COALESCE(MAX(sort_order), 0) + 1, 1), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
			  FROM offsite_targets WHERE role = ? AND domain = ?`,
			slices.Concat([]any{tid, domain, d.Name, location, RoleOffsite, d.CredsRef, d.StorageClass, boolInt(d.Immutable),
				now, d.Provider, id}, followedValues(d), []any{RoleOffsite, domain})...)
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

var (
	// ErrTargetFollowsDestination is returned for a target that already
	// follows a destination.
	ErrTargetFollowsDestination = errors.New("the target already follows a destination")
	// ErrDomainHasDestinationTarget is returned when the domain already has a
	// target from the destination.
	ErrDomainHasDestinationTarget = errors.New("the domain already has a target from this destination")
)

// AdoptIntoDestination hangs a domain target typed in by hand on a
// destination. The target takes the destination's mirrored fields and keeps
// its repository and copy rules, and its own value for every followed setting
// the destination holds another one for. For the target in the primary
// slot, settle brings the domain's off-site settings in line in the same
// transaction, because they rewrite the primary slot on every save.
func (r *Repo) AdoptIntoDestination(targetID, destID string, settle func(*Settings)) (OffsiteTarget, error) {
	r.settingsMu.Lock()
	defer r.settingsMu.Unlock()
	var out OffsiteTarget
	err := r.inTx(func(tx *sql.Tx) error {
		d, err := destinationTx(tx, destID)
		if err != nil {
			return err
		}
		t, err := offsiteTargetTx(tx, targetID)
		if err != nil {
			return err
		}
		if t.DestinationID != "" {
			return ErrTargetFollowsDestination
		}
		var n int
		if err := tx.QueryRow(`SELECT count(*) FROM offsite_targets WHERE role = ? AND destination_id = ? AND domain = ?`,
			RoleOffsite, destID, t.Domain).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrDomainHasDestinationTarget
		}
		if out, err = followDestinationTx(tx, t, d); err != nil {
			return err
		}
		if t.SortOrder == 0 && settle != nil {
			return settleTx(tx, settle)
		}
		return nil
	})
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("AdoptIntoDestination: %w", err)
	}
	return out, nil
}

// PrimaryFromDestination makes location, the destination's folder for the
// domain, the domain's primary off-site target. The row in the primary slot
// moves there and follows the destination; a domain without one gets a new
// row. settle writes the domain's off-site field and append-only flag in the
// same transaction, because a settings save rewrites the primary slot from
// them.
func (r *Repo) PrimaryFromDestination(destID, domain, location string, settle func(*Settings)) (OffsiteTarget, error) {
	if strings.TrimSpace(location) == "" {
		return OffsiteTarget{}, ErrEmptyOffsiteRepo
	}
	r.settingsMu.Lock()
	defer r.settingsMu.Unlock()
	var out OffsiteTarget
	err := r.inTx(func(tx *sql.Tx) error {
		d, err := destinationTx(tx, destID)
		if err != nil {
			return err
		}
		t, err := scanOffsiteTarget(tx.QueryRow(`SELECT `+offsiteTargetCols+` FROM offsite_targets
			WHERE domain = ? AND role = ? AND sort_order = 0 ORDER BY created_at, id LIMIT 1`, domain, RoleOffsite))
		switch {
		case errors.Is(err, sql.ErrNoRows):
			t = OffsiteTarget{ID: newID(), Domain: domain, Role: RoleOffsite, CreatedAt: time.Now().Unix()}
			if _, err := tx.Exec(`INSERT INTO offsite_targets (id, domain, name, repo, role, enabled, created_at, sort_order)
				VALUES (?, ?, ?, ?, ?, 1, ?, 0)`, t.ID, domain, d.Name, location, RoleOffsite, t.CreatedAt); err != nil {
				return err
			}
		case err != nil:
			return err
		case t.DestinationID == destID:
		case t.DestinationID != "":
			if _, err := tx.Exec(`UPDATE offsite_targets SET destination_id = '' WHERE id = ? AND role = ?`, t.ID, RoleOffsite); err != nil {
				return err
			}
			t.DestinationID = ""
		}
		if t.DestinationID == "" {
			var n int
			if err := tx.QueryRow(`SELECT count(*) FROM offsite_targets WHERE role = ? AND destination_id = ? AND domain = ?`,
				RoleOffsite, destID, domain).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return ErrDomainHasDestinationTarget
			}
		}
		if _, err := tx.Exec(`UPDATE offsite_targets SET repo = ?, enabled = 1 WHERE id = ? AND role = ?`,
			location, t.ID, RoleOffsite); err != nil {
			return err
		}
		t.DestinationID, t.Enabled = "", true
		if out, err = followDestinationTx(tx, t, d); err != nil {
			return err
		}
		return settleTx(tx, settle)
	})
	if err != nil {
		return OffsiteTarget{}, fmt.Errorf("PrimaryFromDestination: %w", err)
	}
	return out, nil
}

// followDestinationTx hangs t on d: it takes d's mirrored fields, and so does
// its direct repository.
func followDestinationTx(tx *sql.Tx, t, d OffsiteTarget) (OffsiteTarget, error) {
	if t.DestinationID != "" {
		return OffsiteTarget{}, ErrTargetFollowsDestination
	}
	t.DestinationID, t.Own = d.ID, t.differingFrom(d)
	t.Name, t.CredsRef, t.StorageClass, t.Immutable, t.Provider = d.Name, d.CredsRef, d.StorageClass, d.Immutable, d.Provider
	if _, err := tx.Exec(`UPDATE offsite_targets SET destination_id = ?, name = ?, creds_ref = ?, storage_class = ?,
		  immutable = ?, provider = ?, own_settings = ?
		WHERE id = ? AND role = ?`,
		t.DestinationID, t.Name, t.CredsRef, t.StorageClass, boolInt(t.Immutable), t.Provider, t.Own,
		t.ID, RoleOffsite); err != nil {
		return OffsiteTarget{}, err
	}
	if err := mirrorTx(tx, t, false); err != nil {
		return OffsiteTarget{}, err
	}
	return offsiteTargetTx(tx, t.ID)
}

func settleTx(tx *sql.Tx, settle func(*Settings)) error {
	s, err := getSettings(tx)
	if err != nil {
		return err
	}
	settle(&s)
	return updateSettings(tx, s)
}

// DetachFromDestination unhooks a target from the destination it follows.
// The fields it took from the destination stay until its next save.
func (r *Repo) DetachFromDestination(id string) error {
	if _, err := r.db.Exec(`UPDATE offsite_targets SET destination_id = '', own_settings = 0 WHERE id = ? AND role = ?`,
		id, RoleOffsite); err != nil {
		return fmt.Errorf("DetachFromDestination: %w", err)
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
