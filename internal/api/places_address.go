package api

import (
	"context"
	"fmt"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

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

// addressMove is one address of a place before and after an edit.
type addressMove struct {
	Domain  string
	Old     string
	New     string
	OldMode restic.Mode // opens the old address
	Facts   addressFacts
}

// placeEstablishedErr is a place-location-established refusal carrying what
// lies at the old addresses.
type placeEstablishedErr struct {
	snapshots int
	domains   []string
}

func (e *placeEstablishedErr) Error() string { return errPlaceLocationEstablished.Error() }

func (e *placeEstablishedErr) Is(target error) bool { return target == errPlaceLocationEstablished }

// placeMoves lists every address of the place that an edit from before to
// after changes: each row's, and the path of each domain whose home it is.
func (s *Service) placeMoves(settings store.Settings, before, after store.Place, rows []store.OffsiteTarget, homes map[string]string) ([]addressMove, error) {
	var out []addressMove
	for _, r := range rows {
		// A domain's primary row follows its path, which the homes below cover.
		if r.Role == store.RolePrimary {
			continue
		}
		addr, ok := store.PlaceAddress(after, r.PlaceDomain, r.PlaceSuffix)
		if !ok {
			return nil, fmt.Errorf("%w: a repository of %s lies there", store.ErrPlaceInUse, r.PlaceDomain)
		}
		if sameRepoLocation(addr, r.Repo) {
			continue
		}
		facts, err := s.rowFacts(r)
		if err != nil {
			return nil, err
		}
		out = append(out, addressMove{Domain: r.PlaceDomain, Old: r.Repo, New: addr, OldMode: s.rowMode(settings, r), Facts: facts})
	}
	for _, d := range places.Domains {
		if homes[d] != before.ID {
			continue
		}
		addr, ok := store.PlaceAddress(after, d, "")
		if !ok {
			return nil, errPlaceHomeDomain
		}
		old := domainPathRaw(d, settings)
		if sameRepoLocation(addr, old) {
			continue
		}
		facts, err := s.domainPathFacts(settings, d)
		if err != nil {
			return nil, err
		}
		loc, err := s.resolveRepo(old)
		if err != nil {
			return nil, err
		}
		out = append(out, addressMove{Domain: d, Old: old, New: addr, OldMode: s.primaryModeFor(settings, d, loc), Facts: facts})
	}
	return out, nil
}

// rowMode is the mode a row's own repository opens with.
func (s *Service) rowMode(settings store.Settings, r store.OffsiteTarget) restic.Mode {
	if r.Role == store.RoleRepo && r.CompanionOf == "" {
		return s.applyTargetCreds(s.ModeFor(settings), settings, r)
	}
	return s.offsiteModeForTarget(settings, r)
}

// checkMoves allows each move on its own when the new address is empty and
// nothing lies at the old one, or when both hold the same restic repository,
// which is a repository moved by hand. newMode opens the new addresses.
func (s *Service) checkMoves(ctx context.Context, moves []addressMove, newMode restic.Mode) error {
	refused := &placeEstablishedErr{domains: []string{}}
	for _, m := range moves {
		at := s.probeFolder(ctx, m.New, newMode)
		if at.problem != nil {
			return &probeFailedErr{result: places.ProbeResult{Code: at.problem.Code, Error: at.problem.Message}}
		}
		held := at.state == places.FolderRepository
		if !held && !m.Facts.Established {
			continue
		}
		if held && s.probeFolder(ctx, m.Old, m.OldMode).repoID == at.repoID {
			continue
		}
		refused.snapshots += m.Facts.Snapshots
		if !slices.Contains(refused.domains, m.Domain) {
			refused.domains = append(refused.domains, m.Domain)
		}
	}
	if len(refused.domains) > 0 {
		return refused
	}
	return nil
}
