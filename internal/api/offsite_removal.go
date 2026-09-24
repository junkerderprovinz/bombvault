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

var (
	errRemovalGrown   = errors.New("more snapshots now exist only at this target than were confirmed; check the list again")
	errNameMismatch   = errors.New("the typed name does not match the item")
	errHomeUnreadable = errors.New("the item's location could not be read, so nothing was deleted")
)

// itemAtTarget is one item's snapshots at one off-site target and what it takes
// to delete them there.
type itemAtTarget struct {
	Target store.OffsiteTarget
	Repo   string
	Mode   restic.Mode
	Snaps  []restic.Snapshot
}

// listRepo lists a repository the way snapshotsForTag does, without its tag
// filter: a local repository never created is empty, one on a share that is not
// mounted is an error.
func (s *Service) listRepo(ctx context.Context, repo string, mode restic.Mode) ([]restic.Snapshot, error) {
	if localRepoMissing(repo) {
		if s.repoEstablished(repo) && !s.destinationMounted(repo) {
			return nil, ErrBackupPathNotMounted
		}
		return nil, nil
	}
	return s.listSnapshots(ctx, repo, mode)
}

// snapScope says how much of an item a delete at a target takes.
type snapScope int

const (
	// taggedForItem is the item's own snapshots, under its name or a former
	// one, the list its backup panel shows.
	taggedForItem snapScope = iota
	// ownedByItem is everything the ownership rule gives the item, a machine's
	// disk images included. Only the window that previews them uses it.
	ownedByItem
)

// listItemAtTarget lists what identity owns at the target the source names, as
// far as the scope reaches. A snapshot whose owner the listing cannot settle
// stays out of both scopes: it may belong to another item.
func (s *Service) listItemAtTarget(ctx context.Context, settings store.Settings, domain, identity, source string, scope snapScope) (itemAtTarget, error) {
	target, err := s.offsiteTargetForSource(settings, domain, source)
	if err != nil {
		return itemAtTarget{}, err
	}
	repo, err := s.resolveRepo(target.Repo)
	if err != nil {
		return itemAtTarget{}, err
	}
	at := itemAtTarget{Target: target, Repo: repo, Mode: s.offsiteModeForTarget(settings, target)}
	all, err := s.listRepo(ctx, repo, at.Mode)
	if err != nil {
		return itemAtTarget{}, err
	}
	oc, err := s.ownerContextFor(domain)
	if err != nil {
		return itemAtTarget{}, err
	}
	owners := oc.owners(all)
	for _, snap := range all {
		if owners[snap.ID].Owner != identity {
			continue
		}
		if scope == taggedForItem && !oc.namesItem(snap, identity) {
			continue
		}
		at.Snaps = append(at.Snaps, snap)
	}
	return at, nil
}

// refuseAppendOnlyTarget asks whether the target a source names is append-only,
// switched off or not.
func (s *Service) refuseAppendOnlyTarget(settings store.Settings, domain, source string) error {
	immutable, err := s.offsiteSourceImmutable(settings, domain, source)
	if err != nil {
		return err
	}
	if immutable {
		return errAppendOnlyOffsiteTarget
	}
	return nil
}

