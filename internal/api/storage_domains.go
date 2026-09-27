package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"slices"

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

// storageDomainRows is the rows of the Domains card, one per domain. ZFS has
// a row while the domain is switched on, like its page in the sidebar.
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
		if d == zfsDomain && !settings.ZFSEnabled {
			continue
		}
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

// HomePreview is what choosing a place under "Stored in" does.
type HomePreview struct {
	Mode           string         `json:"mode"`
	PlaceID        string         `json:"placeId"`
	HomePlace      string         `json:"homePlace"` // the domain's home place as read
	HomeHasBackups bool           `json:"homeHasBackups"`
	RepoID         string         `json:"repoId,omitempty"`  // default: the repository the place stands for
	Creates        string         `json:"creates,omitempty"` // default: "direct" or "repository" when this choice makes it
	Impact         *defaultImpact `json:"impact,omitempty"`  // default: what the defaults card asked for the same change
	Backups        int            `json:"backups"`           // home-move: snapshots that stay at the old home place
}

// The three things "Stored in" can do.
const (
	homeModePlace   = "home-place" // the domain path holds no backups: the place becomes the home place
	homeModeDefault = "default"    // containers, VMs and folder sets with backups: the place becomes the default
	homeModeMove    = "home-move"  // flash and config with backups: the home place moves, the backups stay
)

// homePreviewAnswer is a preview in the ok envelope.
type homePreviewAnswer struct {
	OK bool `json:"ok"`
	HomePreview
}

// domainHomePreview decides by the domain path, which it lists: without
// backups the place becomes the home place; with backups containers, VMs and
// folder sets set their default there, flash and config move their home.
func (s *Service) domainHomePreview(ctx context.Context, domain, placeID string) (HomePreview, error) {
	pv := HomePreview{PlaceID: placeID}
	p, err := s.store.GetPlace(placeID)
	if err != nil {
		return pv, err
	}
	if !p.Enabled {
		return pv, errPlaceOff
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return pv, err
	}
	homes, err := s.store.DomainPlaces()
	if err != nil {
		return pv, err
	}
	pv.HomePlace = homes[domain]
	if pv.HomeHasBackups, err = s.domainPathHasBackups(ctx, settings, domain); err != nil {
		return pv, err
	}
	switch {
	case !pv.HomeHasBackups:
		pv.Mode = homeModePlace
		return pv, s.checkHomeAddress(settings, p, domain)
	case validPlacementDomain(domain):
		pv.Mode = homeModeDefault
		return pv, s.previewDefaultHome(ctx, settings, p, domain, &pv)
	}
	pv.Mode = homeModeMove
	if err := s.checkHomeAddress(settings, p, domain); err != nil {
		return pv, err
	}
	facts, err := s.domainPathFacts(settings, domain)
	pv.Backups = facts.Snapshots
	return pv, err
}

// domainPathHasBackups lists the domain's path: a local one only when it was
// ever created, a remote one always. A path that cannot be read keeps its
// home, as an item's home does.
func (s *Service) domainPathHasBackups(ctx context.Context, settings store.Settings, domain string) (bool, error) {
	loc, err := s.repoFor(settings, domain, "local")
	if err != nil {
		return false, err
	}
	if localRepoMissing(loc) {
		switch s.repoEstablishmentOf(loc) {
		case repoNeverEstablished:
			return false, nil
		case repoWasEstablished:
			return false, fmt.Errorf("%w: %w", errHomeUncheckable, ErrBackupPathNotMounted)
		}
		return false, fmt.Errorf("%w: whether the path was ever created could not be read", errHomeUncheckable)
	}
	snaps, err := s.listSnapshots(ctx, loc, s.primaryModeFor(settings, domain, loc))
	if err != nil {
		return false, fmt.Errorf("%w: %w", errHomeUncheckable, err)
	}
	return len(snaps) > 0, nil
}

// checkHomeAddress refuses a place as a domain's home when the domain has no
// folder there, when a target of the domain lies at that address, which would
// then be its own copy source, or when the address nests in another
// repository.
func (s *Service) checkHomeAddress(settings store.Settings, p store.Place, domain string) error {
	addr, ok := store.PlaceAddress(p, domain, "")
	if !ok {
		return store.ErrPlaceDomainUnavailable
	}
	loc, err := s.resolveRepo(addr)
	if err != nil {
		return err
	}
	targets, err := s.store.OffsiteTargetsForDomain(domain)
	if err != nil {
		return err
	}
	for _, t := range targets {
		if tLoc, ok := s.clashCandidate("off-site destination", t.Name, t.Repo); ok && sameRepoLocation(tLoc, loc) {
			return errPlaceAddressTaken
		}
	}
	return s.locationClash(settings, loc, locationSelf{own: domain})
}

