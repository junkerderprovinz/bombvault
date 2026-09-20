package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// placementChange is the home and copies fields of an item PATCH, of the item
// preview and, for copies, of the file set POST.
type placementChange struct {
	Home   *homeChoice   `json:"home"`
	Copies *copiesChoice `json:"copies"`
}

type homeChoice struct {
	Follow bool    `json:"follow"`
	Repo   *string `json:"repo"`
}

type copiesChoice struct {
	Follow bool      `json:"follow"`
	Skip   *[]string `json:"skip"`
}

// placementResult is what an applied change tells the card besides ok.
type placementResult struct {
	Dropped []droppedTarget `json:"dropped"`
}

type droppedTarget struct {
	TargetID   string `json:"targetId"`
	Name       string `json:"name"`
	Copies     *int   `json:"copies"` // observed copies that stay there; null while the first listing runs
	AppendOnly bool   `json:"appendOnly"`
}

type uploadEstimate struct {
	TargetID    string   `json:"targetId"`
	Name        string   `json:"name"`
	Snapshots   int      `json:"snapshots"`   // about this many at the next run
	Uncheckable []string `json:"uncheckable"` // copy sources that could not be listed
}

// pendingCopies is a copies change validateItemCopies approved but has not yet
// written: an item PATCH applies its other fields first and commits this last,
// via commitPlacement, so a later field's failure never leaves the rule behind
// and a rename in the same request resolves the identity under the name it
// leaves things under.
type pendingCopies struct {
	placementResult
	item store.ItemRef
	skip []string // nil for follow; meaningless unless set
	set  bool
}

// applyPlacement validates a copies change against the item's home: repoOverride
// when the same request also moves the item, so the skip is judged against
// where it is going rather than where it has been; the item's stored repo when
// repoOverride is nil. It writes its own refusal and reports whether the
// request may go on. The change itself is written later, by commitPlacement.
func (h *Handler) applyPlacement(w http.ResponseWriter, item store.ItemRef, copies *copiesChoice, repoOverride *string) (pendingCopies, bool) {
	p, err := h.svc.validateItemCopies(item, copies, repoOverride)
	if err != nil {
		placementFail(w, err, nil)
		return pendingCopies{}, false
	}
	return p, true
}

// commitPlacement writes a copies change applyPlacement approved and names the
// targets it stops going to. A target never listed for the domain is listed in
// the background, so the card can say how many copies stay there. It writes
// its own refusal and reports whether the request succeeded.
func (h *Handler) commitPlacement(w http.ResponseWriter, p pendingCopies) bool {
	if err := h.svc.writeItemCopies(p); err != nil {
		placementFail(w, err, nil)
		return false
	}
	return true
}

// validateItemCopies checks a copies change without writing it.
func (s *Service) validateItemCopies(item store.ItemRef, copies *copiesChoice, repoOverride *string) (pendingCopies, error) {
	p := pendingCopies{item: item, placementResult: placementResult{Dropped: []droppedTarget{}}}
	if copies == nil {
		return p, nil
	}
	skip, err := copiesSkip(*copies)
	if err != nil {
		return p, err
	}
	p.set, p.skip = true, skip

	s.placementMu.Lock()
	defer s.placementMu.Unlock()
	settings, err := s.store.GetSettings()
	if err != nil {
		return p, err
	}
	placement, err := s.readPlacement(settings, item.Domain)
	if err != nil {
		return p, err
	}
	named, err := s.namedRepoIndex()
	if err != nil {
		return p, err
	}
	identity, err := s.itemIdentity(item)
	if err != nil {
		return p, err
	}
	repoID, err := s.itemRepoForPlacement(item, repoOverride)
	if err != nil {
		return p, err
	}
	before, after, err := s.copiesChange(settings, placement, named, repoID, identity, skip)
	if err != nil {
		return p, err
	}
	if p.Dropped, err = s.droppedTargets(item.Domain, identity, before, after); err != nil {
		return p, err
	}
	return p, nil
}

// itemRepoForPlacement is the repository a copies change is judged against:
// repoOverride when the request names one, the item's stored repo otherwise.
func (s *Service) itemRepoForPlacement(item store.ItemRef, repoOverride *string) (string, error) {
	if repoOverride != nil {
		return strings.TrimSpace(*repoOverride), nil
	}
	return s.currentItemRepo(item)
}

