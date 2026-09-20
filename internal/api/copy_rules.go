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

// applyPlacement validates and writes the home and copies of one item. It
// writes its own refusal and reports whether the request may go on.
func (h *Handler) applyPlacement(w http.ResponseWriter, r *http.Request, item store.ItemRef, change placementChange) (placementResult, bool) {
	res, err := h.svc.writeItemPlacement(r.Context(), item, change)
	if err != nil {
		placementFail(w, err, nil)
		return placementResult{}, false
	}
	return res, true
}

// writeItemPlacement writes an item's home and copies in one transaction. A
// home change holds the domain lock, so it cannot land while the item's first
// backup settles its location.
func (s *Service) writeItemPlacement(ctx context.Context, item store.ItemRef, change placementChange) (placementResult, error) {
	res := placementResult{Dropped: []droppedTarget{}}
	home, copies, err := placementWrites(change)
	if err != nil || (home == nil && copies == nil) {
		return res, err
	}
	if home != nil {
		unlock, ok := s.tryLockDomainFor(item.Domain, placementLockReason)
		if !ok {
			return res, errPlacementBusy
		}
		defer unlock()
	}
	s.placementMu.Lock()
	defer s.placementMu.Unlock()

	settings, err := s.store.GetSettings()
	if err != nil {
		return res, err
	}
	p, err := s.readPlacement(settings, item.Domain)
	if err != nil {
		return res, err
	}
	named, err := s.namedRepoIndex()
	if err != nil {
		return res, err
	}
	read, err := s.store.ItemHome(item)
	if err != nil {
		return res, err
	}
	next := read
	if home != nil {
		if err := s.checkHomeChange(ctx, item, p, read, *home); err != nil {
			return res, err
		}
		next = store.HomeState{Exists: true, Repo: home.Repo, Choice: home.Choice}
	}
	// The copy side is judged by where an item's backups actually land, not by
	// its row's raw repo field: an open item with a default goes to the
	// default's repository, and that is the repository its copy rule has to
	// answer to.
	beforeRepo, _ := p.effectiveHome(read)
	afterRepo, _ := p.effectiveHome(next)
	if copies != nil {
		if err := s.checkCopies(settings, p, named, afterRepo, *copies); err != nil {
			return res, err
		}
	}
	identity, err := s.itemIdentity(item)
	if err != nil {
		return res, err
	}
	before := s.itemCopyTargets(settings, p, named, beforeRepo, identity)
	after := s.itemCopyTargets(settings, p.withCopies(identity, copies), named, afterRepo, identity)
	if res.Dropped, err = s.droppedTargets(item.Domain, identity, before, after); err != nil {
		return res, err
	}
	if _, err := s.store.WritePlacement(item, home, copies, nil); err != nil {
		return res, err
	}
	for _, d := range res.Dropped {
		if d.Copies == nil {
			s.listTargetInBackground(item.Domain, d.TargetID)
		}
	}
	return res, nil
}

// placementWrites turns the request fields into store writes. Each field
// takes follow or one value, never both and never neither.
func placementWrites(change placementChange) (*store.HomeWrite, *store.CopiesWrite, error) {
	var home *store.HomeWrite
	if c := change.Home; c != nil {
		switch {
		case c.Follow && c.Repo == nil:
			home = &store.HomeWrite{Choice: store.RepoOpen}
		case !c.Follow && c.Repo != nil:
			home = &store.HomeWrite{Repo: strings.TrimSpace(*c.Repo), Choice: store.RepoChosen}
		default:
			return nil, nil, errInvalidPlacement
		}
	}
	var copies *store.CopiesWrite
	if c := change.Copies; c != nil {
		switch {
		case c.Follow && c.Skip == nil:
			copies = &store.CopiesWrite{Follow: true}
		case !c.Follow && c.Skip != nil && validSkip(*c.Skip):
			copies = &store.CopiesWrite{Skip: *c.Skip}
		default:
			return nil, nil, errInvalidPlacement
		}
	}
	return home, copies, nil
}

// validSkip is what store.ValidSkipList accepts: [], ["*"] or a list of
// target ids.
func validSkip(skip []string) bool {
	return store.ValidSkipList(skip) == nil
}

// checkHomeChange refuses a new location for an item with history where it
// is, and a repository that could not take its next backup. "Where it is"
// means the row's effective home, the repository its next backup actually
// lands on: an open item with a default is already there in every way that
// matters, whatever its own (empty) repo field reads.
func (s *Service) checkHomeChange(ctx context.Context, item store.ItemRef, p placementRead, read store.HomeState, home store.HomeWrite) error {
	if home.Choice == store.RepoChosen {
		if before, _ := p.effectiveHome(read); before == home.Repo {
			return nil
		}
		if err := s.validateItemRepoID(home.Repo); err != nil {
			return fmt.Errorf("%w: %w", errRepoInvalid, err)
		}
	}
	had, err := countsAsBackedUp(s.itemBackups(ctx, item))
	if err != nil {
		return err
	}
	if had {
		return errHomeHasBackups
	}
	return nil
}

// withLegacyRepo folds the older repo field into home; both together are
// refused.
func withLegacyRepo(change placementChange, repo *string) (placementChange, error) {
	if repo == nil {
		return change, nil
	}
	if change.Home != nil {
		return change, errInvalidPlacement
	}
	change.Home = &homeChoice{Repo: repo}
	return change, nil
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
// is no copy source goes nowhere either way. repoChanging is whether the same
// request also names a new repository: an id that names no row is then left
// for that repository change to refuse, rather than judged here as a home
// that takes no copies.
func (s *Service) copiesChange(settings store.Settings, p placementRead, named map[string]store.OffsiteTarget, repoID, identity string, skip []string, repoChanging bool) (before, after []store.OffsiteTarget, err error) {
	kind := s.homeKindOf(settings, p.Domain, repoID, named)
	if repoChanging && kind == homeMissing {
		return nil, nil, nil
	}
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

// checkSkip is validSkip plus the rule that every id names a target of the
// domain, switched on or off.
func checkSkip(p placementRead, skip []string) error {
	if !validSkip(skip) {
		return errInvalidPlacement
	}
	for _, id := range skip {
		if id != store.SkipAll && !containsTarget(p.Targets, id) {
			return errNotATarget
		}
	}
	return nil
}

// checkCopies refuses a copy rule the item's home cannot carry: restic copy
// has only the target's own credentials, so a remote or direct repository
// gets none.
func (s *Service) checkCopies(settings store.Settings, p placementRead, named map[string]store.OffsiteTarget, repoID string, copies store.CopiesWrite) error {
	if copies.Follow {
		return nil
	}
	if err := checkSkip(p, copies.Skip); err != nil {
		return err
	}
	if !s.homeKindOf(settings, p.Domain, repoID, named).copySource() && !skipsEverything(copies.Skip) {
		return errCopiesNotAllowed
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
	before, after, err := s.copiesChange(settings, p, named, repoID, identity, skip, false)
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