// forgetAtTarget deletes the snapshots the scope gives identity at one off-site
// target and prunes there. Append-only is asked before the domain lock and again
// inside it, after the listing, so a flag switched on in between still refuses;
// check sees that listing and can refuse before anything is deleted.
func (s *Service) forgetAtTarget(ctx context.Context, domain, identity, source string, scope snapScope, check func(store.Settings, itemAtTarget) error) (int, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return 0, fmt.Errorf("read settings: %w", err)
	}
	if err := s.refuseAppendOnlyTarget(settings, domain, source); err != nil {
		return 0, err
	}
	unlock, ok := s.tryLockDomainFor(domain, "delete")
	if !ok {
		return 0, errDomainBusy
	}
	defer unlock()

	at, err := s.listItemAtTarget(ctx, settings, domain, identity, source, scope)
	if err != nil {
		return 0, err
	}
	if settings, err = s.store.GetSettings(); err != nil {
		return 0, fmt.Errorf("read settings: %w", err)
	}
	if err := s.refuseAppendOnlyTarget(settings, domain, source); err != nil {
		return 0, err
	}
	if check != nil {
		if err := check(settings, at); err != nil {
			return 0, err
		}
	}
	if len(at.Snaps) > 0 {
		ids := make([]string, 0, len(at.Snaps))
		for _, snap := range at.Snaps {
			ids = append(ids, snap.ID)
		}
		s.unlockStale(ctx, at.Repo, at.Mode)
		if err := s.engine.Forget(ctx, at.Repo, ids, true, at.Mode); err != nil {
			return 0, fmt.Errorf("forget snapshots: %w", err)
		}
	}
	// The snapshots are gone by this point and the next listing of the target
	// writes the row again, so a failure here must not report a delete that
	// happened as failed.
	if at.Target.ID != "" {
		gone := []store.ItemCopies{{Domain: domain, Identity: identity, TargetID: at.Target.ID}}
		if err := s.store.AdjustItemCopies(domain, at.Target.ID, time.Now().Unix(), gone); err != nil {
			log.Printf("api: delete at target: observed copies of %q stay until the next listing: %v", identity, err) //nolint:gosec // G706: identity is %q-quoted
		}
	}
	return len(at.Snaps), nil
}

// removalPreview is what deleting one item's copies at one target would remove.
type removalPreview struct {
	Target         store.OffsiteTarget
	Name           string            // the item's name as the delete compares it, not the one a card shows
	Snapshots      []restic.Snapshot // the item's snapshots at the target
	OnlyThere      []restic.Snapshot // their restic.Identity is missing at the home
	HomeUnreadable bool
	HomeLabel      string // name of the home repository, "" for the domain path
}

// removalChanged is a refusal decided on a fresh preview; the answer carries that
// preview so the dialog can show what changed.
type removalChanged struct {
	err     error
	preview removalPreview
}

func (e *removalChanged) Error() string { return e.err.Error() }
func (e *removalChanged) Unwrap() error { return e.err }

func (s *Service) offsiteRemovalPreview(ctx context.Context, item store.ItemRef, targetID string) (removalPreview, error) {
	identity, err := s.itemIdentity(item)
	if err != nil {
		return removalPreview{}, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return removalPreview{}, fmt.Errorf("read settings: %w", err)
	}
	at, err := s.listItemAtTarget(ctx, settings, item.Domain, identity, offsiteSourcePrefix+targetID, ownedByItem)
	if err != nil {
		return removalPreview{}, err
	}
	return s.removalPreviewOf(ctx, settings, item, identity, at)
}

// removeFromTarget deletes an item's copies at one target once the confirmed list
// still covers every snapshot that exists only there, recounted inside the lock,
// and the item's name was typed for them.
func (s *Service) removeFromTarget(ctx context.Context, item store.ItemRef, targetID string, confirmed []string, typedName string) (int, error) {
	identity, err := s.itemIdentity(item)
	if err != nil {
		return 0, err
	}
	return s.forgetAtTarget(ctx, item.Domain, identity, offsiteSourcePrefix+targetID, ownedByItem, func(settings store.Settings, at itemAtTarget) error {
		p, err := s.removalPreviewOf(ctx, settings, item, identity, at)
		if err != nil {
			return err
		}
		for _, snap := range p.OnlyThere {
			if slices.Contains(confirmed, snap.ID) {
				continue
			}
			if p.HomeUnreadable {
				return &removalChanged{err: errHomeUnreadable, preview: p}
			}
			return &removalChanged{err: errRemovalGrown, preview: p}
		}
		if len(p.OnlyThere) > 0 && strings.TrimSpace(typedName) != p.Name {
			return errNameMismatch
		}
		return nil
	})
}

