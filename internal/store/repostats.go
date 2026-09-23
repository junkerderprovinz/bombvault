package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// RepoStat is one sampled repository-size measurement for a domain + source,
// taken after a successful backup. It powers the dashboard's size/dedup trend.
type RepoStat struct {
	Domain      string `json:"domain"`
	Source      string `json:"source"`
	At          int64  `json:"at"`          // unix seconds the sample was taken
	RawSize     int64  `json:"rawSize"`     // physical (deduplicated + compressed) repo size
	RestoreSize int64  `json:"restoreSize"` // logical restore size
	Snapshots   int64  `json:"snapshots"`   // snapshot count at sample time
}

// defaultRepoStatLimit caps an unbounded ListRepoStats request at about a year
// of daily samples.
const defaultRepoStatLimit = 365

// AddRepoStat records a repository-size sample.
func (r *Repo) AddRepoStat(s RepoStat) error {
	_, err := r.db.Exec(`
		INSERT INTO repo_stats (domain, source, at, raw_size, restore_size, snapshots)
		VALUES (?, ?, ?, ?, ?, ?)`,
		s.Domain, s.Source, s.At, s.RawSize, s.RestoreSize, s.Snapshots,
	)
	if err != nil {
		return fmt.Errorf("AddRepoStat: %w", err)
	}
	return nil
}

// LatestRepoStat returns the most recent sample for a domain and source, or
// false when none exists.
func (r *Repo) LatestRepoStat(domain, source string) (RepoStat, bool, error) {
	row := r.db.QueryRow(`
		SELECT domain, source, at, raw_size, restore_size, snapshots
		FROM repo_stats
		WHERE domain = ? AND source = ?
		ORDER BY at DESC
		LIMIT 1`, domain, source)
	stat, err := scanRepoStat(row)
	if errors.Is(err, sql.ErrNoRows) {
		return RepoStat{}, false, nil
	}
	if err != nil {
		return RepoStat{}, false, fmt.Errorf("LatestRepoStat: %w", err)
	}
	return stat, true, nil
}

// ListRepoStats returns the newest limit samples for a domain and source,
// oldest first so the dashboard can plot them left to right. A limit of 0 or
// less means defaultRepoStatLimit.
func (r *Repo) ListRepoStats(domain, source string, limit int) ([]RepoStat, error) {
	if limit <= 0 {
		limit = defaultRepoStatLimit
	}
	rows, err := r.db.Query(`
		SELECT domain, source, at, raw_size, restore_size, snapshots
		FROM repo_stats
		WHERE domain = ? AND source = ?
		ORDER BY at DESC
		LIMIT ?`, domain, source, limit)
	if err != nil {
		return nil, fmt.Errorf("ListRepoStats: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []RepoStat
	for rows.Next() {
		stat, sErr := scanRepoStat(rows)
		if sErr != nil {
			return nil, sErr
		}
		out = append(out, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func scanRepoStat(s scanner) (RepoStat, error) {
	var stat RepoStat
	err := s.Scan(
		&stat.Domain, &stat.Source, &stat.At,
		&stat.RawSize, &stat.RestoreSize, &stat.Snapshots,
	)
	if err != nil {
		return RepoStat{}, err
	}
	return stat, nil
}
