package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// RestoreDrill is one restore-verification run for a domain and source, which
// reads back real pack data to prove the backup is restorable. It backs the
// "last verified restorable" badge.
type RestoreDrill struct {
	Domain string `json:"domain"`
	Source string `json:"source"`
	At     int64  `json:"at"`     // unix seconds the drill ran
	OK     bool   `json:"ok"`     // true when the checked data was intact
	Detail string `json:"detail"` // short scrubbed reason on failure; empty on success
	// Kind is the drill flavour: "subset" (the default; `restic check
	// --read-data-subset`) or "dr" (a real sandbox restore from off-site).
	Kind string `json:"kind"`
	// OffsiteTargetID names the off-site target the drill read, so one named
	// repository going bad is a finding of its own rather than a gap in the
	// domain's history. Empty for a local drill and for a domain-wide one.
	OffsiteTargetID string `json:"offsiteTargetId"`
}

// DrillKey is one series of restore drills: a domain and source, optionally one
// named off-site target, per drill flavour.
type DrillKey struct{ Domain, Source, TargetID, Kind string }

// defaultRestoreDrillLimit caps an unbounded ListRestoreDrills request.
const defaultRestoreDrillLimit = 365

// AddRestoreDrill records a restore-verification drill result. An empty Kind
// defaults to "subset", like the column default.
func (r *Repo) AddRestoreDrill(d RestoreDrill) error {
	if d.Kind == "" {
		d.Kind = "subset"
	}
	_, err := r.db.Exec(`
		INSERT INTO restore_drills (domain, source, at, ok, detail, kind, offsite_target_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		d.Domain, d.Source, d.At, boolInt(d.OK), d.Detail, d.Kind, d.OffsiteTargetID,
	)
	if err != nil {
		return fmt.Errorf("AddRestoreDrill: %w", err)
	}
	return nil
}

// LatestRestoreDrill returns the most recent drill for a domain and source,
// regardless of kind. The bool is false (with a zero RestoreDrill) when none
// has been recorded yet.
func (r *Repo) LatestRestoreDrill(domain, source string) (RestoreDrill, bool, error) {
	row := r.db.QueryRow(`
		SELECT domain, source, at, ok, detail, kind, offsite_target_id
		FROM restore_drills
		WHERE domain = ? AND source = ?
		ORDER BY at DESC
		LIMIT 1`, domain, source)
	d, err := scanRestoreDrill(row)
	if errors.Is(err, sql.ErrNoRows) {
		return RestoreDrill{}, false, nil
	}
	if err != nil {
		return RestoreDrill{}, false, fmt.Errorf("LatestRestoreDrill: %w", err)
	}
	return d, true, nil
}

// LatestRestoreDrillKind returns the most recent drill of one kind ("subset" or
// "dr") for a domain and source, so the newest DR drill is found even when
// subset checks ran since. The bool is false when that kind has never been
// recorded.
func (r *Repo) LatestRestoreDrillKind(domain, source, kind string) (RestoreDrill, bool, error) {
	row := r.db.QueryRow(`
		SELECT domain, source, at, ok, detail, kind, offsite_target_id
		FROM restore_drills
		WHERE domain = ? AND source = ? AND kind = ?
		ORDER BY at DESC
		LIMIT 1`, domain, source, kind)
	d, err := scanRestoreDrill(row)
	if errors.Is(err, sql.ErrNoRows) {
		return RestoreDrill{}, false, nil
	}
	if err != nil {
		return RestoreDrill{}, false, fmt.Errorf("LatestRestoreDrillKind: %w", err)
	}
	return d, true, nil
}

// ListRestoreDrills returns up to limit drills for a domain and source, newest
// first. A limit of 0 or less falls back to defaultRestoreDrillLimit.
func (r *Repo) ListRestoreDrills(domain, source string, limit int) ([]RestoreDrill, error) {
	if limit <= 0 {
		limit = defaultRestoreDrillLimit
	}
	rows, err := r.db.Query(`
		SELECT domain, source, at, ok, detail, kind, offsite_target_id
		FROM restore_drills
		WHERE domain = ? AND source = ?
		ORDER BY at DESC
		LIMIT ?`, domain, source, limit)
	if err != nil {
		return nil, fmt.Errorf("ListRestoreDrills: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []RestoreDrill
	for rows.Next() {
		d, sErr := scanRestoreDrill(rows)
		if sErr != nil {
			return nil, sErr
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ListRestoreDrillKeys returns every series of drills that has rows, so a
// regression check runs over exactly the checks this installation performs.
func (r *Repo) ListRestoreDrillKeys() ([]DrillKey, error) {
	rows, err := r.db.Query(`
		SELECT DISTINCT domain, source, offsite_target_id, kind
		FROM restore_drills
		ORDER BY domain, source, offsite_target_id, kind`)
	if err != nil {
		return nil, fmt.Errorf("ListRestoreDrillKeys: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []DrillKey
	for rows.Next() {
		var k DrillKey
		if sErr := rows.Scan(&k.Domain, &k.Source, &k.TargetID, &k.Kind); sErr != nil {
			return nil, fmt.Errorf("ListRestoreDrillKeys: %w", sErr)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// ListRestoreDrillsKind returns up to limit drills of one series, newest first.
// It reads one kind and one off-site target, so a nightly subset check cannot
// bury the handful of DR drills below it.
func (r *Repo) ListRestoreDrillsKind(domain, source, targetID, kind string, limit int) ([]RestoreDrill, error) {
	if limit <= 0 {
		limit = defaultRestoreDrillLimit
	}
	rows, err := r.db.Query(`
		SELECT domain, source, at, ok, detail, kind, offsite_target_id
		FROM restore_drills
		WHERE domain = ? AND source = ? AND offsite_target_id = ? AND kind = ?
		ORDER BY at DESC, rowid DESC
		LIMIT ?`, domain, source, targetID, kind, limit)
	if err != nil {
		return nil, fmt.Errorf("ListRestoreDrillsKind: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []RestoreDrill
	for rows.Next() {
		d, sErr := scanRestoreDrill(rows)
		if sErr != nil {
			return nil, fmt.Errorf("ListRestoreDrillsKind: %w", sErr)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func scanRestoreDrill(s scanner) (RestoreDrill, error) {
	var d RestoreDrill
	var ok int
	if err := s.Scan(&d.Domain, &d.Source, &d.At, &ok, &d.Detail, &d.Kind, &d.OffsiteTargetID); err != nil {
		return RestoreDrill{}, err
	}
	d.OK = ok != 0
	return d, nil
}
