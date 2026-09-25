package store

import "fmt"

// PlaceSites is where the places put repositories: the off_premises answer of
// the place each row belongs to, and of each domain's home place. Rows and
// domains without a place are absent, and the caller supplies its own reading
// for them.
type PlaceSites struct {
	Rows    map[string]bool // offsite_targets id -> off_premises of its place
	Domains map[string]bool // domain -> off_premises of its home place
}

// Row reports whether a row stands off the premises: its place's answer, or
// unplaced for a row without one.
func (p PlaceSites) Row(id string, unplaced bool) bool {
	if off, ok := p.Rows[id]; ok {
		return off
	}
	return unplaced
}

// Domain is Row for a domain path, answered by its home place.
func (p PlaceSites) Domain(domain string, unplaced bool) bool {
	if off, ok := p.Domains[domain]; ok {
		return off
	}
	return unplaced
}

// PlaceSites reads where every place in use stands. The off_premises column of
// a row is not asked: UpsertOffsiteTarget keeps it 0 on targets and direct
// repositories, whatever their place says.
func (r *Repo) PlaceSites() (PlaceSites, error) {
	rows, err := r.placeAnswers(`SELECT t.id, p.off_premises FROM offsite_targets t
		JOIN storage_places p ON p.id = t.place_id`)
	if err != nil {
		return PlaceSites{}, fmt.Errorf("PlaceSites rows: %w", err)
	}
	domains, err := r.placeAnswers(`SELECT d.domain, p.off_premises FROM storage_domain_places d
		JOIN storage_places p ON p.id = d.place_id`)
	if err != nil {
		return PlaceSites{}, fmt.Errorf("PlaceSites domains: %w", err)
	}
	return PlaceSites{Rows: rows, Domains: domains}, nil
}

// placeAnswers runs a query of key and off_premises pairs.
func (r *Repo) placeAnswers(query string) (map[string]bool, error) {
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	out := map[string]bool{}
	for rows.Next() {
		var key string
		var off int
		if err := rows.Scan(&key, &off); err != nil {
			return nil, err
		}
		out[key] = off != 0
	}
	return out, rows.Err()
}
