package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// placeOfItem is one place that holds backups of an item: its home ("local")
// or one of its section's off-site targets.
type placeOfItem struct {
	Place      string
	Label      string
	AppendOnly bool
	Unreadable bool
	target     store.OffsiteTarget
	repo       string
	mode       restic.Mode
	snaps      []restic.Snapshot
}

// placeResult is what removing an item's backups did at one place.
type placeResult struct {
	Place   string `json:"place"`
	Label   string `json:"label"`
	Deleted int    `json:"deleted"`
	Kept    string `json:"kept,omitempty"` // "append-only" when the place may not be deleted from
	Error   string `json:"error,omitempty"`
}

// everywhereChanged is a removal refused because the places changed since the
// window listed them; it carries the fresh list.
type everywhereChanged struct {
	name   string
	places []placeOfItem
}

var errPlacesChanged = errors.New("the places that hold backups of this entry changed; check the list again")

func (e *everywhereChanged) Error() string { return errPlacesChanged.Error() }
func (e *everywhereChanged) Unwrap() error { return errPlacesChanged }

// placesOfItem lists every place that holds backups of the item: the home
// first, then each off-site target of its section. A target that is the home's
// own repository is the home and is not listed twice.
func (s *Service) placesOfItem(ctx context.Context, settings store.Settings, item store.ItemRef, identity string) ([]placeOfItem, error) {
	home, err := s.homeOfItem(ctx, settings, item, identity)
	if err != nil {
		return nil, err
	}
	targets, err := s.store.OffsiteTargetsForDomain(item.Domain)
	if err != nil {
		return nil, err
	}
	var places []placeOfItem
	if home.Unreadable || len(home.snaps) > 0 {
		places = append(places, home)
	}
	for _, t := range targets {
		p := placeOfItem{Place: offsiteSourcePrefix + t.ID, Label: placementTargetName(t), AppendOnly: t.Immutable, target: t}
		if p.repo, err = s.resolveRepo(t.Repo); err != nil {
			p.Unreadable = true
			places = append(places, p)
			continue
		}
		if p.repo == home.repo {
			continue
		}
		p.mode = s.offsiteModeForTarget(settings, t)
		if p.snaps, err = s.ownedIn(ctx, item.Domain, identity, p.repo, p.mode, ownedByItem); err != nil {
			p.Unreadable = true
		}
		if p.Unreadable || len(p.snaps) > 0 {
			places = append(places, p)
		}
	}
	return places, nil
}

// homeOfItem reads the item's home, the repository its next backup goes to.
func (s *Service) homeOfItem(ctx context.Context, settings store.Settings, item store.ItemRef, identity string) (placeOfItem, error) {
	state, err := s.store.ItemHome(item)
	if err != nil {
		return placeOfItem{}, err
	}
	placement, err := s.readPlacement(settings, item.Domain)
	if err != nil {
		return placeOfItem{}, err
	}
	repoID, _ := placement.effectiveHome(state)
	home := placeOfItem{Place: "local"}
	if repoID != "" {
		if named, nErr := s.store.GetNamedRepo(repoID); nErr == nil {
			home.Label = named.Name
		}
	}
	home.repo, err = s.itemRepoPath(repoID, func() (string, error) { return s.repoFor(settings, item.Domain, "local") })
	if err != nil {
		home.Unreadable = true
		return home, nil
	}
	home.mode = s.primaryModeFor(settings, item.Domain, home.repo)
	home.AppendOnly = s.primaryAppendOnly(item.Domain, home.repo) != appendOnlyNone
	if home.snaps, err = s.ownedIn(ctx, item.Domain, identity, home.repo, home.mode, ownedByItem); err != nil {
		home.Unreadable = true
	}
	return home, nil
}

func (s *Service) everywhereRemovalPreview(ctx context.Context, item store.ItemRef) (string, []placeOfItem, error) {
	identity, err := s.itemIdentity(item)
	if err != nil {
		return "", nil, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", nil, fmt.Errorf("read settings: %w", err)
	}
	places, err := s.placesOfItem(ctx, settings, item, identity)
	_, name, _ := strings.Cut(identity, ":")
	return name, places, err
}

