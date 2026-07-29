package api

import "github.com/junkerderprovinz/bombvault/internal/store"

// primaryOffsiteTarget returns the first ENABLED off-site target for a domain in
// the store's per-domain order (sort_order, then created_at), or ok=false when
// the domain has no enabled off-site target.
//
// Stage 1: this helper is dormant. Nothing in the live replication path calls it
// yet — offsiteRepoFor and copyToOffsite still read the single-repo Settings
// columns, so behavior is unchanged. Stage 2 rewires callers onto this. It is
// proven correct by a unit test in the meantime.
func (s *Service) primaryOffsiteTarget(domain string) (store.OffsiteTarget, bool) {
	targets, err := s.store.OffsiteTargetsForDomain(domain)
	if err != nil {
		return store.OffsiteTarget{}, false
	}
	for _, t := range targets {
		if t.Enabled {
			return t, true
		}
	}
	return store.OffsiteTarget{}, false
}
