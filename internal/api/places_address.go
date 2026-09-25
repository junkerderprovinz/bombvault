package api

import "github.com/junkerderprovinz/bombvault/internal/store"

// addressFacts is what the database knows about one address of a place,
// without asking the backend.
type addressFacts struct {
	Established bool // a repository there holds backups, or once did
	Snapshots   int  // snapshots counted there when it was last listed or measured
	Items       int  // items whose backups go there
}

// rowFacts answers for a target, a named repository or a direct one.
func (s *Service) rowFacts(row store.OffsiteTarget) (addressFacts, error) {
	var f addressFacts
	if loc, err := s.resolveRepo(row.Repo); err == nil {
		// A store that cannot tell counts as established: the answer guards
		// backups that may lie there.
		f.Established = s.repoEstablishmentOf(loc) != repoNeverEstablished
	}
	switch row.Role {
	case store.RoleRepo:
		n, err := s.store.ItemsUsingNamedRepo(row.ID)
		if err != nil {
			return f, err
		}
		f.Items = n
		f.Established = f.Established || n > 0
	case store.RoleOffsite:
		copies, err := s.store.ItemCopiesForDomain(row.Domain)
		if err != nil {
			return f, err
		}
		for _, c := range copies {
			if c.TargetID == row.ID {
				f.Snapshots += c.SnapshotCount
			}
		}
		if f.Snapshots == 0 {
			stat, found, err := s.store.LatestRepoStat(row.Domain, offsiteStatSource(row.ID))
			if err != nil {
				return f, err
			}
			if found {
				f.Snapshots = int(stat.Snapshots)
			}
		}
		_, copied, err := s.store.LatestSuccessfulOffsiteRunForTarget(row.Domain, row.ID)
		if err != nil {
			return f, err
		}
		f.Established = f.Established || copied || f.Snapshots > 0
	}
	return f, nil
}

// domainPathFacts answers for a domain's own path.
func (s *Service) domainPathFacts(settings store.Settings, domain string) (addressFacts, error) {
	var f addressFacts
	if loc, err := s.repoFor(settings, domain, "local"); err == nil {
		f.Established = s.repoEstablishmentOf(loc) != repoNeverEstablished
	}
	stat, found, err := s.store.LatestRepoStat(domain, "local")
	if err != nil {
		return f, err
	}
	if found {
		f.Snapshots = int(stat.Snapshots)
	}
	backed, err := s.store.DomainPathBackedUp(domain)
	if err != nil {
		return f, err
	}
	f.Established = f.Established || backed || f.Snapshots > 0
	return f, nil
}