// writeItemCopies writes a copies change validateItemCopies approved. The
// identity is resolved fresh, after the rest of the request's fields have
// already been applied, so a rename earlier in the same request carries the
// rule to the name it leaves things under.
func (s *Service) writeItemCopies(p pendingCopies) error {
	if !p.set {
		return nil
	}
	s.placementMu.Lock()
	defer s.placementMu.Unlock()
	identity, err := s.itemIdentity(p.item)
	if err != nil {
		return err
	}
	if p.skip == nil {
		err = s.store.DeleteCopyRule(p.item.Domain, identity)
	} else {
		err = s.store.SetCopyRule(p.item.Domain, identity, p.skip)
	}
	if err != nil {
		return err
	}
	for _, d := range p.Dropped {
		if d.Copies == nil {
			s.listTargetInBackground(p.item.Domain, d.TargetID)
		}
	}
	return nil
}

// copiesSkip is the skip a copies field asks for, nil for follow. Follow and a
// skip together, or neither, is refused.
func copiesSkip(c copiesChoice) ([]string, error) {
	switch {
	case c.Follow && c.Skip == nil:
		return nil, nil
	case !c.Follow && c.Skip != nil:
		return *c.Skip, nil
	}
	return nil, errInvalidPlacement
}

// currentItemRepo is the repository id an item's row names, "" for the domain
// path and for a container or VM that has no row yet.
func (s *Service) currentItemRepo(item store.ItemRef) (string, error) {
	switch item.Domain {
	case "containers":
		t, err := s.store.GetTargetByContainer(item.Key)
		return repoOfRow(t.Repo, err)
	case "vms":
		v, err := s.store.GetVMTargetByName(item.Key)
		return repoOfRow(v.Repo, err)
	case "files":
		fs, err := s.store.GetFileSet(item.Key)
		if errors.Is(err, sql.ErrNoRows) {
			return "", errFileSetNotFound
		}
		return strings.TrimSpace(fs.Repo), err
	}
	return "", fmt.Errorf("%q has no placement", item.Domain)
}

// repoOfRow reads a missing container or VM row as the domain path: it has no
// repository of its own yet.
func repoOfRow(repo string, err error) (string, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return strings.TrimSpace(repo), err
}

// copiesChange checks a skip against the item's home and returns the targets the
// item is copied to before and after it; skip nil is follow. An item whose home
// is no copy source goes nowhere either way.
func (s *Service) copiesChange(settings store.Settings, p placementRead, named map[string]store.OffsiteTarget, repoID, identity string, skip []string) (before, after []store.OffsiteTarget, err error) {
	kind := s.homeKindOf(settings, p.Domain, repoID, named)
	if skip != nil {
		if err := checkSkip(p, skip); err != nil {
			return nil, nil, err
		}
		if !kind.copySource() && !skipsEverything(skip) {
			return nil, nil, errCopiesNotAllowed
		}
	}
	if !kind.copySource() {
		return nil, nil, nil
	}
	after = p.targetsFor(p.defaultSkip())
	if skip != nil {
		after = p.targetsFor(skip)
	}
	return p.effectiveTargets(identity), after, nil
}

// checkSkip is the store's rule for what a skip may hold, plus the rule that
// every id names a target of the domain, switched on or off.
func checkSkip(p placementRead, skip []string) error {
	if err := store.ValidSkipList(skip); err != nil {
		return errInvalidPlacement
	}
	for _, id := range skip {
		if id != store.SkipAll && !containsTarget(p.Targets, id) {
			return errNotATarget
		}
	}
	return nil
}

// droppedTargets lists the targets in before that after leaves out, with the
// copies they keep. Copies stays nil for a target never listed for the domain.
func (s *Service) droppedTargets(domain, identity string, before, after []store.OffsiteTarget) ([]droppedTarget, error) {
	out := []droppedTarget{}
	observed, err := s.store.ItemCopiesFor(domain, identity)
	if err != nil {
		return nil, err
	}
	for _, t := range before {
		if containsTarget(after, t.ID) {
			continue
		}
		d := droppedTarget{TargetID: t.ID, Name: placementTargetName(t), AppendOnly: t.Immutable}
		_, listed, err := s.store.TargetObservationFor(domain, t.ID)
		if err != nil {
			return nil, err
		}
		if listed {
			n := 0
			for _, c := range observed {
				if c.TargetID == t.ID {
					n = c.SnapshotCount
				}
			}
			d.Copies = &n
		}
		out = append(out, d)
	}
	return out, nil
}

// sourceListing is one listing of a domain's copy sources with owners settled.
type sourceListing struct {
	ByIdentity map[string][]restic.Snapshot // unique by restic.Identity
	Unreadable []string                     // names of copy sources that could not be listed
}

