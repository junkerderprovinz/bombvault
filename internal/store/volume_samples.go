package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// VolumeSample is how much room one volume had at one moment. Samples are taken
// on every backup attempt, so the free-space trend survives a domain whose
// statistics pass is throttled or never ran.
type VolumeSample struct {
	Volume     string
	At         int64
	FreeBytes  int64
	TotalBytes *int64 // nil when the backend reports no size
	Domains    []string
	Source     string
}

// AddVolumeSample records one reading. Two readings of one volume in the same
// second are the same sample, and the later one wins.
func (r *Repo) AddVolumeSample(v VolumeSample) error {
	var total any
	if v.TotalBytes != nil {
		total = *v.TotalBytes
	}
	_, err := r.db.Exec(`
		INSERT INTO volume_samples (volume, at, free_bytes, total_bytes, domains, source)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(volume, at) DO UPDATE SET
			free_bytes  = excluded.free_bytes,
			total_bytes = excluded.total_bytes,
			domains     = excluded.domains,
			source      = excluded.source`,
		v.Volume, v.At, v.FreeBytes, total, strings.Join(v.Domains, ","), v.Source)
	if err != nil {
		return fmt.Errorf("AddVolumeSample: %w", err)
	}
	return nil
}

// ListVolumeSamples returns the samples taken at or after since, oldest first,
// which is the order the capacity rule fits its slope over.
func (r *Repo) ListVolumeSamples(since int64) ([]VolumeSample, error) {
	rows, err := r.db.Query(`
		SELECT volume, at, free_bytes, total_bytes, domains, source
		FROM volume_samples
		WHERE at >= ?
		ORDER BY at ASC, volume ASC`, since)
	if err != nil {
		return nil, fmt.Errorf("ListVolumeSamples: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []VolumeSample
	for rows.Next() {
		var v VolumeSample
		var total sql.NullInt64
		var domains string
		if sErr := rows.Scan(&v.Volume, &v.At, &v.FreeBytes, &total, &domains, &v.Source); sErr != nil {
			return nil, fmt.Errorf("ListVolumeSamples: %w", sErr)
		}
		v.TotalBytes = nullableInt(total)
		if domains != "" {
			v.Domains = strings.Split(domains, ",")
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// PruneVolumeSamples deletes readings older than the cutoff and returns how
// many went.
func (r *Repo) PruneVolumeSamples(before int64) (int64, error) {
	res, err := r.db.Exec(`DELETE FROM volume_samples WHERE at < ?`, before)
	if err != nil {
		return 0, fmt.Errorf("PruneVolumeSamples: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
