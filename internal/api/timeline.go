package api

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// timelinePlace is one place an item's backups can lie at.
type timelinePlace struct {
	Place      string `json:"place"` // "local" | "offsite:<id>"
	Label      string `json:"label"` // "" for the domain path
	Kind       string `json:"kind"`  // "home" | "target"
	Remote     bool   `json:"remote"`
	Enabled    bool   `json:"enabled"`
	AppendOnly bool   `json:"appendOnly"`
	State      string `json:"state"` // "read" | "unchecked" | "unreadable"
	Error      string `json:"error,omitempty"`
}

// timelineMark is what one place holds of one backup.
type timelineMark struct {
	Place       string   `json:"place"`
	SnapshotIDs []string `json:"snapshotIds"` // newest first
	Tags        []string `json:"tags"`
	Incomplete  bool     `json:"incomplete,omitempty"` // a VM run whose disk snapshots are not all here
}

// timelineRow is one backup, keyed by restic.Identity, with a mark for every
// place that holds it.
type timelineRow struct {
	Key    string         `json:"key"`
	Time   string         `json:"time"`
	Places []timelineMark `json:"places"`
}

type timeline struct {
	Places []timelinePlace `json:"places"`
	Rows   []timelineRow   `json:"rows"`
}

// timelineItem is whose snapshots a timeline shows and where the item writes.
type timelineItem struct {
	domain   string
	identity string   // "" for flash and config, whose repositories hold nothing else
	homeID   string   // named repository id, "" for the domain path
	zvolDevs []string // disks a restore of the VM looks up by their own snapshot
}

// placeRef is a place with what it takes to list it; openErr says why its
// location could not be worked out.
type placeRef struct {
	timelinePlace
	repo    string
	mode    restic.Mode
	target  store.OffsiteTarget
	openErr error
}

func (s *Service) timelineItemFor(domain, key string) (timelineItem, error) {
	if domain == "flash" || domain == "config" {
		return timelineItem{domain: domain}, nil
	}
	ref := store.ItemRef{Domain: domain, Key: key}
	identity, err := s.itemIdentity(ref)
	if err != nil {
		return timelineItem{}, err
	}
	home, err := s.store.ItemHome(ref)
	if err != nil {
		return timelineItem{}, err
	}
	it := timelineItem{domain: domain, identity: identity, homeID: home.Repo}
	if domain == "vms" {
		vm, err := s.store.GetVMTargetByName(key)
		switch {
		case err == nil:
			it.zvolDevs = zvolDevsOf(vm.Definition)
		case !errors.Is(err, sql.ErrNoRows):
			return timelineItem{}, err
		}
	}
	return it, nil
}

// zvolDevsOf returns the target devs of a VM's zvol disks, each of which a
// restore looks up as vm:<name>:zvol:<dev> in the run's vmrun group.
func zvolDevsOf(definition string) []string {
	var def vmDefinition
	if err := json.Unmarshal([]byte(definition), &def); err != nil {
		return nil
	}
	parsed, err := virshcli.ParseDomain(def.DomainXML)
	if err != nil {
		return nil
	}
	var devs []string
	for _, bd := range parsed.BlockDisks {
		if _, ok := virshcli.ZvolDatasetFromDevPath(bd.Source); ok && bd.Dev != "" {
			devs = append(devs, bd.Dev)
		}
	}
	return devs
}

// isDisk reports whether snap is one of the VM's disk snapshots rather than a run.
func (it timelineItem) isDisk(snap restic.Snapshot) bool {
	return it.domain == "vms" && slices.ContainsFunc(snap.Tags, func(tag string) bool {
		return strings.HasPrefix(tag, it.identity+":zvol:")
	})
}

// incomplete reports whether a place misses a disk snapshot a restore of the
// run would look up there. Without a vmrun tag there is no group to look them
// up in, and a backup of a VM with zvol disks always sets one.
func (it timelineItem) incomplete(run restic.Snapshot, own []restic.Snapshot) bool {
	if len(it.zvolDevs) == 0 {
		return false
	}
	group := vmRunTag(own, run.ID)
	if group == "" {
		return true
	}
	for _, dev := range it.zvolDevs {
		disk := it.identity + ":zvol:" + dev
		if !slices.ContainsFunc(own, func(s restic.Snapshot) bool {
			return slices.Contains(s.Tags, group) && slices.Contains(s.Tags, disk)
		}) {
			return true
		}
	}
	return false
}