// removeEverywhere deletes an item's backups at every place the window showed
// once its name was typed. The places are read again inside the section's lock,
// and one the window did not show refuses the whole removal. Off-site copies go
// before the home, so a failure part way leaves the copy closest at hand. The
// entry and its settings stay. Its run history goes once no backup is left.
func (s *Service) removeEverywhere(ctx context.Context, item store.ItemRef, typedName string, confirmed []string) ([]placeResult, error) {
	identity, err := s.itemIdentity(item)
	if err != nil {
		return nil, err
	}
	_, name, _ := strings.Cut(identity, ":")
	if strings.TrimSpace(typedName) != name {
		return nil, errNameMismatch
	}
	unlock, ok := s.tryLockDomainFor(item.Domain, "delete")
	if !ok {
		return nil, s.busyErr(item.Domain, errDomainBusy)
	}
	defer unlock()
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	places, err := s.placesOfItem(ctx, settings, item, identity)
	if err != nil {
		return nil, err
	}
	for _, p := range places {
		if !slices.Contains(confirmed, p.Place) {
			return nil, &everywhereChanged{name: name, places: places}
		}
	}
	if len(places) > 0 && places[0].Place == "local" {
		home := places[0]
		places = append(slices.Clone(places[1:]), home)
	}
	results := make([]placeResult, 0, len(places))
	emptied := true
	for _, p := range places {
		r := placeResult{Place: p.Place, Label: p.Label}
		switch {
		case p.Unreadable:
			r.Error = "could not be read, so nothing was deleted there"
		case p.AppendOnly:
			r.Kept = "append-only"
		default:
			if err := s.forgetAtPlace(ctx, item.Domain, identity, p); err != nil {
				r.Error = scrubError(err)
				break
			}
			r.Deleted = len(p.snaps)
		}
		if r.Deleted != len(p.snaps) || p.Unreadable {
			emptied = false
		}
		results = append(results, r)
	}
	if emptied {
		s.forgetHistoryOf(ctx, item)
	}
	return results, nil
}

// forgetAtPlace deletes the item's snapshots at one place and prunes there.
func (s *Service) forgetAtPlace(ctx context.Context, domain, identity string, p placeOfItem) error {
	ids := make([]string, 0, len(p.snaps))
	for _, snap := range p.snaps {
		ids = append(ids, snap.ID)
	}
	forget := func() error { return s.engine.Forget(ctx, p.repo, ids, false, p.mode) }
	if err := s.retryAfterUnlock(ctx, p.repo, p.mode, forget); err != nil {
		return fmt.Errorf("forget snapshots: %w", err)
	}
	s.pruneAfterDelete(domain, p.repo, p.mode)
	if p.target.ID != "" {
		gone := []store.ItemCopies{{Domain: domain, Identity: identity, TargetID: p.target.ID}}
		if err := s.store.AdjustItemCopies(domain, p.target.ID, time.Now().Unix(), gone); err != nil {
			log.Printf("api: delete everywhere: observed copies of %q stay until the next listing: %v", identity, err) //nolint:gosec // G706: identity is %q-quoted
		}
	}
	return nil
}

// forgetHistoryOf drops the runs of an item whose backups are all gone, and a
// VM's block checkpoints, which a next backup could otherwise build on.
func (s *Service) forgetHistoryOf(ctx context.Context, item store.ItemRef) {
	rowID := item.Key
	switch item.Domain {
	case "containers":
		tg, err := s.store.GetTargetByContainer(item.Key)
		if err != nil {
			return
		}
		rowID = tg.ID
	case "vms":
		vm, err := s.store.GetVMTargetByName(item.Key)
		if err != nil {
			return
		}
		rowID = vm.ID
		s.dropBlockCheckpointsIfOn(ctx, item.Key)
	}
	if err := s.store.ForgetBackupHistory(rowID); err != nil {
		log.Printf("api: delete everywhere: the run history of %q stays: %v", item.Key, err) //nolint:gosec // G706: the key is %q-quoted
	}
}

func placesJSON(name string, places []placeOfItem) map[string]any {
	rows := make([]map[string]any, 0, len(places))
	for _, p := range places {
		row := map[string]any{"place": p.Place, "label": p.Label, "appendOnly": p.AppendOnly, "unreadable": p.Unreadable, "count": nil}
		if !p.Unreadable {
			row["count"] = len(p.snaps)
		}
		rows = append(rows, row)
	}
	return map[string]any{"name": name, "places": rows}
}

// handleEverywhereRemovalPreview serves GET /api/items/{domain}/{name}/removal.
func (h *Handler) handleEverywhereRemovalPreview(w http.ResponseWriter, r *http.Request) {
	domain, key, ok := h.itemParam(w, r, store.PlacementDomains...)
	if !ok {
		return
	}
	name, places, err := h.svc.everywhereRemovalPreview(r.Context(), store.ItemRef{Domain: domain, Key: key})
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(placesJSON(name, places)))
}

// handleEverywhereRemoval serves DELETE on the same path.
func (h *Handler) handleEverywhereRemoval(w http.ResponseWriter, r *http.Request) {
	domain, key, ok := h.itemParam(w, r, store.PlacementDomains...)
	if !ok || !h.requireSignIn(w, r, "deleting backups") {
		return
	}
	var body struct {
		TypedName string   `json:"typedName"`
		Places    []string `json:"places"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, done := h.svc.outlastRequest(r.Context())
	defer done()
	results, err := h.svc.removeEverywhere(ctx, store.ItemRef{Domain: domain, Key: key}, body.TypedName, body.Places)
	if err != nil {
		var extra map[string]any
		if changed, ok := errors.AsType[*everywhereChanged](err); ok {
			extra = map[string]any{"preview": placesJSON(changed.name, changed.places)}
		}
		placementFail(w, err, extra)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"results": results}))
}
