package api

import (
	"log"
	"net/http"
	"net/url"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// DomainRow is one line of the Domains card: where the domain stores, where
// it copies to, and which items chose otherwise. It comes from the database;
// HomeHasBackups stays unset here and is answered by the home preview.
type DomainRow struct {
	Domain         string          `json:"domain"`
	HomePlace      string          `json:"homePlace"`
	StoredIn       string          `json:"storedIn"`
	HomeHasBackups *bool           `json:"homeHasBackups,omitempty"`
	Chips          []DomainChip    `json:"chips"`
	Exceptions     []ExceptionItem `json:"exceptions"`
	Paused         bool            `json:"paused"`
	Schedule       string          `json:"schedule"`
	Unreadable     bool            `json:"unreadable"`
}

// DomainChip is one place under "Copied to".
type DomainChip struct {
	PlaceID  string `json:"placeId"`
	TargetID string `json:"targetId,omitempty"`
	On       bool   `json:"on"`
	Disabled bool   `json:"disabled"`
	Reason   string `json:"reason,omitempty"` // "off" or "creds-differ"
}

// ExceptionItem is an item that chose a place or copies of its own.
type ExceptionItem struct {
	Identity string `json:"identity"`
	Name     string `json:"name"`
	Link     string `json:"link"`
}

// storageDomainRows is the five rows of the Domains card.
func (s *Service) storageDomainRows() ([]DomainRow, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, err
	}
	all, err := s.store.ListPlaces()
	if err != nil {
		return nil, err
	}
	homes, err := s.store.DomainPlaces()
	if err != nil {
		return nil, err
	}
	named, err := s.namedRepoIndex()
	if err != nil {
		return nil, err
	}
	targets, err := s.store.ListOffsiteTargets()
	if err != nil {
		return nil, err
	}
	rows := make([]DomainRow, 0, len(places.Domains))
	for _, d := range places.Domains {
		row, err := s.storageDomainRow(settings, d, all, homes[d], named, targets)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// storageDomainRow leaves the home place out of the chips, and the default's
// place when the default is a plain named repository there: those places are
// where the domain stores. A default on a direct repository keeps its
// target's chip, since that target still takes the project folders and the
// items on the domain path.
func (s *Service) storageDomainRow(settings store.Settings, domain string, all []store.Place, home string,
	named map[string]store.OffsiteTarget, targets []store.OffsiteTarget) (DomainRow, error) {
	row := DomainRow{Domain: domain, HomePlace: home, StoredIn: home, Chips: []DomainChip{}, Exceptions: []ExceptionItem{},
		Schedule: digestBackupScheduleFor(domain, settings)}
	var skip []string
	defaultPlace := ""
	if validPlacementDomain(domain) {
		p, err := s.readPlacement(settings, domain)
		if err != nil {
			log.Printf("api: placement %s: could not read its placement, marking its domain row unreadable: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
			row.Unreadable = true
			return row, nil
		}
		row.Paused, skip = p.State.Paused(), p.defaultSkip()
		if repo, ok := named[p.State.Default.Home]; ok {
			if row.StoredIn, err = s.repoPlace(repo); err != nil {
				return row, err
			}
			if repo.CompanionOf == "" {
				defaultPlace = repo.PlaceID
			}
		}
		if row.Exceptions, err = s.domainExceptions(p); err != nil {
			return row, err
		}
	}
	src, srcEnv := s.remotePathEnv(settings, domain)
	for _, pl := range all {
		if _, offered := pl.Folders[domain]; !offered || pl.ID == home || (defaultPlace != "" && pl.ID == defaultPlace) ||
			holdsOnlyItsRepository(pl, domain, targets, named) {
			continue
		}
		row.Chips = append(row.Chips, s.domainChip(pl, domain, targets, skip, src, srcEnv))
	}
	return row, nil
}

// holdsOnlyItsRepository reports whether a place that is itself a repository
// already holds one serving the domain and no target of it: a target there
// would lie inside that repository, which the chip's write refuses.
func holdsOnlyItsRepository(pl store.Place, domain string, targets []store.OffsiteTarget, named map[string]store.OffsiteTarget) bool {
	var rows []store.OffsiteTarget
	for _, t := range targets {
		if t.PlaceID == pl.ID {
			rows = append(rows, t)
		}
	}
	for _, r := range named {
		if r.PlaceID == pl.ID {
			rows = append(rows, r)
		}
	}
	a := domainAt(pl, rows, "", domain)
	return a.target == nil && a.repo != nil && placeIsRepository(pl, rows)
}

// repoPlace is the place a named repository lies at; a direct repository
// without one of its own counts at its target's.
func (s *Service) repoPlace(repo store.OffsiteTarget) (string, error) {
	if repo.PlaceID != "" || repo.CompanionOf == "" {
		return repo.PlaceID, nil
	}
	target, _, err := s.store.GetOffsiteTarget(repo.CompanionOf)
	return target.PlaceID, err
}

// domainChip is the chip of one place. A switched-off place turned its target
// off too, so its chip of a copies domain shows what the default keeps.
func (s *Service) domainChip(pl store.Place, domain string, targets []store.OffsiteTarget, skip []string, src string, srcEnv []string) DomainChip {
	c := DomainChip{PlaceID: pl.ID}
	for _, t := range targets {
		if t.PlaceID != pl.ID || t.Domain != domain {
			continue
		}
		c.TargetID = t.ID
		c.On = !skipsTarget(skip, t.ID) && (t.Enabled || (!pl.Enabled && validPlacementDomain(domain)))
		break
	}
	switch {
	case !pl.Enabled:
		c.Disabled, c.Reason = true, "off"
	case src != "" && s.credsClash(pl, domain, src, srcEnv):
		c.Reason = "creds-differ"
	}
	return c
}

// credsClash reports whether a copy from the domain's remote path src to the
// place needs a variable the place sets to another value, which one restic
// copy cannot carry. The copy itself and the target chips of an item card
// judge it by the same copyNeeds.
func (s *Service) credsClash(pl store.Place, domain, src string, srcEnv []string) bool {
	env, err := s.placeEnv(pl)
	if err != nil {
		log.Printf("api: place %s: could not build its environment: %v", pl.ID, err) //nolint:gosec // G706: the id is store-generated
		return false
	}
	dest, _ := store.PlaceAddress(pl, domain, "")
	_, ok := copyNeeds(dest, env, src, srcEnv)
	return !ok
}

// remotePathEnv is a domain's remote path and the environment it opens with,
// "" for a local path, which a copy needs no variables for.
func (s *Service) remotePathEnv(settings store.Settings, domain string) (string, []string) {
	loc, err := s.repoFor(settings, domain, "local")
	if err != nil || !restic.IsRemoteRepo(loc) {
		return "", nil
	}
	return loc, s.primaryModeFor(settings, domain, loc).Env
}

// domainExceptions are the items with a copy rule of their own or a
// repository other than the default's, linked to their cards.
func (s *Service) domainExceptions(p placementRead) ([]ExceptionItem, error) {
	items, err := s.domainItems(p.Domain)
	if err != nil {
		return nil, err
	}
	out := []ExceptionItem{}
	for _, it := range items {
		_, ownRule := p.State.Rules[it.identity]
		ownHome := it.home.Repo != "" && it.home.Repo != p.State.Default.Home
		if ownRule || ownHome {
			out = append(out, ExceptionItem{Identity: it.identity, Name: it.label, Link: "/" + p.Domain + "?item=" + url.QueryEscape(it.ref.Key)})
		}
	}
	return out, nil
}

// handleStorageDomains serves GET /api/storage/domains.
func (h *Handler) handleStorageDomains(w http.ResponseWriter, _ *http.Request) {
	rows, err := h.svc.storageDomainRows()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"domains": rows}))
}
