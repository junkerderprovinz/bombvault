package store

import (
	"database/sql"
	"errors"
	"fmt"
)

var ErrNoTargetID = errors.New("observations need a target id")

// ItemCopies is what one target held of one item at its last listing.
type ItemCopies struct {
	Domain           string
	Identity         string
	TargetID         string
	SnapshotCount    int
	LatestSnapshotAt int64
	ObservedAt       int64
}

// TargetObservation is when a target was last listed for a domain, and under
// which state of the rules it was last aged.
type TargetObservation struct {
	Domain   string
	TargetID string
	ListedAt int64
	RulesRev string
	AgedAt   int64
}

// RecordTargetListing replaces what a target holds of the domain with rows and
// stamps the listing. A name missing from rows loses its row; a row without
// snapshots is not stored.
func (r *Repo) RecordTargetListing(domain, targetID string, listedAt int64, rows []ItemCopies) error {
	if targetID == "" {
		return ErrNoTargetID
	}
	seen := make(map[string]bool, len(rows))
	for _, c := range rows {
		if seen[c.Identity] {
			return fmt.Errorf("RecordTargetListing: %s listed twice", c.Identity)
		}
		seen[c.Identity] = true
	}
	err := r.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM offsite_item_copies WHERE domain = ? AND target_id = ?`, domain, targetID); err != nil {
			return err
		}
		for _, c := range rows {
			if c.SnapshotCount == 0 {
				continue
			}
			if _, err := tx.Exec(`INSERT INTO offsite_item_copies
				(domain, identity, target_id, snapshot_count, latest_snapshot_at, observed_at) VALUES (?, ?, ?, ?, ?, ?)`,
				domain, c.Identity, targetID, c.SnapshotCount, c.LatestSnapshotAt, listedAt); err != nil {
				return err
			}
		}
		_, err := tx.Exec(`INSERT INTO offsite_observations (domain, target_id, listed_at) VALUES (?, ?, ?)
			ON CONFLICT(domain, target_id) DO UPDATE SET listed_at = excluded.listed_at`, domain, targetID, listedAt)
		return err
	})
	if err != nil {
		return fmt.Errorf("RecordTargetListing: %w", err)
	}
	return nil
}

// AdjustItemCopies applies what a delete or a keep-policy left at a target: each
// row replaces the one of its name, a row without snapshots removes it. The
// listing time stays.
func (r *Repo) AdjustItemCopies(domain, targetID string, at int64, rows []ItemCopies) error {
	if targetID == "" {
		return ErrNoTargetID
	}
	err := r.inTx(func(tx *sql.Tx) error {
		for _, c := range rows {
			var err error
			if c.SnapshotCount == 0 {
				_, err = tx.Exec(`DELETE FROM offsite_item_copies WHERE domain = ? AND target_id = ? AND identity = ?`,
					domain, targetID, c.Identity)
			} else {
				_, err = tx.Exec(`INSERT INTO offsite_item_copies
					(domain, identity, target_id, snapshot_count, latest_snapshot_at, observed_at) VALUES (?, ?, ?, ?, ?, ?)
					ON CONFLICT(domain, target_id, identity) DO UPDATE SET snapshot_count = excluded.snapshot_count,
					  latest_snapshot_at = excluded.latest_snapshot_at, observed_at = excluded.observed_at`,
					domain, c.Identity, targetID, c.SnapshotCount, c.LatestSnapshotAt, at)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("AdjustItemCopies: %w", err)
	}
	return nil
}

// ItemCopiesFor returns what each target held of one item, by target.
func (r *Repo) ItemCopiesFor(domain, identity string) ([]ItemCopies, error) {
	rows, err := r.db.Query(`SELECT domain, identity, target_id, snapshot_count, latest_snapshot_at, observed_at
		FROM offsite_item_copies WHERE domain = ? AND identity = ? ORDER BY target_id, identity`, domain, identity)
	if err != nil {
		return nil, fmt.Errorf("ItemCopiesFor: %w", err)
	}
	return scanItemCopies(rows)
}

// ItemCopiesForDomain returns what every target held of the domain, by target
// and name.
func (r *Repo) ItemCopiesForDomain(domain string) ([]ItemCopies, error) {
	rows, err := r.db.Query(`SELECT domain, identity, target_id, snapshot_count, latest_snapshot_at, observed_at
		FROM offsite_item_copies WHERE domain = ? ORDER BY target_id, identity`, domain)
	if err != nil {
		return nil, fmt.Errorf("ItemCopiesForDomain: %w", err)
	}
	return scanItemCopies(rows)
}

// TargetObservationFor returns the observation of one target for a domain; none
// means the target was never listed for it.
func (r *Repo) TargetObservationFor(domain, targetID string) (TargetObservation, bool, error) {
	o := TargetObservation{Domain: domain, TargetID: targetID}
	err := r.db.QueryRow(`SELECT listed_at, rules_rev, aged_at FROM offsite_observations WHERE domain = ? AND target_id = ?`,
		domain, targetID).Scan(&o.ListedAt, &o.RulesRev, &o.AgedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return TargetObservation{}, false, nil
	}
	if err != nil {
		return TargetObservation{}, false, fmt.Errorf("TargetObservationFor: %w", err)
	}
	return o, true, nil
}

// TargetObservationsForDomain returns the observations of a domain by target id.
func (r *Repo) TargetObservationsForDomain(domain string) (map[string]TargetObservation, error) {
	rows, err := r.db.Query(`SELECT target_id, listed_at, rules_rev, aged_at FROM offsite_observations WHERE domain = ?`, domain)
	if err != nil {
		return nil, fmt.Errorf("TargetObservationsForDomain: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := map[string]TargetObservation{}
	for rows.Next() {
		o := TargetObservation{Domain: domain}
		if err := rows.Scan(&o.TargetID, &o.ListedAt, &o.RulesRev, &o.AgedAt); err != nil {
			return nil, fmt.Errorf("TargetObservationsForDomain: %w", err)
		}
		out[o.TargetID] = o
	}
	return out, rows.Err()
}

// MarkTargetAged notes that the target was aged under the rules fingerprinted by
// rulesRev. The target must have been listed for the domain.
func (r *Repo) MarkTargetAged(domain, targetID, rulesRev string, at int64) error {
	res, err := r.db.Exec(`UPDATE offsite_observations SET rules_rev = ?, aged_at = ? WHERE domain = ? AND target_id = ?`,
		rulesRev, at, domain, targetID)
	if err != nil {
		return fmt.Errorf("MarkTargetAged: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("MarkTargetAged: %s was never listed for %s", targetID, domain)
	}
	return nil
}

// ResetTargetAged forgets the aging mark, so the next pass ages the target again.
func (r *Repo) ResetTargetAged(domain, targetID string) error {
	if _, err := r.db.Exec(`UPDATE offsite_observations SET rules_rev = '', aged_at = 0 WHERE domain = ? AND target_id = ?`,
		domain, targetID); err != nil {
		return fmt.Errorf("ResetTargetAged: %w", err)
	}
	return nil
}

func deleteTargetObservationsTx(tx *sql.Tx, targetID string) error {
	if _, err := tx.Exec(`DELETE FROM offsite_item_copies WHERE target_id = ?`, targetID); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM offsite_observations WHERE target_id = ?`, targetID)
	return err
}

func scanItemCopies(rows *sql.Rows) ([]ItemCopies, error) {
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := []ItemCopies{}
	for rows.Next() {
		var c ItemCopies
		if err := rows.Scan(&c.Domain, &c.Identity, &c.TargetID, &c.SnapshotCount, &c.LatestSnapshotAt, &c.ObservedAt); err != nil {
			return nil, fmt.Errorf("scan item copies: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
