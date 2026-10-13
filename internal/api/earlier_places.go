package api

import (
	"errors"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// An earlier place is where an item's backups were written before it followed
// its section somewhere else. A timeline lists it beside the item's home and
// targets, and a restore or a listing reaches it as a source of its own:
// folderPlace for the section's own folder, repoPlacePrefix and an id for a
// named repository.
const (
	folderPlace     = "folder"
	repoPlacePrefix = "repo:"
)

var (
	errInvalidItemSource = errors.New("invalid source: the entry's location, one of its targets or a place that holds its earlier backups")
	errNotEarlierPlace   = errors.New("this storage location holds no earlier backups of this entry")
	errEarlierDeleteAll  = errors.New("backups at an earlier storage location are deleted one at a time from the entry's list of backups")
)

// earlierPlace is the place of a former home.
func earlierPlace(repoID string) string {
	if repoID == "" {
		return folderPlace
	}
	return repoPlacePrefix + repoID
}

// isEarlierSource reports whether a source names an earlier place, well
// formed or not.
func isEarlierSource(source string) bool {
	return source == folderPlace || strings.HasPrefix(source, repoPlacePrefix)
}

// earlierRepoID is the repository id an earlier place names, "" for the
// section's own folder. ok is false for a malformed one.
func earlierRepoID(place string) (id string, ok bool) {
	if place == folderPlace {
		return "", true
	}
	id, ok = strings.CutPrefix(place, repoPlacePrefix)
	return id, ok && validOffsiteTargetID(id)
}

// validItemSource reports whether an entry's backups can be read from source.
func validItemSource(source string) bool {
	return source == "local" || isOffsiteSource(source) || isEarlierSource(source)
}

// leftBehind are the identities whose items left the repository r of domain
// for another place. A prune there keeps their backups as they are: they
// receive no new ones, so a rule that ages them would only ever take away.
func (s *Service) leftBehind(settings store.Settings, domain string, r domainRepoRef) ([]string, error) {
	if !validPlacementDomain(domain) || (!r.Own && r.Named.ID == "") {
		return nil, nil
	}
	repoID := r.Named.ID
	p, err := s.readPlacement(settings, domain)
	if err != nil {
		return nil, err
	}
	items, err := s.domainItems(domain)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, it := range items {
		if home, _ := p.effectiveHome(it.home); home == repoID {
			continue
		}
		former, err := s.store.FormerHomes(domain, it.rowID)
		if err != nil {
			return nil, err
		}
		if slices.Contains(former, repoID) {
			out = append(out, it.identity)
		}
	}
	return out, nil
}

// earlierRepo resolves an earlier place of the item rowID. A place the item
// never left is refused, so a source cannot open any other repository.
func (s *Service) earlierRepo(settings store.Settings, domain, rowID, place string) (string, error) {
	repoID, ok := earlierRepoID(place)
	if !ok || rowID == "" {
		return "", errNotEarlierPlace
	}
	former, err := s.store.FormerHomes(domain, rowID)
	if err != nil {
		return "", err
	}
	if !slices.Contains(former, repoID) {
		return "", errNotEarlierPlace
	}
	return s.itemRepoPath(repoID, func() (string, error) { return s.repoFor(settings, domain, "local") })
}