// previewDefaultHome is the default half of the preview: the repository the
// place stands for and the question the defaults card asked for it.
func (s *Service) previewDefaultHome(ctx context.Context, settings store.Settings, p store.Place, domain string, pv *HomePreview) error {
	id, creates, err := s.placeRepoFor(ctx, p, domain, false)
	if err != nil {
		return err
	}
	pv.RepoID, pv.Creates = id, creates
	var impact defaultImpact
	if creates == "" {
		impact, err = s.defaultImpactFor(ctx, domain, defaultChange{Home: &id})
	} else {
		impact, err = s.newHomeImpact(ctx, settings, domain)
	}
	pv.Impact = &impact
	return err
}

// newHomeImpact is defaultImpactFor for a home this choice makes, which no
// validation can see yet: it moves no copies, and every open item without
// backups on the domain path takes it.
func (s *Service) newHomeImpact(ctx context.Context, settings store.Settings, domain string) (defaultImpact, error) {
	impact := defaultImpact{Dropped: []targetImpact{}, Added: []targetImpact{}, Skip: []string{}}
	p, err := s.readPlacement(settings, domain)
	if err != nil {
		return impact, err
	}
	impact.Home = p.State.Default.Home
	impact.Skip = append(impact.Skip, p.State.Default.Skip...)
	items, err := s.domainItems(domain)
	if err != nil {
		return impact, err
	}
	impact.OpenTakeHome, err = s.openTakers(ctx, settings, domain, items)
	return impact, err
}

// storageDomainParam reads {d} of a /api/storage/domains route and writes the
// 400 itself.
func storageDomainParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	domain := r.PathValue("d")
	if !validOffsiteDomain(domain) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return "", false
	}
	return domain, true
}

