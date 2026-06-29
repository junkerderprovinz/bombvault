package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// RestoreDrill is one recorded restore-verification "drill" for a domain +
// source: a `restic check --read-data-subset` run that read back real pack data
// to prove the backup is restorable. It powers the "last verified restorable"
// badge.
type RestoreDrill struct {
	Domain string `json:"domain"`
	Source string `json:"source"`
	At     int64  `json:"at"`     // unix seconds the drill ran
	OK     bool   `json:"ok"`     // true when the checked data was intact
	Detail string `json:"detail"` // short scrubbed reason on failure; empty on success
}

// defaultRestoreDrillLimit caps an unbounded ListRestoreDrills request.
const defaultRestoreDrillLimit = 365

// AddRestoreDrill records a restore-verification drill result.
func (r *Repo) AddRestoreDrill(d RestoreDrill) error {
	_, err := r.db.Exec(`
		INSERT INTO restore_drills (domain, source, at, ok, detail)
		VALUES (?, ?, ?, ?, ?)`,
		d.Domain, d.Source, d.At, boolInt(d.OK), d.Detail,
	)
	if err != nil {
		return fmt.Errorf("AddRestoreDrill: %w", err)
	}
	return nil
}

// LatestRestoreDrill returns the most recent drill for a domain + source. The
// bool is false (with a zero RestoreDrill) when none has been recorded yet.
func (r *Repo) LatestRestoreDrill(domain, source string) (RestoreDrill, bool, error) {
	row := r.db.QueryRow(`
		SELECT domain, source, at, ok, detail
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

// ListRestoreDrills returns up to limit drills for a domain + source, newest
// first (descending by `at`). A limit of 0 or less falls back to
// defaultRestoreDrillLimit.
func (r *Repo) ListRestoreDrills(domain, source string, limit int) ([]RestoreDrill, error) {
	if limit <= 0 {
		limit = defaultRestoreDrillLimit
	}
	rows, err := r.db.Query(`
		SELECT domain, source, at, ok, detail
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

func scanRestoreDrill(s scanner) (RestoreDrill, error) {
	var d RestoreDrill
	var ok int
	if err := s.Scan(&d.Domain, &d.Source, &d.At, &ok, &d.Detail); err != nil {
		return RestoreDrill{}, err
	}
	d.OK = ok != 0
	return d, nil
}