// homePlace is where the item writes: its repository, or the domain path while
// it has none. A repository that is switched off or gone leaves the place
// unreadable, the same answer a restore from it gives, rather than a silent fall
// back to the domain path.
func (s *Service) homePlace(settings store.Settings, it timelineItem) placeRef {
	p := placeRef{timelinePlace: timelinePlace{Place: "local", Kind: "home", Enabled: true}}
	if it.homeID != "" {
		if named, err := s.store.GetNamedRepo(it.homeID); err == nil {
			p.Label, p.Enabled = named.Name, named.Enabled
		}
	}
	p.repo, p.openErr = s.itemRepoPath(it.homeID, func() (string, error) { return s.repoFor(settings, it.domain, "local") })
	if p.openErr == nil {
		p.mode = s.primaryModeFor(settings, it.domain, p.repo)
		p.Remote = restic.IsRemoteRepo(p.repo)
		p.AppendOnly = s.primaryIsImmutable(it.domain, p.repo)
	}
	return p
}

func (s *Service) targetPlace(settings store.Settings, t store.OffsiteTarget) placeRef {
	p := placeRef{
		timelinePlace: timelinePlace{
			Place:      offsiteSourcePrefix + t.ID,
			Label:      placementTargetName(t),
			Kind:       "target",
			Remote:     restic.IsRemoteRepo(t.Repo),
			Enabled:    t.Enabled,
			AppendOnly: t.Immutable,
		},
		mode:   s.offsiteModeForTarget(settings, t),
		target: t,
	}
	p.repo, p.openErr = s.resolveRepo(t.Repo)
	return p
}

// timelineRefs is the item and its places: the home, then every target row of
// the domain, switched on or off, in the domain's order.
func (s *Service) timelineRefs(domain, key string) (timelineItem, []placeRef, error) {
	it, err := s.timelineItemFor(domain, key)
	if err != nil {
		return timelineItem{}, nil, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return timelineItem{}, nil, fmt.Errorf("read settings: %w", err)
	}
	targets, err := s.store.OffsiteTargetsForDomain(domain)
	if err != nil {
		return timelineItem{}, nil, err
	}
	refs := []placeRef{s.homePlace(settings, it)}
	for _, t := range targets {
		refs = append(refs, s.targetPlace(settings, t))
	}
	return it, refs, nil
}

// ownedAt lists one place and keeps the snapshots the item owns there.
func (s *Service) ownedAt(ctx context.Context, it timelineItem, p placeRef) ([]restic.Snapshot, error) {
	if p.openErr != nil {
		return nil, p.openErr
	}
	all, err := s.listRepo(ctx, p.repo, p.mode)
	if err != nil {
		return nil, err
	}
	if it.identity == "" {
		return all, nil
	}
	oc, err := s.ownerContextFor(it.domain)
	if err != nil {
		return nil, err
	}
	owners := oc.owners(all)
	var own []restic.Snapshot
	for _, snap := range all {
		if owners[snap.ID].Owner == it.identity {
			own = append(own, snap)
		}
	}
	return own, nil
}

// readPlace lists one place into rows. A place that cannot be listed comes back
// unreadable, with the reason and no rows.
func (s *Service) readPlace(ctx context.Context, it timelineItem, p placeRef) (timelinePlace, []timelineRow) {
	own, err := s.ownedAt(ctx, it, p)
	if err != nil {
		p.State, p.Error = "unreadable", scrubError(err)
		return p.timelinePlace, []timelineRow{}
	}
	p.State = "read"
	return p.timelinePlace, rowsAt(it, p.Place, own)
}

// rowsAt groups one place's snapshots by restic.Identity. A VM's disk snapshots
// are no rows of their own; two snapshots of one key, as restic leaves after
// copying a re-tagged snapshot again, share a mark.
func rowsAt(it timelineItem, place string, own []restic.Snapshot) []timelineRow {
	rows := []timelineRow{}
	index := map[string]int{}
	for _, snap := range newestFirst(own) {
		if it.isDisk(snap) {
			continue
		}
		key := restic.Identity(snap)
		if i, ok := index[key]; ok {
			rows[i].Places[0].SnapshotIDs = append(rows[i].Places[0].SnapshotIDs, snap.ID)
			continue
		}
		index[key] = len(rows)
		rows = append(rows, timelineRow{Key: key, Time: snap.Time, Places: []timelineMark{{
			Place:       place,
			SnapshotIDs: []string{snap.ID},
			Tags:        append([]string{}, snap.Tags...),
			Incomplete:  it.incomplete(snap, own),
		}}})
	}
	return rows
}

