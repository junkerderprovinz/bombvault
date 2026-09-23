package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"slices"
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
		if err := s.checkHomeChange(ctx, item, read, *home); err != nil {
			return res, err
		}
		next = store.HomeState{Exists: true, Repo: home.Repo, Choice: home.Choice}
	}
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
	ok, err := s.store.WritePlacement(item, home, copies, &read)
	if err != nil {
		return res, err
	}
	if !ok {
		return res, errPlacementStale
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
// means the row's own repo field, the same place itemBackups looks: an open
// item's history, if it has any, sits at the domain path, whatever its
// default currently names.
func (s *Service) checkHomeChange(ctx context.Context, item store.ItemRef, read store.HomeState, home store.HomeWrite) error {
	if home.Choice == store.RepoChosen {
		if read.Repo == home.Repo {
			return nil
		}
		if err := s.validateItemRepoID(item.Domain, home.Repo); err != nil {
			return fmt.Errorf("%w: %w", errRepoInvalid, err)
		}
		// The new repository may already hold fileset:<name> snapshots of an
		// unrelated folder, which this set would adopt on its next backup.
		if item.Domain == "files" {
			set, err := s.store.GetFileSet(item.Key)
			if err != nil {
				return err
			}
			if err := s.fileSetNameAdoptable(ctx, set.Name, home.Repo, set.Path); err != nil {
				return err
			}
		}
	}
	presence, err := s.itemBackups(ctx, item)
	switch {
	case presence == backupsUnreadable:
		return fmt.Errorf("%w: %w", errHomeUncheckable, err)
	case err != nil:
		return err
	case presence == backupsPresent:
		return errHomeHasBackups
	}
	return nil
}

// checkPlacementChange previews a home or copies change against the current
// placement without writing it, including a probe of the domain lock a home
// change needs, so a handler can refuse an invalid or busy one before any of
// its other fields land. writeItemPlacement runs the same checks again under
// lock right before the write, which stays authoritative.
func (s *Service) checkPlacementChange(ctx context.Context, item store.ItemRef, change placementChange) error {
	home, copies, err := placementWrites(change)
	if err != nil || (home == nil && copies == nil) {
		return err
	}
	if home != nil {
		if _, busy := s.domainBusy(item.Domain); busy {
			return errPlacementBusy
		}
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	p, err := s.readPlacement(settings, item.Domain)
	if err != nil {
		return err
	}
	read, err := s.store.ItemHome(item)
	if err != nil {
		return err
	}
	next := read
	if home != nil {
		if err := s.checkHomeChange(ctx, item, read, *home); err != nil {
			return err
		}
		next = store.HomeState{Exists: true, Repo: home.Repo, Choice: home.Choice}
	}
	if copies == nil {
		return nil
	}
	named, err := s.namedRepoIndex()
	if err != nil {
		return err
	}
	afterRepo, _ := p.effectiveHome(next)
	return s.checkCopies(settings, p, named, afterRepo, *copies)
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

// previewPlacement answers what a change would do without writing it: the
// targets that would get this item's history, at about how many snapshots, and
// the targets that would stop getting it, with the copies they keep.
func (s *Service) previewPlacement(ctx context.Context, item store.ItemRef, change placementChange) ([]uploadEstimate, []droppedTarget, error) {
	home, copies, err := placementWrites(change)
	if err != nil {
		return nil, nil, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, nil, err
	}
	p, err := s.readPlacement(settings, item.Domain)
	if err != nil {
		return nil, nil, err
	}
	named, err := s.namedRepoIndex()
	if err != nil {
		return nil, nil, err
	}
	read, err := s.store.ItemHome(item)
	if err != nil {
		return nil, nil, err
	}
	next := read
	if home != nil {
		next = store.HomeState{Exists: true, Repo: home.Repo, Choice: home.Choice}
	}
	beforeRepo, _ := p.effectiveHome(read)
	afterRepo, _ := p.effectiveHome(next)
	if copies != nil {
		if err := s.checkCopies(settings, p, named, afterRepo, *copies); err != nil {
			return nil, nil, err
		}
	}
	identity, err := s.itemIdentity(item)
	if err != nil {
		return nil, nil, err
	}
	before := s.itemCopyTargets(settings, p, named, beforeRepo, identity)
	after := s.itemCopyTargets(settings, p.withCopies(identity, copies), named, afterRepo, identity)
	dropped, err := s.droppedTargets(item.Domain, identity, before, after)
	if err != nil {
		return nil, nil, err
	}
	added := []uploadEstimate{}
	var gained []store.OffsiteTarget
	for _, t := range after {
		if !containsTarget(before, t.ID) {
			gained = append(gained, t)
		}
	}
	if len(gained) == 0 {
		return added, dropped, nil
	}
	listing, err := s.listCopySources(ctx, settings, item.Domain)
	if err != nil {
		return nil, nil, err
	}
	observed, err := s.store.ItemCopiesFor(item.Domain, identity)
	if err != nil {
		return nil, nil, err
	}
	for _, t := range gained {
		added = append(added, uploadEstimate{
			TargetID:    t.ID,
			Name:        placementTargetName(t),
			Snapshots:   listing.uploadEstimate(identity, observed, t.ID),
			Uncheckable: append([]string{}, listing.Unreadable...),
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

// moveFileSetRule carries a file set's copy rule to its new name. A file set
// renamed before its first backup keeps what it was copied to.
func (s *Service) moveFileSetRule(from, to string) error {
	s.placementMu.Lock()
	defer s.placementMu.Unlock()
	return s.store.MoveCopyRule("files", "fileset:"+from, "fileset:"+to)
}

// createFileSet creates a set and the copy rule it was created with. The rule
// hangs on the set's name, so it follows the row, and the row goes again when
// the rule cannot be written.
func (s *Service) createFileSet(fs store.FileSet, choice *copiesChoice) (store.FileSet, error) {
	if err := s.validateItemRepoID("files", fs.Repo); err != nil {
		return store.FileSet{}, fmt.Errorf("%w: %w", errRepoInvalid, err)
	}
	var copies *store.CopiesWrite
	if choice != nil {
		var err error
		if _, copies, err = placementWrites(placementChange{Copies: choice}); err != nil {
			return store.FileSet{}, err
		}
		s.placementMu.Lock()
		defer s.placementMu.Unlock()
		settings, err := s.store.GetSettings()
		if err != nil {
			return store.FileSet{}, err
		}
		p, err := s.readPlacement(settings, "files")
		if err != nil {
			return store.FileSet{}, err
		}
		named, err := s.namedRepoIndex()
		if err != nil {
			return store.FileSet{}, err
		}
		repoID, _ := p.effectiveHome(store.HomeState{Repo: fs.Repo, Choice: fs.RepoChosen})
		if err := s.checkCopies(settings, p, named, repoID, *copies); err != nil {
			return store.FileSet{}, err
		}
	}
	created, err := s.store.CreateFileSet(fs)
	if err != nil || copies == nil {
		return created, err
	}
	if _, err := s.store.WritePlacement(store.ItemRef{Domain: "files", Key: created.ID}, nil, copies, nil); err != nil {
		if dErr := s.store.DeleteFileSet(created.ID); dErr != nil {
			log.Printf("api: create file set %q: removing it after its copy rule failed: %v", fs.Name, dErr) //nolint:gosec // G706: name is %q-quoted
		}
		return store.FileSet{}, err
	}
	return created, nil
}

// newTargetExclusion is "leave these out here too" from the new-target question.
type newTargetExclusion struct {
	Identities []string `json:"identities"` // items with an own rule; each gets the target id added to its skip
	Default    bool     `json:"default"`    // add the target id to the default's skip as well
}

// checkExclusion refuses an answer the new-target question could not have
// offered, before the target it belongs to is written.
func checkExclusion(domain string, ex newTargetExclusion) error {
	prefix := domainTagPrefix(domain)
	if prefix == "" {
		return errInvalidPlacement
	}
	for _, id := range ex.Identities {
		if strings.HasPrefix(id, "stack:") {
			return store.ErrStackCopyRule
		}
		if !strings.HasPrefix(id, prefix) || id == prefix {
			return errInvalidPlacement
		}
	}
	return nil
}

// excludeFromTarget leaves the named items, and with ex.Default the default, out
// of a target they would otherwise start copying to.
func (s *Service) excludeFromTarget(domain, targetID string, ex newTargetExclusion) error {
	if err := checkExclusion(domain, ex); err != nil {
		return err
	}
	s.placementMu.Lock()
	defer s.placementMu.Unlock()
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	p, err := s.readPlacement(settings, domain)
	if err != nil {
		return err
	}
	if !containsTarget(p.Targets, targetID) {
		return errNotATarget
	}
	for _, id := range ex.Identities {
		skip, _ := p.resolvedSkip(id)
		if err := s.store.SetCopyRule(domain, id, withTarget(skip, targetID)); err != nil {
			return err
		}
	}
	if ex.Default {
		d := p.State.Default
		if _, err := s.store.PutPlacementDefault(domain, d.Home, withTarget(d.Skip, targetID)); err != nil {
			return err
		}
	}
	return nil
}

// withTarget adds a target to a skip list; ["*"] already leaves everything out.
func withTarget(skip []string, targetID string) []string {
	if skipsEverything(skip) || slices.Contains(skip, targetID) {
		return skip
	}
	return append(slices.Clone(skip), targetID)
}