// listCopySources lists every copy source of the domain once. A source that
// cannot be read is named rather than fatal; the estimate then says what it
// could not check.
func (s *Service) listCopySources(ctx context.Context, settings store.Settings, domain string) (sourceListing, error) {
	out := sourceListing{ByIdentity: map[string][]restic.Snapshot{}, Unreadable: []string{}}
	owners, err := s.ownerContextFor(domain)
	if err != nil {
		return out, err
	}
	sources, skipped := s.offsiteReplicationSources(settings, domain)
	for _, sk := range unreachableSkips(skipped) {
		out.Unreadable = append(out.Unreadable, sk.Name)
	}
	seen := map[string]bool{}
	for _, src := range sources {
		snaps, err := s.listSnapshots(ctx, src.Loc, s.primaryModeFor(settings, domain, src.Loc))
		if err != nil {
			out.Unreadable = append(out.Unreadable, s.refName(src))
			continue
		}
		byID := owners.owners(snaps)
		for _, sn := range snaps {
			owner := byID[sn.ID].Owner
			key := owner + "\x00" + restic.Identity(sn)
			if owner == "" || seen[key] {
				continue
			}
			seen[key] = true
			out.ByIdentity[owner] = append(out.ByIdentity[owner], sn)
		}
	}
	return out, nil
}

// uploadEstimate is about how many snapshots of identity the next run copies to
// the target: those in the sources less what the target was seen holding.
func (l sourceListing) uploadEstimate(identity string, observed []store.ItemCopies, targetID string) int {
	n := len(l.ByIdentity[identity])
	for _, c := range observed {
		if c.TargetID == targetID && c.Identity == identity {
			n -= c.SnapshotCount
		}
	}
	return max(n, 0)
}

// previewPlacement answers what a copies change would do without writing it:
// the targets the item gains, with an estimate of the upload, and the targets it
// loses, with the copies they keep.
func (s *Service) previewPlacement(ctx context.Context, item store.ItemRef, change placementChange) ([]uploadEstimate, []droppedTarget, error) {
	added, dropped := []uploadEstimate{}, []droppedTarget{}
	if change.Home != nil {
		return added, dropped, errInvalidPlacement
	}
	if change.Copies == nil {
		return added, dropped, nil
	}
	skip, err := copiesSkip(*change.Copies)
	if err != nil {
		return added, dropped, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return added, dropped, err
	}
	p, err := s.readPlacement(settings, item.Domain)
	if err != nil {
		return added, dropped, err
	}
	named, err := s.namedRepoIndex()
	if err != nil {
		return added, dropped, err
	}
	identity, err := s.itemIdentity(item)
	if err != nil {
		return added, dropped, err
	}
	repoID, err := s.currentItemRepo(item)
	if err != nil {
		return added, dropped, err
	}
	before, after, err := s.copiesChange(settings, p, named, repoID, identity, skip)
	if err != nil {
		return added, dropped, err
	}
	if dropped, err = s.droppedTargets(item.Domain, identity, before, after); err != nil {
		return added, dropped, err
	}
	observed, err := s.store.ItemCopiesFor(item.Domain, identity)
	if err != nil {
		return added, dropped, err
	}
	var listing *sourceListing
	for _, t := range after {
		if containsTarget(before, t.ID) {
			continue
		}
		if listing == nil {
			l, err := s.listCopySources(ctx, settings, item.Domain)
			if err != nil {
				return added, dropped, err
			}
			listing = &l
		}
		added = append(added, uploadEstimate{
			TargetID:    t.ID,
			Name:        placementTargetName(t),
			Snapshots:   listing.uploadEstimate(identity, observed, t.ID),
			Uncheckable: listing.Unreadable,
		})
	}
	return added, dropped, nil
}

// handlePreviewItemPlacement answers what a change to an item's placement would
// do, without writing it. POST /api/items/{domain}/{name}/placement/preview
func (h *Handler) handlePreviewItemPlacement(w http.ResponseWriter, r *http.Request) {
	domain, key, ok := h.itemParam(w, r, store.PlacementDomains...)
	if !ok {
		return
	}
	var change placementChange
	if !decodeBody(w, r, &change) {
		return
	}
	added, dropped, err := h.svc.previewPlacement(r.Context(), store.ItemRef{Domain: domain, Key: key}, change)
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"added": added, "dropped": dropped}))
}

// handleConfirmPlacement ends a domain's placement pause: replication resumes
// on the next run. The optional body names identities to leave out for good,
// the same identities and meaning ConfirmPlacement's exclude argument takes.
// POST /api/placement/{domain}/confirm
func (h *Handler) handleConfirmPlacement(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	if !validPlacementDomain(domain) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	var body struct {
		Skip []string `json:"skip"`
	}
	if !decodeOptionalBody(w, r, &body) {
		return
	}
	if err := h.svc.confirmPlacement(domain, body.Skip); err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// moveFileSetRule carries a file set's copy rule to its new name. A file set
// renamed before its first backup keeps what it was copied to.
func (s *Service) moveFileSetRule(from, to string) error {
	s.placementMu.Lock()
	defer s.placementMu.Unlock()
	return s.store.MoveCopyRule("files", "fileset:"+from, "fileset:"+to)
}