// newestFirst orders snapshots by time, newest first. restic lists oldest
// first, so of two with the same time the one listed later comes first.
func newestFirst(snaps []restic.Snapshot) []restic.Snapshot {
	out := slices.Clone(snaps)
	slices.Reverse(out)
	slices.SortStableFunc(out, func(a, b restic.Snapshot) int { return cmp.Compare(unixOf(b.Time), unixOf(a.Time)) })
	return out
}

// unixOf is restic's time text in Unix seconds, 0 when it does not parse.
func unixOf(text string) int64 {
	t, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return 0
	}
	return t.Unix()
}

// mergeRows puts the rows of several places together by key, newest first.
func mergeRows(parts [][]timelineRow) []timelineRow {
	out := []timelineRow{}
	index := map[string]int{}
	for _, rows := range parts {
		for _, r := range rows {
			if i, ok := index[r.Key]; ok {
				out[i].Places = append(out[i].Places, r.Places...)
				continue
			}
			index[r.Key] = len(out)
			out = append(out, r)
		}
	}
	slices.SortStableFunc(out, func(a, b timelineRow) int {
		return cmp.Or(cmp.Compare(unixOf(b.Time), unixOf(a.Time)), cmp.Compare(a.Key, b.Key))
	})
	return out
}

// Timeline lists an item's backups over all its places. Local places are read
// at once; a remote one stays unchecked until TimelinePlace reads it, so opening
// a timeline never reaches for a bucket.
func (s *Service) Timeline(ctx context.Context, domain, key string) (timeline, error) {
	it, refs, err := s.timelineRefs(domain, key)
	if err != nil {
		return timeline{}, err
	}
	out := timeline{Places: make([]timelinePlace, 0, len(refs))}
	var parts [][]timelineRow
	for _, p := range refs {
		if p.Remote {
			p.State = "unchecked"
			out.Places = append(out.Places, p.timelinePlace)
			continue
		}
		place, rows := s.readPlace(ctx, it, p)
		out.Places = append(out.Places, place)
		parts = append(parts, rows)
	}
	out.Rows = mergeRows(parts)
	return out, nil
}

// TimelinePlace reads one place of an item's timeline; its rows carry only that
// place's marks. An offsite id must name one of the domain's targets.
func (s *Service) TimelinePlace(ctx context.Context, domain, key, place string) (timelinePlace, []timelineRow, error) {
	it, err := s.timelineItemFor(domain, key)
	if err != nil {
		return timelinePlace{}, nil, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return timelinePlace{}, nil, fmt.Errorf("read settings: %w", err)
	}
	var p placeRef
	if place == "local" {
		p = s.homePlace(settings, it)
	} else {
		t, err := s.offsiteTargetForSource(settings, domain, place)
		if err != nil {
			return timelinePlace{}, nil, err
		}
		p = s.targetPlace(settings, t)
	}
	tp, rows := s.readPlace(ctx, it, p)
	return tp, rows, nil
}

// timelineDomains are the domains that carry a timeline: the three that carry
// copy rules and named repositories, plus flash and config, whose repository
// is a single path.
var timelineDomains = []string{"containers", "vms", "files", "flash", "config"}

// validTimelinePlace reports whether place is "local" or "offsite:" followed
// by a well-formed target id.
func validTimelinePlace(place string) bool {
	if place == "local" {
		return true
	}
	id, ok := strings.CutPrefix(place, offsiteSourcePrefix)
	return ok && validOffsiteTargetID(id)
}

// handleTimeline serves an item's timeline over all its places, or with
// ?place= just the one the caller asked to read.
func (h *Handler) handleTimeline(w http.ResponseWriter, r *http.Request) {
	domain, key, ok := h.itemParam(w, r, timelineDomains...)
	if !ok {
		return
	}
	if place := r.URL.Query().Get("place"); place != "" {
		if !validTimelinePlace(place) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid place"})
			return
		}
		p, rows, err := h.svc.TimelinePlace(r.Context(), domain, key, place)
		if err != nil {
			placementFail(w, err, nil)
			return
		}
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"place": p, "rows": rows}))
		return
	}
	tl, err := h.svc.Timeline(r.Context(), domain, key)
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"places": tl.Places, "rows": tl.Rows}))
}

// handleStackDir serves GET /api/stacks/{project}/dir: whether the project
// folder has a snapshot at the source, and from when.
func (h *Handler) handleStackDir(w http.ResponseWriter, r *http.Request) {
	project, ok := stackParam(w, r)
	if !ok {
		return
	}
	d, err := h.svc.latestStackDir(r.Context(), project, sourceParam(r))
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"found": d.found, "time": d.snap.Time}))
}

