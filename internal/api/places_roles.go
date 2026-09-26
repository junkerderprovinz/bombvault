package api

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// domainAtPlace is what one domain already has at one place.
type domainAtPlace struct {
	home   bool
	target *store.OffsiteTarget // the domain's target there
	repo   *store.OffsiteTarget // a named repository there serving the domain, direct or plain
}

func domainAt(p store.Place, rows []store.OffsiteTarget, homePlace, domain string) domainAtPlace {
	a := domainAtPlace{home: homePlace == p.ID}
	for i := range rows {
		switch r := &rows[i]; {
		case r.Role == store.RoleOffsite && r.PlaceDomain == domain && a.target == nil:
			a.target = r
		case r.Role == store.RoleRepo && (r.PlaceDomain == domain || r.PlaceDomain == "") && a.repo == nil:
			a.repo = r
		}
	}
	return a
}

// placeRepoFor is the repository a place stands for as the home of a domain's
// items: the domain path at its home place, the domain's repository there,
// the direct repository beside the domain's target there, or a new named
// one. With create false it only says what it would create, "direct" or
// "repository".
func (s *Service) placeRepoFor(ctx context.Context, p store.Place, domain string, create bool) (string, string, error) {
	if !validPlacementDomain(domain) {
		return "", "", errInvalidPlacement
	}
	if !p.Enabled {
		return "", "", errPlaceOff
	}
	homes, err := s.store.DomainPlaces()
	if err != nil {
		return "", "", err
	}
	rows, err := s.store.PlaceRows(p.ID)
	if err != nil {
		return "", "", err
	}
	a := domainAt(p, rows, homes[domain], domain)
	switch {
	case a.home:
		return "", "", nil
	case a.repo != nil:
		return a.repo.ID, "", nil
	case a.target != nil:
		direct, found, err := s.store.CompanionFor(a.target.ID)
		switch {
		case err != nil:
			return "", "", err
		case found:
			return direct.ID, "", nil
		case placeIsRepository(p, rows):
			return "", "", errPlaceIsRepository
		case !create:
			return "", "direct", nil
		}
		id, err := s.createPlaceDirect(ctx, p, *a.target, domain)
		return id, "", err
	}
	if _, ok := p.Folders[domain]; !ok {
		return "", "", store.ErrPlaceDomainUnavailable
	}
	// Every domain at a place that is itself a repository shares that one
	// repository, so it is made without a domain, and only while the base
	// serves nothing else.
	named := domain
	if placeIsRepository(p, rows) {
		if len(rows) > 0 || slices.Contains(slices.Collect(maps.Values(homes)), p.ID) {
			return "", "", errPlaceIsRepository
		}
		named = ""
	}
	if !create {
		return "", "repository", nil
	}
	id, err := s.createPlaceNamed(p, named)
	return id, "", err
}

// createPlaceDirect makes the direct repository beside a target the way the
// direct repository dialog does, and puts it at the place.
func (s *Service) createPlaceDirect(ctx context.Context, p store.Place, target store.OffsiteTarget, domain string) (string, error) {
	addr, _ := store.PlaceAddress(p, domain, directSuffix)
	row, err := s.createDirectRepo(ctx, target.ID, "", addr)
	if err != nil {
		return "", err
	}
	if _, err := s.store.AdoptRow(row.ID, p.ID, domain, directSuffix); err != nil {
		return "", err
	}
	return row.ID, nil
}

// createPlaceNamed writes a named repository for the domain at the place,
// after the location check every named repository gets.
func (s *Service) createPlaceNamed(p store.Place, domain string) (string, error) {
	addr, _ := store.PlaceAddress(p, domain, "")
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", err
	}
	loc, err := s.resolveRepo(addr)
	if err != nil {
		return "", err
	}
	if err := s.locationClash(settings, loc, locationSelf{}); err != nil {
		return "", err
	}
	row, err := s.store.CreatePlaceRepo(p.ID, domain, "")
	return row.ID, err
}

// ensurePlaceRepo is the repository the place stands for as the home of the
// domain's items, made on first use. It holds placeEditMu, so an edit that
// moves the place cannot land between the address read here and the row made
// there.
func (s *Service) ensurePlaceRepo(ctx context.Context, id, domain string) (string, error) {
	s.placeEditMu.Lock()
	defer s.placeEditMu.Unlock()
	p, err := s.store.GetPlace(id)
	if err != nil {
		return "", err
	}
	repoID, _, err := s.placeRepoFor(ctx, p, domain, true)
	return repoID, err
}

// handleEnsurePlaceRepo serves POST /api/places/{id}/repo. An empty repoId is
// the domain path.
func (h *Handler) handleEnsurePlaceRepo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Domain string `json:"domain"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	id, err := h.svc.ensurePlaceRepo(r.Context(), r.PathValue("id"), body.Domain)
	switch {
	case errors.Is(err, store.ErrPlaceNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such place"})
	case err != nil:
		placeFail(w, err)
	default:
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"repoId": id}))
	}
}