// removalPreviewOf compares the item's snapshots at the target with its home. A
// home that cannot be read leaves every copy counted as the only one.
func (s *Service) removalPreviewOf(ctx context.Context, settings store.Settings, item store.ItemRef, identity string, at itemAtTarget) (removalPreview, error) {
	home, err := s.store.ItemHome(item)
	if err != nil {
		return removalPreview{}, err
	}
	placement, err := s.readPlacement(settings, item.Domain)
	if err != nil {
		return removalPreview{}, err
	}
	// An open item's own column reads empty however far its default points
	// elsewhere, and comparing against the domain path instead would call copies
	// the only ones that exist while the default's repository holds them.
	repoID, _ := placement.effectiveHome(home)
	_, name, _ := strings.Cut(identity, ":")
	p := removalPreview{Target: at.Target, Name: name, Snapshots: at.Snaps}
	if repoID != "" {
		if named, nErr := s.store.GetNamedRepo(repoID); nErr == nil {
			p.HomeLabel = named.Name
		}
	}
	held, err := s.homeIdentities(ctx, settings, item.Domain, repoID)
	if err != nil {
		p.HomeUnreadable = true
		p.OnlyThere = at.Snaps
		return p, nil
	}
	for _, snap := range at.Snaps {
		if !held[restic.Identity(snap)] {
			p.OnlyThere = append(p.OnlyThere, snap)
		}
	}
	return p, nil
}

// homeIdentities returns the identity of every snapshot in an item's home: its
// repository, or the domain path while it has none.
func (s *Service) homeIdentities(ctx context.Context, settings store.Settings, domain, repoID string) (map[string]bool, error) {
	repo, err := s.itemRepoPath(repoID, func() (string, error) { return s.repoFor(settings, domain, "local") })
	if err != nil {
		return nil, err
	}
	snaps, err := s.listRepo(ctx, repo, s.primaryModeFor(settings, domain, repo))
	if err != nil {
		return nil, err
	}
	held := make(map[string]bool, len(snaps))
	for _, snap := range snaps {
		held[restic.Identity(snap)] = true
	}
	return held, nil
}

func removalJSON(p removalPreview) map[string]any {
	only := make([]map[string]string, 0, len(p.OnlyThere))
	for _, snap := range p.OnlyThere {
		only = append(only, map[string]string{"id": snap.ID, "time": snap.Time})
	}
	return map[string]any{
		"target":         map[string]any{"id": p.Target.ID, "name": placementTargetName(p.Target), "appendOnly": p.Target.Immutable},
		"name":           p.Name,
		"count":          len(p.Snapshots),
		"onlyThere":      only,
		"homeUnreadable": p.HomeUnreadable,
		"homeLabel":      p.HomeLabel,
	}
}

func (h *Handler) removalParams(w http.ResponseWriter, r *http.Request) (store.ItemRef, string, bool) {
	domain, key, ok := h.itemParam(w, r, store.PlacementDomains...)
	if !ok {
		return store.ItemRef{}, "", false
	}
	targetID := r.PathValue("target")
	if !validOffsiteTargetID(targetID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid target id"})
		return store.ItemRef{}, "", false
	}
	return store.ItemRef{Domain: domain, Key: key}, targetID, true
}

// handleOffsiteRemovalPreview serves GET /api/items/{domain}/{name}/offsite/{target}/removal.
func (h *Handler) handleOffsiteRemovalPreview(w http.ResponseWriter, r *http.Request) {
	item, targetID, ok := h.removalParams(w, r)
	if !ok {
		return
	}
	p, err := h.svc.offsiteRemovalPreview(r.Context(), item, targetID)
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(removalJSON(p)))
}

// handleOffsiteRemoval serves DELETE on the same path. A refusal decided on a
// fresh preview carries it, so the dialog can ask again with the new list.
func (h *Handler) handleOffsiteRemoval(w http.ResponseWriter, r *http.Request) {
	item, targetID, ok := h.removalParams(w, r)
	if !ok {
		return
	}
	var body struct {
		OnlyThere []string `json:"onlyThere"`
		TypedName string   `json:"typedName"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	n, err := h.svc.removeFromTarget(r.Context(), item, targetID, body.OnlyThere, body.TypedName)
	if err != nil {
		var extra map[string]any
		var changed *removalChanged
		if errors.As(err, &changed) {
			extra = map[string]any{"preview": removalJSON(changed.preview)}
		}
		placementFail(w, err, extra)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"deleted": n}))
}