// handleDomainHomePreview serves POST /api/storage/domains/{d}/home/preview.
func (h *Handler) handleDomainHomePreview(w http.ResponseWriter, r *http.Request) {
	domain, ok := storageDomainParam(w, r)
	if !ok {
		return
	}
	var body struct {
		PlaceID string `json:"placeId"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	pv, err := h.svc.domainHomePreview(r.Context(), domain, body.PlaceID)
	switch {
	case errors.Is(err, store.ErrPlaceNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such place"})
	case err != nil:
		placeFail(w, err)
	default:
		writeJSON(w, http.StatusOK, homePreviewAnswer{OK: true, HomePreview: pv})
	}
}

// same compares what the window showed, not snapshot counts, which every
// backup changes.
func (pv HomePreview) same(o HomePreview) bool {
	if pv.Mode != o.Mode || pv.PlaceID != o.PlaceID || pv.HomePlace != o.HomePlace || pv.RepoID != o.RepoID || pv.Creates != o.Creates {
		return false
	}
	if pv.Impact == nil || o.Impact == nil {
		return pv.Impact == nil && o.Impact == nil
	}
	return pv.Impact.sameCounts(*o.Impact)
}

// domainHomeBody is "Stored in" as the window confirms it.
type domainHomeBody struct {
	PlaceID     string       `json:"placeId"`
	Expect      *HomePreview `json:"expect"`
	ApplyToOpen bool         `json:"applyToOpen"`
}

// setDomainHome writes "Stored in" once the preview reads as the window
// showed it. It holds the domain lock from the preview to the last item it
// resets, so no backup writes to the old path or takes the old default in
// between.
func (s *Service) setDomainHome(ctx context.Context, domain string, body domainHomeBody) (HomePreview, []string, []keptItem, error) {
	unlock, ok := s.tryLockDomainFor(domain, placementLockReason)
	if !ok {
		return HomePreview{}, nil, nil, errPlacementBusy
	}
	defer unlock()
	pv, err := s.writeDomainHome(ctx, domain, body)
	if err != nil || pv.Mode != homeModeDefault || !body.ApplyToOpen {
		return pv, []string{}, []keptItem{}, err
	}
	reset, kept, err := s.applyDefaultToOpen(ctx, domain)
	return pv, reset, kept, err
}

// writeDomainHome holds placeEditMu from the preview to the write, so no edit
// of the place moves the address the preview judged.
func (s *Service) writeDomainHome(ctx context.Context, domain string, body domainHomeBody) (HomePreview, error) {
	s.placeEditMu.Lock()
	defer s.placeEditMu.Unlock()
	pv, err := s.domainHomePreview(ctx, domain, body.PlaceID)
	switch {
	case err != nil:
		return pv, err
	case body.Expect == nil || !pv.same(*body.Expect):
		return pv, errPlacementStale
	case pv.Mode != homeModeDefault:
		return pv, s.moveHomePlace(domain, pv.PlaceID)
	}
	if err := s.setDefaultHome(ctx, domain, pv); !errors.Is(err, errPlacementStale) {
		return pv, err
	}
	// The defaults card writes the default without the domain lock, so it can
	// move after the preview; a stale answer carries a fresh one.
	fresh, err := s.domainHomePreview(ctx, domain, body.PlaceID)
	if err != nil {
		return fresh, err
	}
	return fresh, errPlacementStale
}

// moveHomePlace makes the place the domain's home: its path moves there, and
// for containers, VMs and folder sets the default goes back to that path.
// Backups at the old home stay where they are.
func (s *Service) moveHomePlace(domain, placeID string) error {
	p, err := s.store.GetPlace(placeID)
	if err != nil {
		return err
	}
	if _, err := s.writePlace(store.PlaceWrite{Place: p, HomeDomains: map[string]string{domain: p.ID}}, nil); err != nil {
		return err
	}
	if !validPlacementDomain(domain) {
		return nil
	}
	s.placementMu.Lock()
	defer s.placementMu.Unlock()
	d, found, err := s.store.PlacementDefaultFor(domain)
	if err != nil || !found || d.Home == "" {
		return err
	}
	_, err = s.store.PutPlacementDefault(domain, "", d.Skip)
	return err
}

// setDefaultHome points the default at the repository the place stands for,
// made on this first choice, through the defaults card's own write.
func (s *Service) setDefaultHome(ctx context.Context, domain string, pv HomePreview) error {
	p, err := s.store.GetPlace(pv.PlaceID)
	if err != nil {
		return err
	}
	id, _, err := s.placeRepoFor(ctx, p, domain, true)
	if err != nil {
		return err
	}
	_, _, err = s.putDefault(ctx, domain, defaultChange{Home: &id, Expect: pv.Impact})
	return err
}

// applyDefaultToOpen resets every item without backups to the default, what
// "Apply to entries without backups" does, under the caller's domain lock.
func (s *Service) applyDefaultToOpen(ctx context.Context, domain string) ([]string, []keptItem, error) {
	candidates, _, err := s.applyDefaultPreview(ctx, domain)
	if err != nil {
		return nil, nil, err
	}
	keys := make([]string, 0, len(candidates))
	for _, c := range candidates {
		keys = append(keys, c.Key)
	}
	return s.applyDefaultLocked(ctx, domain, keys)
}

// handleDomainHome serves PUT /api/storage/domains/{d}/home. A stale answer
// carries the fresh preview.
func (h *Handler) handleDomainHome(w http.ResponseWriter, r *http.Request) {
	domain, ok := storageDomainParam(w, r)
	if !ok {
		return
	}
	var body domainHomeBody
	if !decodeBody(w, r, &body) {
		return
	}
	pv, reset, kept, err := h.svc.setDomainHome(r.Context(), domain, body)
	switch {
	case errors.Is(err, store.ErrPlaceNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such place"})
	case errors.Is(err, errPlacementStale):
		placementFail(w, err, map[string]any{"preview": pv})
	case err != nil:
		placeFail(w, err)
	default:
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"reset": reset, "kept": kept}))
	}
}

// CopiesPreview is what switching one chip of a domain row does.
type CopiesPreview struct {
	PlaceID   string         `json:"placeId"`
	On        bool           `json:"on"`
	TargetID  string         `json:"targetId,omitempty"`  // "" while the place holds no target of the domain
	Suffix    string         `json:"suffix,omitempty"`    // the address ending of a new target
	Skip      []string       `json:"skip"`                // containers, VMs, folder sets: the default's skip after the chip
	Enabled   bool           `json:"enabled"`             // whether the target is switched on before the chip
	NewTarget *targetPreview `json:"newTarget,omitempty"` // a new target, or a stopped one switched on: what it receives at its next run
	Impact    *defaultImpact `json:"impact,omitempty"`
}

// same compares what the window showed, not snapshot counts, which every
// backup changes.
func (pv CopiesPreview) same(o CopiesPreview) bool {
	if pv.PlaceID != o.PlaceID || pv.On != o.On || pv.TargetID != o.TargetID || pv.Suffix != o.Suffix ||
		pv.Enabled != o.Enabled || !slices.Equal(pv.Skip, o.Skip) {
		return false
	}
	switch {
	case pv.NewTarget != nil && o.NewTarget != nil:
		return pv.NewTarget.Items == o.NewTarget.Items
	case pv.Impact != nil && o.Impact != nil:
		return pv.Impact.sameCounts(*o.Impact)
	}
	return pv.NewTarget == nil && o.NewTarget == nil && pv.Impact == nil && o.Impact == nil
}

// copiesPreviewAnswer is a preview in the ok envelope.
type copiesPreviewAnswer struct {
	OK bool `json:"ok"`
	CopiesPreview
}

// domainCopiesPreview works out a chip: the target it switches or creates at
// the place and, for containers, VMs and folder sets, the default's skip
// after it with the question the defaults card asked for the same change.
func (s *Service) domainCopiesPreview(ctx context.Context, domain, placeID string, on bool) (CopiesPreview, error) {
	pv := CopiesPreview{PlaceID: placeID, On: on, Skip: []string{}}
	p, err := s.store.GetPlace(placeID)
	if err != nil {
		return pv, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return pv, err
	}
	homes, err := s.store.DomainPlaces()
	if err != nil {
		return pv, err
	}
	rows, err := s.store.PlaceRows(p.ID)
	if err != nil {
		return pv, err
	}
	a := domainAt(p, rows, homes[domain], domain)
	switch {
	case a.home:
		return pv, errPlaceHomeDomain
	case on && !p.Enabled:
		return pv, errPlaceOff
	case a.target != nil:
		pv.TargetID, pv.Enabled = a.target.ID, a.target.Enabled
	case on:
		if pv.Suffix, err = s.newTargetSuffix(settings, p, rows, a, domain); err != nil {
			return pv, err
		}
	}
	if !validPlacementDomain(domain) {
		return pv, nil
	}
	return pv, s.previewCopiesSkip(ctx, settings, domain, p, &pv)
}

// newTargetSuffix is the address ending a new target of the domain takes at
// the place, none or -copies beside the domain's named repository there,
// once that address has passed the location check.
func (s *Service) newTargetSuffix(settings store.Settings, p store.Place, rows []store.OffsiteTarget, a domainAtPlace, domain string) (string, error) {
	if _, ok := p.Folders[domain]; !ok {
		return "", store.ErrPlaceDomainUnavailable
	}
	suffix := ""
	if a.repo != nil {
		if placeIsRepository(p, rows) {
			return "", errPlaceIsRepository
		}
		suffix = copiesSuffix
	}
	addr, _ := store.PlaceAddress(p, domain, suffix)
	loc, err := s.resolveRepo(addr)
	if err != nil {
		return "", err
	}
	return suffix, s.locationClash(settings, loc, locationSelf{target: true})
}

// previewCopiesSkip fills in the default's skip after the chip and what it
// changes. The default's own place takes no chip: it is where the domain
// stores.
func (s *Service) previewCopiesSkip(ctx context.Context, settings store.Settings, domain string, p store.Place, pv *CopiesPreview) error {
	read, err := s.readPlacement(settings, domain)
	if err != nil {
		return err
	}
	named, err := s.namedRepoIndex()
	if err != nil {
		return err
	}
	if repo, ok := named[read.State.Default.Home]; ok && repo.PlaceID == p.ID && repo.CompanionOf == "" {
		return errPlaceHomeDomain
	}
	if pv.TargetID == "" && !pv.On {
		pv.Skip = append(pv.Skip, read.defaultSkip()...)
		return nil
	}
	pv.Skip = chipSkip(read, pv.TargetID, pv.On)
	if pv.On && (pv.TargetID == "" || !pv.Enabled) {
		next := read.withDefaultSkip(pv.Skip)
		preview, err := s.newTargetPreview(ctx, domain, pv.TargetID, &next)
		pv.NewTarget = &preview
		return err
	}
	impact, err := s.defaultImpactFor(ctx, domain, defaultChange{Skip: &pv.Skip})
	pv.Impact = &impact
	return err
}

// chipSkip is the default's skip once a chip is switched. Under ["*"] a chip
// switched on leaves out every other enabled target instead, since not being
// in ["*"] means nothing; a chip switched off that leaves no enabled target
// in goes back to ["*"].
func chipSkip(p placementRead, targetID string, on bool) []string {
	cur := p.defaultSkip()
	enabled := p.enabledTargets()
	switch {
	case on && skipsEverything(cur):
		out := []string{}
		for _, t := range enabled {
			if t.ID != targetID {
				out = append(out, t.ID)
			}
		}
		return out
	case on:
		return slices.DeleteFunc(slices.Clone(cur), func(id string) bool { return id == targetID })
	}
	next := withTarget(cur, targetID)
	if len(enabled) > 0 && !slices.ContainsFunc(enabled, func(t store.OffsiteTarget) bool { return !skipsTarget(next, t.ID) }) {
		return []string{store.SkipAll}
	}
	return slices.Clone(next)
}

// domainCopiesBody is a chip as the window confirms it.
type domainCopiesBody struct {
	PlaceID string         `json:"placeId"`
	On      bool           `json:"on"`
	Expect  *CopiesPreview `json:"expect"`
}

// setDomainCopies writes a chip once the preview reads as the window showed
// it: the target and the default's skip in one transaction, and for flash and
// config the target's switch. A chip switched on also starts a target that was
// stopped on its own, or leaving it out of the skip would copy nothing.
// placeEditMu keeps an edit of the place from moving the address the preview
// judged, and placementMu every other write of the default out, from the
// preview to the write.
func (s *Service) setDomainCopies(ctx context.Context, domain string, body domainCopiesBody) (CopiesPreview, error) {
	s.placeEditMu.Lock()
	defer s.placeEditMu.Unlock()
	s.placementMu.Lock()
	defer s.placementMu.Unlock()
	pv, err := s.domainCopiesPreview(ctx, domain, body.PlaceID, body.On)
	if err != nil {
		return pv, err
	}
	if body.Expect == nil || !pv.same(*body.Expect) {
		return pv, errPlacementStale
	}
	if pv.TargetID == "" && !pv.On {
		return pv, nil
	}
	w := store.DomainCopiesWrite{Domain: domain, PlaceID: pv.PlaceID, TargetID: pv.TargetID, Suffix: pv.Suffix}
	if validPlacementDomain(domain) {
		w.Skip = &pv.Skip
	}
	if !validPlacementDomain(domain) || (pv.On && pv.TargetID != "" && !pv.Enabled) {
		w.Enabled = &pv.On
	}
	if _, err := s.store.WriteDomainCopies(w); err != nil {
		return pv, err
	}
	if pv.Impact != nil {
		for _, t := range pv.Impact.Dropped {
			if t.Unknown {
				s.listTargetInBackground(domain, t.TargetID)
			}
		}
	}
	return pv, nil
}

// handleDomainCopiesPreview serves POST /api/storage/domains/{d}/copies/preview.
func (h *Handler) handleDomainCopiesPreview(w http.ResponseWriter, r *http.Request) {
	domain, ok := storageDomainParam(w, r)
	if !ok {
		return
	}
	var body struct {
		PlaceID string `json:"placeId"`
		On      bool   `json:"on"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	pv, err := h.svc.domainCopiesPreview(r.Context(), domain, body.PlaceID, body.On)
	switch {
	case errors.Is(err, store.ErrPlaceNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such place"})
	case err != nil:
		placeFail(w, err)
	default:
		writeJSON(w, http.StatusOK, copiesPreviewAnswer{OK: true, CopiesPreview: pv})
	}
}

// handleDomainCopies serves PUT /api/storage/domains/{d}/copies. A stale
// answer carries the fresh preview.
func (h *Handler) handleDomainCopies(w http.ResponseWriter, r *http.Request) {
	domain, ok := storageDomainParam(w, r)
	if !ok {
		return
	}
	var body domainCopiesBody
	if !decodeBody(w, r, &body) {
		return
	}
	pv, err := h.svc.setDomainCopies(r.Context(), domain, body)
	switch {
	case errors.Is(err, store.ErrPlaceNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such place"})
	case errors.Is(err, errPlacementStale):
		placementFail(w, err, map[string]any{"preview": pv})
	case err != nil:
		placeFail(w, err)
	default:
		writeJSON(w, http.StatusOK, okEnvelope(nil))
	}
}
