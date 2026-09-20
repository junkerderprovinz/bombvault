package store

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"
)

// PlacementDomains are the domains whose items choose where they are copied.
var PlacementDomains = []string{"containers", "vms", "files"}

// ItemRef names one container, VM or file set.
type ItemRef struct {
	Domain string // "containers", "vms" or "files"
	Key    string // container name, VM name or file set id
}

// PlacementDefault is a domain's default: its home applies to an open item at its
// first backup, its skip to every item without its own copy rule.
type PlacementDefault struct {
	Domain      string
	Home        string // named repository id, "" for the domain path
	Skip        []string
	ConfirmedAt int64 // 0 while replication waits for the default to be confirmed
	// ConfirmedManually is set only by ConfirmPlacement, never by
	// PutPlacementDefault: saving a default answers where an item's next
	// backup goes, not whether an operator has looked at a rebuilt box.
	ConfirmedManually bool
	UpdatedAt         int64
}

// Paused reports whether replication waits for this default to be confirmed.
func (d PlacementDefault) Paused() bool { return d.ConfirmedAt == 0 }

// PlacementState is what a run reads once, before its first restic call.
type PlacementState struct {
	Domain     string
	Default    PlacementDefault
	HasDefault bool
	Rules      map[string]CopyRule // by identity
}

// Paused reports whether the domain's replication waits for a confirmation. A
// domain without a default is not paused.
func (s PlacementState) Paused() bool { return s.HasDefault && s.Default.Paused() }

// Confirmed reports whether an operator has confirmed the domain's default
// through ConfirmPlacement, the state that retires the rebuild-detection
// pause checks for good. Saving a default through PutPlacementDefault never
// sets this, so it grants no immunity from them.
func (s PlacementState) Confirmed() bool { return s.HasDefault && s.Default.ConfirmedManually }

// HasRules reports whether anything may keep an item from a target: a rule of
// its own, or a default that leaves a target out.
func (s PlacementState) HasRules() bool {
	return len(s.Rules) > 0 || (s.HasDefault && len(s.Default.Skip) > 0)
}

// backupHistoryQ asks, per domain, for a successful backup of one of its items.
var backupHistoryQ = map[string]string{
	"containers": `SELECT EXISTS (SELECT 1 FROM runs WHERE kind = 'backup' AND status = 'success' AND target_id IN (SELECT id FROM targets))`,
	"vms":        `SELECT EXISTS (SELECT 1 FROM runs WHERE kind = 'backup' AND status = 'success' AND target_id IN (SELECT id FROM vms))`,
	"files":      `SELECT EXISTS (SELECT 1 FROM runs WHERE kind = 'backup' AND status = 'success' AND target_id IN (SELECT id FROM file_sets))`,
}

// ReadPlacement reads a domain's default and rules in one transaction, so a run
// decides by one state. One rule that cannot be read fails the whole read.
func (r *Repo) ReadPlacement(domain string) (PlacementState, error) {
	st := PlacementState{Domain: domain}
	err := r.inTx(func(tx *sql.Tx) error {
		var err error
		if st.Default, st.HasDefault, err = placementDefaultQ(tx, domain); err != nil {
			return err
		}
		st.Rules, err = copyRulesQ(tx, domain)
		return err
	})
	if err != nil {
		return PlacementState{}, fmt.Errorf("ReadPlacement %s: %w", domain, err)
	}
	return st, nil
}

// PlacementDefaultFor returns a domain's default, if it has one.
func (r *Repo) PlacementDefaultFor(domain string) (PlacementDefault, bool, error) {
	return placementDefaultQ(r.db, domain)
}