// placeDelete is what a delete takes at one place.
type placeDelete struct {
	Place       string   `json:"place"`
	Label       string   `json:"label"`
	SnapshotIDs []string `json:"snapshotIds"`
}

// otherPlace is a place a delete leaves alone, and why.
type otherPlace struct {
	Place string `json:"place"`
	Label string `json:"label"`
	State string `json:"state"` // "holds" | "missing" | "unreadable" | "append-only"
}

// rowIDs is what one place holds of a row: its snapshots and, for a VM, the
// disk snapshots of those runs.
func (it timelineItem) rowIDs(own []restic.Snapshot, rowKey string) []string {
	var ids, runs []string
	for _, snap := range own {
		if it.isDisk(snap) || restic.Identity(snap) != rowKey {
			continue
		}
		ids = append(ids, snap.ID)
		if tag := vmRunTag(own, snap.ID); tag != "" {
			runs = append(runs, tag)
		}
	}
	for _, snap := range own {
		if it.isDisk(snap) && slices.ContainsFunc(runs, func(tag string) bool { return slices.Contains(snap.Tags, tag) }) {
			ids = append(ids, snap.ID)
		}
	}
	return ids
}

// askedPlaces marks the places a delete names among the item's places; naming
// none means every place. An id that is not a target of the domain is refused.
func askedPlaces(refs []placeRef, places []string) (map[string]bool, error) {
	asked := make(map[string]bool, len(refs))
	for _, p := range refs {
		asked[p.Place] = len(places) == 0
	}
	for _, place := range places {
		if _, ok := asked[place]; !ok {
			return nil, errUnknownOffsiteTarget
		}
		asked[place] = true
	}
	return asked, nil
}

// timelineDeletePreview lists every place live and splits them into the ones a
// delete of the row would take and the rest, so the question can tell the last
// copy from a place that could not be checked.
func (s *Service) timelineDeletePreview(ctx context.Context, domain, key, rowKey string, places []string) ([]placeDelete, []otherPlace, error) {
	it, refs, err := s.timelineRefs(domain, key)
	if err != nil {
		return nil, nil, err
	}
	asked, err := askedPlaces(refs, places)
	if err != nil {
		return nil, nil, err
	}
	del, others := []placeDelete{}, []otherPlace{}
	for _, p := range refs {
		own, err := s.ownedAt(ctx, it, p)
		ids := it.rowIDs(own, rowKey)
		other := otherPlace{Place: p.Place, Label: p.Label}
		switch {
		case err != nil:
			other.State = "unreadable"
		case len(ids) == 0:
			other.State = "missing"
		case !asked[p.Place]:
			other.State = "holds"
		case p.AppendOnly:
			other.State = "append-only"
		default:
			del = append(del, placeDelete{Place: p.Place, Label: p.Label, SnapshotIDs: ids})
			continue
		}
		others = append(others, other)
	}
	return del, others, nil
}

// timelineDelete forgets the confirmed ids of a row at each place, under the
// domain lock. What a place no longer holds of the row stays untouched, and a
// place that is append-only by now is left out and named.
func (s *Service) timelineDelete(ctx context.Context, domain, key, rowKey string, del []placeDelete) ([]placeDelete, []otherPlace, error) {
	deleted, skipped := []placeDelete{}, []otherPlace{}
	unlock, ok := s.tryLockDomainFor(domain, "delete")
	if !ok {
		return deleted, skipped, errDomainBusy
	}
	defer unlock()

	it, refs, err := s.timelineRefs(domain, key)
	if err != nil {
		return deleted, skipped, err
	}
	names := make([]string, 0, len(del))
	for _, d := range del {
		names = append(names, d.Place)
	}
	if _, err := askedPlaces(refs, names); err != nil {
		return deleted, skipped, err
	}
	for _, d := range del {
		// askedPlaces just confirmed every d.Place names one of refs, so the
		// index below is never -1.
		p := refs[slices.IndexFunc(refs, func(r placeRef) bool { return r.Place == d.Place })]
		if p.AppendOnly {
			skipped = append(skipped, otherPlace{Place: p.Place, Label: p.Label, State: "append-only"})
			continue
		}
		own, err := s.ownedAt(ctx, it, p)
		if err != nil {
			return deleted, skipped, err
		}
		held := it.rowIDs(own, rowKey)
		ids := slices.DeleteFunc(slices.Clone(d.SnapshotIDs), func(id string) bool { return !slices.Contains(held, id) })
		if len(ids) == 0 {
			continue
		}
		s.unlockStale(ctx, p.repo, p.mode)
		if err := s.engine.Forget(ctx, p.repo, ids, false, p.mode); err != nil {
			return deleted, skipped, fmt.Errorf("delete at %s: %w", cmp.Or(p.Label, "the item's location"), err)
		}
		deleted = append(deleted, placeDelete{Place: p.Place, Label: p.Label, SnapshotIDs: ids})
		s.adjustObservedCopies(it, p, own, ids)
	}
	return deleted, skipped, nil
}

