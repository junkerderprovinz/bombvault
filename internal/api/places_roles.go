package api

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// copiesSuffix ends the address of a domain's target at a place that already
// holds the domain's named repository, the counterpart of directSuffix.
const copiesSuffix = "-copies"

var errRowPlaced = errors.New("that row already belongs to a place")

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

// adoptBody names the row a place takes over. An empty rowId is the domain's
// own path.
type adoptBody struct {
	RowID  string `json:"rowId"`
	Domain string `json:"domain"`
}

// adoptRow puts a row without a place at the place, at the address its role
// takes there, under the rules of an address change. It holds placeEditMu for
// the reason patchPlace does.
func (s *Service) adoptRow(ctx context.Context, placeID string, body adoptBody) error {
	s.placeEditMu.Lock()
	defer s.placeEditMu.Unlock()
	p, err := s.store.GetPlace(placeID)
	if err != nil {
		return err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	homes, err := s.store.DomainPlaces()
	if err != nil {
		return err
	}
	rows, err := s.store.PlaceRows(p.ID)
	if err != nil {
		return err
	}
	if body.RowID == "" {
		return s.adoptDomainPath(ctx, settings, p, rows, homes, body.Domain)
	}
	row, err := s.unplacedRow(body.RowID)
	if err != nil {
		return err
	}
	domain, suffix, err := s.adoptSlot(p, rows, homes, row, body.Domain)
	if err != nil {
		return err
	}
	if err := s.adoptionKeeps(settings, p, row); err != nil {
		return err
	}
	addr, ok := store.PlaceAddress(p, domain, suffix)
	if !ok {
		return store.ErrPlaceDomainUnavailable
	}
	// A direct repository keeps its own credentials at the place, so only its
	// address can change; every other row takes the place's.
	oldMode := s.rowMode(settings, row)
	mode, sameCreds := oldMode, row.CompanionOf != "" || row.CredsRef == p.CredsRef
	if !sameCreds {
		if mode, err = s.placeMode(settings, p); err != nil {
			return err
		}
	}
	locks := &domainLocks{s: s}
	defer locks.release()
	if !sameRepoLocation(addr, row.Repo) {
		if err := locks.take(domain); err != nil {
			return err
		}
	}
	// The same address with the same credentials is the same repository;
	// anything else is opened first.
	if !sameRepoLocation(addr, row.Repo) || !sameCreds {
		facts, err := s.rowFacts(row)
		if err != nil {
			return err
		}
		self, err := s.rowSelf(row)
		if err != nil {
			return err
		}
		move := addressMove{Domain: domain, Old: row.Repo, New: addr, OldMode: oldMode, Self: self, Facts: facts}
		if err := s.checkAdoption(ctx, settings, move, mode); err != nil {
			return err
		}
	}
	if _, err := s.store.AdoptRow(row.ID, p.ID, domain, suffix); err != nil {
		return err
	}
	_, err = s.writePlace(store.PlaceWrite{Place: p}, nil)
	return err
}

// checkAdoption holds the way of a row or a domain path onto the place to the
// rules of an address change: a new address may not nest with another
// repository, and it has to hold nothing or what the old one held. mode opens
// the new address.
func (s *Service) checkAdoption(ctx context.Context, settings store.Settings, m addressMove, mode restic.Mode) error {
	if !sameRepoLocation(m.New, m.Old) {
		if err := s.checkNesting(settings, []addressMove{m}); err != nil {
			return err
		}
	}
	return s.checkMoves(ctx, []addressMove{m}, mode)
}

// adoptionKeeps refuses to put a row at a place whose rules would keep fewer
// of its snapshots, since the place mirrors them onto the row. A target hands
// them on to its direct repository, which is held to the same.
func (s *Service) adoptionKeeps(settings store.Settings, p store.Place, row store.OffsiteTarget) error {
	before := rowRetentionPolicy(row)
	if row.Role == store.RoleRepo {
		before = s.retentionPolicyForRef(settings, "", domainRepoRef{Named: row})
	}
	if err := placeKeeps(p, before, row.Immutable); err != nil {
		return err
	}
	if row.Role != store.RoleOffsite {
		return nil
	}
	direct, found, err := s.store.CompanionFor(row.ID)
	if err != nil || !found {
		return err
	}
	return placeKeeps(p, rowRetentionPolicy(direct), direct.Immutable)
}

// placeKeeps refuses a repository that ages by before, or is append-only, a
// place whose rules would have its next prune forget more. An append-only
// place forgets nothing.
func placeKeeps(p store.Place, before restic.RetentionPolicy, appendOnly bool) error {
	switch {
	case p.Immutable:
		return nil
	case appendOnly:
		return errPlaceAppendOnlyOff
	case retentionLowered(before, placeRetentionPolicy(p)):
		return errPlaceKeepsLess
	}
	return nil
}

// unplacedRow is a target or named repository that belongs to no place.
func (s *Service) unplacedRow(id string) (store.OffsiteTarget, error) {
	row, found, err := s.store.GetOffsiteTarget(id)
	if err != nil {
		return row, err
	}
	if !found {
		if row, err = s.store.GetNamedRepo(id); err != nil {
			return row, fmt.Errorf("no such row: %w", err)
		}
	}
	if row.PlaceID != "" {
		return row, errRowPlaced
	}
	return row, nil
}

// adoptSlot is the domain and address ending a row takes at the place, by the
// rules a row made there follows. At a place that is itself a repository a
// named repository becomes the one every domain there shares, while the base
// serves nothing else.
func (s *Service) adoptSlot(p store.Place, rows []store.OffsiteTarget, homes map[string]string, row store.OffsiteTarget, domain string) (string, string, error) {
	switch {
	case row.Role == store.RoleOffsite:
		if domain != "" && domain != row.Domain {
			return "", "", errInvalidPlacement
		}
		a := domainAt(p, rows, homes[row.Domain], row.Domain)
		switch {
		case a.home || a.target != nil:
			return "", "", errPlaceAddressTaken
		case a.repo == nil:
			return row.Domain, "", nil
		case placeIsRepository(p, rows):
			return "", "", errPlaceIsRepository
		}
		return row.Domain, copiesSuffix, nil
	case !p.Enabled:
		// A repository there would be switched off with the place, and every
		// item backing up to it would fail.
		return "", "", errPlaceOff
	case row.CompanionOf != "":
		target, found, err := s.store.GetOffsiteTarget(row.CompanionOf)
		switch {
		case err != nil:
			return "", "", err
		case !found || target.PlaceID != p.ID:
			return "", "", errMirroredField
		case placeIsRepository(p, rows):
			return "", "", errPlaceIsRepository
		}
		return target.Domain, directSuffix, nil
	case domain != "" && !validPlacementDomain(domain):
		return "", "", errInvalidPlacement
	case placeIsRepository(p, rows):
		if _, offered := p.Folders[domain]; domain != "" && !offered {
			return "", "", store.ErrPlaceDomainUnavailable
		}
		if len(rows) > 0 || slices.Contains(slices.Collect(maps.Values(homes)), p.ID) {
			return "", "", errPlaceAddressTaken
		}
		return "", "", nil
	case domain == "":
		return "", "", store.ErrPlaceDomainUnavailable
	}
	if a := domainAt(p, rows, homes[domain], domain); a.home || a.target != nil || a.repo != nil {
		return "", "", errPlaceAddressTaken
	}
	return domain, "", nil
}

// adoptDomainPath makes the place the home of a domain whose path has no
// place. A home place stays on, so a switched-off place takes none. A remote
// path is opened with the place's credentials first, even at the same address.
func (s *Service) adoptDomainPath(ctx context.Context, settings store.Settings, p store.Place, rows []store.OffsiteTarget, homes map[string]string, domain string) error {
	if !validOffsiteDomain(domain) {
		return errInvalidPlacement
	}
	if homes[domain] != "" {
		return errRowPlaced
	}
	if !p.Enabled {
		return errPlaceOff
	}
	addr, ok := store.PlaceAddress(p, domain, "")
	if !ok {
		return store.ErrPlaceDomainUnavailable
	}
	if domainAt(p, rows, "", domain).target != nil {
		return errPlaceAddressTaken
	}
	old := domainPathRaw(domain, settings)
	loc, err := s.resolveRepo(old)
	if err != nil {
		return err
	}
	before := s.retentionPolicyForRef(settings, domain, domainRepoRef{Own: true})
	if err := placeKeeps(p, before, s.primaryIsImmutable(domain, loc)); err != nil {
		return err
	}
	locks := &domainLocks{s: s}
	defer locks.release()
	if !sameRepoLocation(addr, old) {
		if err := locks.take(domain); err != nil {
			return err
		}
	}
	if !sameRepoLocation(addr, old) || restic.IsRemoteRepo(loc) {
		facts, err := s.domainPathFacts(settings, domain)
		if err != nil {
			return err
		}
		mode, err := s.placeMode(settings, p)
		if err != nil {
			return err
		}
		move := addressMove{Domain: domain, Old: old, New: addr, OldMode: s.primaryModeFor(settings, domain, loc), Self: locationSelf{own: domain}, Facts: facts}
		if err := s.checkAdoption(ctx, settings, move, mode); err != nil {
			return err
		}
	}
	_, err = s.writePlace(store.PlaceWrite{Place: p, HomeDomains: map[string]string{domain: p.ID}}, nil)
	return err
}

// handleAdoptRow serves POST /api/places/{id}/adopt.
func (h *Handler) handleAdoptRow(w http.ResponseWriter, r *http.Request) {
	var body adoptBody
	if !decodeBody(w, r, &body) {
		return
	}
	id := r.PathValue("id")
	err := h.svc.adoptRow(r.Context(), id, body)
	switch {
	case errors.Is(err, store.ErrPlaceNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such place"})
	case err != nil:
		placeFail(w, err)
	default:
		h.writePlaceAnswer(w, id, nil)
	}
}