// ListPlacementDefaults returns every stored default, by domain.
func (r *Repo) ListPlacementDefaults() ([]PlacementDefault, error) {
	rows, err := r.db.Query(`SELECT domain, home, skip, confirmed_at, confirmed_manually, updated_at FROM placement_defaults ORDER BY domain`)
	if err != nil {
		return nil, fmt.Errorf("ListPlacementDefaults: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := []PlacementDefault{}
	for rows.Next() {
		d, err := scanPlacementDefault(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// PutPlacementDefault writes a domain's home and skip. A new row starts
// unpaused; an existing one keeps its pause.
func (r *Repo) PutPlacementDefault(domain, home string, skip []string) (PlacementDefault, error) {
	if err := checkPlacementDomain(domain); err != nil {
		return PlacementDefault{}, err
	}
	raw, err := encodeSkip(skip)
	if err != nil {
		return PlacementDefault{}, err
	}
	now := time.Now().Unix()
	if _, err := r.db.Exec(`INSERT INTO placement_defaults (domain, home, skip, confirmed_at, updated_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(domain) DO UPDATE SET home = excluded.home, skip = excluded.skip, updated_at = excluded.updated_at`,
		domain, home, raw, now, now); err != nil {
		return PlacementDefault{}, fmt.Errorf("PutPlacementDefault: %w", err)
	}
	d, _, err := r.PlacementDefaultFor(domain)
	return d, err
}

// PausePlacement stops a domain's replication until its default is confirmed,
// creating the row on the domain path when there is none. It reports whether
// this call started the pause.
func (r *Repo) PausePlacement(domain string) (bool, error) {
	if err := checkPlacementDomain(domain); err != nil {
		return false, err
	}
	started := false
	err := r.inTx(func(tx *sql.Tx) error {
		var confirmed int64
		err := tx.QueryRow(`SELECT confirmed_at FROM placement_defaults WHERE domain = ?`, domain).Scan(&confirmed)
		now := time.Now().Unix()
		switch {
		case errors.Is(err, sql.ErrNoRows):
			_, err = tx.Exec(`INSERT INTO placement_defaults (domain, home, skip, confirmed_at, updated_at)
				VALUES (?, '', '[]', 0, ?)`, domain, now)
		case err != nil:
			return err
		case confirmed == 0:
			return nil
		default:
			_, err = tx.Exec(`UPDATE placement_defaults SET confirmed_at = 0, updated_at = ? WHERE domain = ?`, now, domain)
		}
		started = err == nil
		return err
	})
	if err != nil {
		return false, fmt.Errorf("PausePlacement %s: %w", domain, err)
	}
	return started, nil
}

// ConfirmPlacement ends a domain's pause and marks the default manually
// confirmed for good. Every name in exclude gets the rule ["*"] in the same
// transaction; such a name needs no item row.
func (r *Repo) ConfirmPlacement(domain string, exclude []string) error {
	if err := checkPlacementDomain(domain); err != nil {
		return err
	}
	now := time.Now().Unix()
	err := r.inTx(func(tx *sql.Tx) error {
		for _, identity := range exclude {
			if err := setCopyRuleTx(tx, domain, identity, []string{SkipAll}, now); err != nil {
				return err
			}
		}
		_, err := tx.Exec(`INSERT INTO placement_defaults (domain, home, skip, confirmed_at, confirmed_manually, updated_at)
			VALUES (?, '', '[]', ?, 1, ?)
			ON CONFLICT(domain) DO UPDATE SET confirmed_at = excluded.confirmed_at, confirmed_manually = 1, updated_at = excluded.updated_at`,
			domain, now, now)
		return err
	})
	if err != nil {
		return fmt.Errorf("ConfirmPlacement %s: %w", domain, err)
	}
	return nil
}

// DomainHasHistory reports whether an item of the domain was ever backed up
// successfully, and whether the domain was ever replicated successfully.
func (r *Repo) DomainHasHistory(domain string) (backup, offsite bool, err error) {
	q, ok := backupHistoryQ[domain]
	if !ok {
		return false, false, fmt.Errorf("DomainHasHistory: %q has no placement", domain)
	}
	if err := r.db.QueryRow(q).Scan(&backup); err != nil {
		return false, false, fmt.Errorf("DomainHasHistory: %w", err)
	}
	if err := r.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM offsite_runs WHERE domain = ? AND ok = 1)`, domain).Scan(&offsite); err != nil {
		return false, false, fmt.Errorf("DomainHasHistory: %w", err)
	}
	return backup, offsite, nil
}

func placementDomainsUsingRepoTx(tx *sql.Tx, repoID string) ([]string, error) {
	rows, err := tx.Query(`SELECT domain FROM placement_defaults WHERE home = ? ORDER BY domain`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := []string{}
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			return nil, err
		}
		out = append(out, domain)
	}
	return out, rows.Err()
}

func placementDefaultQ(q queryer, domain string) (PlacementDefault, bool, error) {
	d, err := scanPlacementDefault(q.QueryRow(`SELECT domain, home, skip, confirmed_at, confirmed_manually, updated_at
		FROM placement_defaults WHERE domain = ?`, domain))
	if errors.Is(err, sql.ErrNoRows) {
		return PlacementDefault{}, false, nil
	}
	if err != nil {
		return PlacementDefault{}, false, err
	}
	return d, true, nil
}

func scanPlacementDefault(s scanner) (PlacementDefault, error) {
	var d PlacementDefault
	var raw string
	if err := s.Scan(&d.Domain, &d.Home, &raw, &d.ConfirmedAt, &d.ConfirmedManually, &d.UpdatedAt); err != nil {
		return PlacementDefault{}, err
	}
	skip, err := decodeSkip(raw)
	if err != nil {
		return PlacementDefault{}, fmt.Errorf("placement default %s: %w", d.Domain, err)
	}
	d.Skip = skip
	return d, nil
}

func checkPlacementDomain(domain string) error {
	if !slices.Contains(PlacementDomains, domain) {
		return fmt.Errorf("%q has no placement", domain)
	}
	return nil
}