// adjustObservedCopies corrects what a target is seen to hold of the item after
// ids were forgotten there, counted like a listing: every snapshot the item owns.
func (s *Service) adjustObservedCopies(it timelineItem, p placeRef, own []restic.Snapshot, gone []string) {
	if it.identity == "" || p.Kind != "target" {
		return
	}
	var left int
	var latest int64
	for _, snap := range own {
		if !slices.Contains(gone, snap.ID) {
			left++
			latest = max(latest, unixOf(snap.Time))
		}
	}
	row := store.ItemCopies{Identity: it.identity, SnapshotCount: left, LatestSnapshotAt: latest}
	if err := s.store.AdjustItemCopies(it.domain, p.target.ID, time.Now().Unix(), []store.ItemCopies{row}); err != nil {
		log.Printf("api: timeline delete: observed copies of %q stay until the next listing: %v", it.identity, err) //nolint:gosec // G706: identity is %q-quoted
	}
}

// timelineRowParams reads the item and the row key of a timeline row route.
func (h *Handler) timelineRowParams(w http.ResponseWriter, r *http.Request) (domain, key, rowKey string, ok bool) {
	domain, key, ok = h.itemParam(w, r, timelineDomains...)
	if !ok {
		return "", "", "", false
	}
	rowKey = r.PathValue("key")
	if !backup.ValidSnapshotID(rowKey) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid backup key"})
		return "", "", "", false
	}
	return domain, key, rowKey, true
}

// handleTimelineDeletePreview serves GET /api/items/{domain}/{name}/timeline/{key}/delete.
// Every ?place= names a place to delete at; none means every place of the row.
func (h *Handler) handleTimelineDeletePreview(w http.ResponseWriter, r *http.Request) {
	domain, key, rowKey, ok := h.timelineRowParams(w, r)
	if !ok {
		return
	}
	places := r.URL.Query()["place"]
	if slices.ContainsFunc(places, func(p string) bool { return !validTimelinePlace(p) }) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid place"})
		return
	}
	del, others, err := h.svc.timelineDeletePreview(r.Context(), domain, key, rowKey, places)
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"delete": del, "others": others}))
}

// handleTimelineDelete serves DELETE /api/items/{domain}/{name}/timeline/{key}.
// A refusal after some places were done still says which.
func (h *Handler) handleTimelineDelete(w http.ResponseWriter, r *http.Request) {
	domain, key, rowKey, ok := h.timelineRowParams(w, r)
	if !ok {
		return
	}
	var body struct {
		Places []placeDelete `json:"places"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	for _, d := range body.Places {
		if !validTimelinePlace(d.Place) || slices.ContainsFunc(d.SnapshotIDs, func(id string) bool { return !backup.ValidSnapshotID(id) }) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid place or backup id"})
			return
		}
	}
	deleted, skipped, err := h.svc.timelineDelete(r.Context(), domain, key, rowKey, body.Places)
	result := map[string]any{"deleted": deleted, "skipped": skipped}
	if err != nil {
		placementFail(w, err, result)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(result))
}

var errSnapshotMissing = errors.New("the chosen place no longer holds this backup")

// notInListing is a snapshot id the chosen source does not hold for the item.
type notInListing struct{ id, owner string }

func (e notInListing) Error() string {
	if e.owner == "" {
		return "snapshot not found"
	}
	return fmt.Sprintf("snapshot %s does not belong to this %s", e.id, e.owner)
}

// restoreFail answers a restore, a file listing or a download that could not
// start. From a named target an id it does not hold comes back as
// snapshot-missing, so the timeline can read that place again and offer the
// next one instead of showing a restic error.
func restoreFail(w http.ResponseWriter, source string, err error) {
	if offsiteTargetIDFromSource(source) != "" && errors.As(err, new(notInListing)) {
		err = fmt.Errorf("%w: %w", errSnapshotMissing, err)
	}
	placementFail(w, err, nil)
}
