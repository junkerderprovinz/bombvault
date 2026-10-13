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
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// timelinePlace is one place an item's backups can lie at.
type timelinePlace struct {
	Place      string `json:"place"`              // "local" | "offsite:<id>"
	Label      string `json:"label"`              // "" for the domain path
	DirectOf   string `json:"directOf,omitempty"` // the target a direct home without a name of its own goes by
	Kind       string `json:"kind"`               // "home" | "target" | "earlier"
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

// timelineRow is one backup with a mark for every place that holds it. Its key
// is the restic.Identity of its snapshot, or the stamp of a ZFS run, whose mark
// lists the snapshots of all its datasets.
type timelineRow struct {
	Key  string `json:"key"`
	Time string `json:"time"`
	// Size is what restic read for the backup, before deduplication and
	// compression. It is absent where neither a run nor the snapshot recorded it.
	Size   *int64         `json:"size,omitempty"`
	Places []timelineMark `json:"places"`
}

type timeline struct {
	Places []timelinePlace `json:"places"`
	Rows   []timelineRow   `json:"rows"`
}

// timelineItem is whose snapshots a timeline shows and where the item writes.
type timelineItem struct {
	domain       string
	identity     string   // "" for flash and config, whose repositories hold nothing else
	homeID       string   // named repository id, "" for the domain path
	earlier      []string // homes the item held before it followed its section elsewhere
	zvolDevs     []string // disks a restore of the VM looks up by their own snapshot
	diskPrefixes []string // vm:<name>:zvol: for a VM's name and each of its former names
	zfsRoot      string   // root dataset of a ZFS entry
	// zfsRuns is what each recorded run of a ZFS entry did to each dataset,
	// by the run's stamp.
	zfsRuns map[string]map[string]string
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
	if domain == zfsDomain {
		d, err := s.store.GetZFSDataset(key)
		if err != nil {
			return timelineItem{}, fmt.Errorf("load dataset: %w", err)
		}
		return timelineItem{domain: domain, homeID: d.Repo, zfsRoot: d.Dataset, zfsRuns: s.zfsRunOutcomes(d.ID)}, nil
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
	rowID, err := s.itemRowID(ref)
	if err != nil {
		return timelineItem{}, err
	}
	if rowID != "" {
		former, err := s.store.FormerHomes(domain, rowID)
		if err != nil {
			return timelineItem{}, err
		}
		it.earlier = slices.DeleteFunc(former, func(repo string) bool { return repo == home.Repo })
	}
	if domain != "vms" {
		return it, nil
	}
	it.diskPrefixes = []string{identity + ":zvol:"}
	vm, err := s.store.GetVMTargetByName(key)
	if errors.Is(err, sql.ErrNoRows) {
		return it, nil
	}
	if err != nil {
		return timelineItem{}, err
	}
	it.zvolDevs = zvolDevsOf(vm.Definition)
	former, err := s.store.AliasNames("vm", vm.ID)
	if err != nil {
		return timelineItem{}, err
	}
	for _, name := range former {
		it.diskPrefixes = append(it.diskPrefixes, "vm:"+name+":zvol:")
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

// isDisk reports whether snap is one of the VM's disk snapshots rather than a
// run. A run from before a rename keeps its disks under the name it had then.
func (it timelineItem) isDisk(snap restic.Snapshot) bool {
	return slices.ContainsFunc(snap.Tags, func(tag string) bool {
		return slices.ContainsFunc(it.diskPrefixes, func(prefix string) bool { return strings.HasPrefix(tag, prefix) })
	})
}

// isDump reports whether snap is a container's database dump, which the card
// lists apart from the backups a timeline row restores.
func isDump(snap restic.Snapshot) bool {
	return slices.ContainsFunc(snap.Tags, func(tag string) bool { return strings.HasPrefix(tag, dbDumpIdentityPrefix) })
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
	p, _ := s.ownPlace(settings, it.domain, it.homeID, timelinePlace{Place: "local", Kind: "home", Enabled: true})
	return p
}

// earlierPlaces are the item's former homes as places. One whose repository
// was deleted since has nothing left to list.
func (s *Service) earlierPlaces(settings store.Settings, it timelineItem) []placeRef {
	var out []placeRef
	for _, repoID := range it.earlier {
		p, known := s.ownPlace(settings, it.domain, repoID, timelinePlace{Place: earlierPlace(repoID), Kind: "earlier", Enabled: true})
		if known {
			out = append(out, p)
		}
	}
	return out
}

// ownPlace is a place the item writes to or wrote to: the named repository
// repoID, or the domain path for "". known is false for an id without a row.
func (s *Service) ownPlace(settings store.Settings, domain, repoID string, tp timelinePlace) (p placeRef, known bool) {
	p, known = placeRef{timelinePlace: tp}, repoID == ""
	if !known {
		if named, err := s.store.GetNamedRepo(repoID); err == nil {
			p.Label, p.DirectOf, p.Enabled, known = named.Name, s.directOf(named), named.Enabled, true
		}
	}
	p.repo, p.openErr = s.itemRepoPath(repoID, func() (string, error) { return s.repoFor(settings, domain, "local") })
	if p.openErr == nil {
		p.mode = s.primaryModeFor(settings, domain, p.repo)
		p.Remote = restic.IsRemoteRepo(p.repo)
		p.AppendOnly = s.primaryIsImmutable(domain, p.repo)
	}
	return p, known
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
// the domain, switched on or off, in the domain's order, then the homes it
// held before it followed its section elsewhere.
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
	return it, append(refs, s.earlierPlaces(settings, it)...), nil
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
	if it.zfsRoot != "" {
		var own []restic.Snapshot
		for _, snap := range all {
			if _, member := zfsSnapshotDataset(snap, it.zfsRoot); member {
				own = append(own, snap)
			}
		}
		return own, nil
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

// readNow is readPlace for a place Timeline reads at once. A remote one, the
// home, gets remoteHomeListTimeout.
func (s *Service) readNow(ctx context.Context, it timelineItem, p placeRef) (timelinePlace, []timelineRow) {
	if !p.Remote {
		return s.readPlace(ctx, it, p)
	}
	ctx, cancel := context.WithTimeout(ctx, remoteHomeListTimeout)
	defer cancel()
	return s.readPlace(ctx, it, p)
}

// rowsAt groups one place's snapshots by restic.Identity. A VM's disk snapshots
// are no rows of their own; two snapshots of one key, as restic leaves after
// copying a re-tagged snapshot again, share a mark.
func rowsAt(it timelineItem, place string, own []restic.Snapshot) []timelineRow {
	if it.zfsRoot != "" {
		return zfsRowsAt(it, place, own)
	}
	rows := []timelineRow{}
	index := map[string]int{}
	for _, snap := range newestFirst(own) {
		if it.isDisk(snap) || isDump(snap) {
			continue
		}
		key := restic.Identity(snap)
		if i, ok := index[key]; ok {
			rows[i].Places[0].SnapshotIDs = append(rows[i].Places[0].SnapshotIDs, snap.ID)
			continue
		}
		index[key] = len(rows)
		row := timelineRow{Key: key, Time: snap.Time, Places: []timelineMark{{
			Place:       place,
			SnapshotIDs: []string{snap.ID},
			Tags:        append([]string{}, snap.Tags...),
			Incomplete:  it.incomplete(snap, own),
		}}}
		if !row.Places[0].Incomplete {
			row.Size = recordedSize(append(it.disksOf(snap, own), snap))
		}
		rows = append(rows, row)
	}
	return rows
}

// disksOf returns the zvol disk snapshots a VM run wrote beside its own.
func (it timelineItem) disksOf(run restic.Snapshot, own []restic.Snapshot) []restic.Snapshot {
	group := vmRunTag(own, run.ID)
	if group == "" {
		return nil
	}
	var disks []restic.Snapshot
	for _, snap := range own {
		if it.isDisk(snap) && slices.Contains(snap.Tags, group) {
			disks = append(disks, snap)
		}
	}
	return disks
}

// recordedSize adds up what restic recorded as read on each snapshot. It is nil
// when one of them carries no summary, as those written before restic 0.17.
func recordedSize(snaps []restic.Snapshot) *int64 {
	var total int64
	for _, snap := range snaps {
		if snap.Summary == nil {
			return nil
		}
		total += clampToInt64(snap.Summary.TotalBytesProcessed)
	}
	return &total
}

// zfsRowsAt groups one place's snapshots by the run that wrote them, the root
// dataset first. A run counts as incomplete at a place that lacks a dataset
// the run backed up.
func zfsRowsAt(it timelineItem, place string, own []restic.Snapshot) []timelineRow {
	rows := []timelineRow{}
	index := map[string]int{}
	held := map[string][]string{}
	members := map[string][]restic.Snapshot{}
	byDataset := slices.Clone(own)
	slices.SortStableFunc(byDataset, func(a, b restic.Snapshot) int {
		da, _ := zfsSnapshotDataset(a, it.zfsRoot)
		db, _ := zfsSnapshotDataset(b, it.zfsRoot)
		return cmp.Compare(da, db)
	})
	for _, snap := range byDataset {
		stamp, ok := zfsStampOf(snap)
		if !ok {
			continue
		}
		i, seen := index[stamp]
		if !seen {
			i = len(rows)
			index[stamp] = i
			at, _ := zfs.StampTime(stamp)
			rows = append(rows, timelineRow{Key: stamp, Time: at.Format(time.RFC3339), Places: []timelineMark{{
				Place: place, SnapshotIDs: []string{}, Tags: []string{},
			}}})
		}
		mark := &rows[i].Places[0]
		mark.SnapshotIDs = append(mark.SnapshotIDs, snap.ID)
		for _, tag := range snap.Tags {
			if !slices.Contains(mark.Tags, tag) {
				mark.Tags = append(mark.Tags, tag)
			}
		}
		dataset, _ := zfsSnapshotDataset(snap, it.zfsRoot)
		// A dataset copied again after a tag lies here twice and counts once.
		if !slices.Contains(held[stamp], dataset) {
			held[stamp] = append(held[stamp], dataset)
			members[stamp] = append(members[stamp], snap)
		}
	}
	for i, row := range rows {
		for dataset, outcome := range it.zfsRuns[row.Key] {
			if outcome == outcomeBackedUp && !slices.Contains(held[row.Key], dataset) {
				rows[i].Places[0].Incomplete = true
			}
		}
		if !rows[i].Places[0].Incomplete {
			rows[i].Size = recordedSize(members[row.Key])
		}
	}
	slices.SortFunc(rows, func(a, b timelineRow) int { return cmp.Compare(b.Key, a.Key) })
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
				if out[i].Size == nil {
					out[i].Size = r.Size
				}
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

// remoteHomeListTimeout bounds the one remote listing opening a timeline
// waits for, so a home that does not answer leaves the page readable.
const remoteHomeListTimeout = 20 * time.Second

// Timeline lists an item's backups over all its places. Local places and the
// home are read at once, the home also off the premises, since it is where the
// newest backups land. Any other remote place stays unchecked until
// TimelinePlace reads it.
func (s *Service) Timeline(ctx context.Context, domain, key string) (timeline, error) {
	it, refs, err := s.timelineRefs(domain, key)
	if err != nil {
		return timeline{}, err
	}
	out := timeline{Places: make([]timelinePlace, 0, len(refs))}
	var parts [][]timelineRow
	for _, p := range refs {
		if p.Remote && p.Kind != "home" {
			p.State = "unchecked"
			out.Places = append(out.Places, p.timelinePlace)
			continue
		}
		place, rows := s.readNow(ctx, it, p)
		// A former home the item left before any backup of it landed there
		// has nothing of it to show.
		if p.Kind == "earlier" && place.State == "read" && len(rows) == 0 {
			continue
		}
		out.Places = append(out.Places, place)
		parts = append(parts, rows)
	}
	out.Rows = mergeRows(parts)
	measured, err := s.measuredSizes(domain, key)
	if err != nil {
		return timeline{}, err
	}
	withMeasuredSizes(out.Rows, measured)
	return out, nil
}

// measuredSizes is what the item's runs read, by the key of the timeline row
// each of them wrote.
func (s *Service) measuredSizes(domain, key string) (map[string]int64, error) {
	if domain == zfsDomain {
		return s.store.ZFSBackupSizesByStamp(key)
	}
	id, err := s.runTargetID(domain, key)
	if err != nil || id == "" {
		return nil, err
	}
	return s.store.BackupSizesBySnapshot(id)
}

// withMeasuredSizes puts what a run measured in place of what the snapshots
// say. The two agree where both exist; the run also knows the size of a backup
// restic wrote no summary for.
func withMeasuredSizes(rows []timelineRow, measured map[string]int64) {
	for i, row := range rows {
		if size, ok := measured[row.Key]; ok {
			rows[i].Size = &size
		}
	}
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
	switch {
	case place == "local":
		p = s.homePlace(settings, it)
	case isEarlierSource(place):
		earlier := s.earlierPlaces(settings, it)
		i := slices.IndexFunc(earlier, func(r placeRef) bool { return r.Place == place })
		if i < 0 {
			return timelinePlace{}, nil, errNotEarlierPlace
		}
		p = earlier[i]
	default:
		t, err := s.offsiteTargetForSource(settings, domain, place)
		if err != nil {
			return timelinePlace{}, nil, err
		}
		p = s.targetPlace(settings, t)
	}
	tp, rows := s.readPlace(ctx, it, p)
	measured, err := s.measuredSizes(domain, key)
	if err != nil {
		return timelinePlace{}, nil, err
	}
	withMeasuredSizes(rows, measured)
	return tp, rows, nil
}

// timelineDomains are the domains that carry a timeline: the three that carry
// copy rules and named repositories, flash and config, whose repository is a
// single path, and ZFS, whose entries each write to the domain repository or
// one of their own.
var timelineDomains = []string{"containers", "vms", "files", "flash", "config", zfsDomain}

// validTimelinePlace reports whether place is "local", "offsite:" followed by
// a well-formed target id, or a well-formed earlier place.
func validTimelinePlace(place string) bool {
	if _, ok := earlierRepoID(place); ok || place == "local" {
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
	DirectOf    string   `json:"directOf,omitempty"`
	SnapshotIDs []string `json:"snapshotIds"`
}

// otherPlace is a place a delete leaves alone, and why.
type otherPlace struct {
	Place    string `json:"place"`
	Label    string `json:"label"`
	DirectOf string `json:"directOf,omitempty"`
	State    string `json:"state"` // "holds" | "missing" | "unreadable" | "append-only"
}

// rowIDs is what one place holds of a row: its snapshots and, for a VM, the
// disk snapshots of those runs. A ZFS row is every dataset of its run.
func (it timelineItem) rowIDs(own []restic.Snapshot, rowKey string) []string {
	var ids, runs []string
	if it.zfsRoot != "" {
		for _, snap := range own {
			if stamp, ok := zfsStampOf(snap); ok && stamp == rowKey {
				ids = append(ids, snap.ID)
			}
		}
		return ids
	}
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
		other := otherPlace{Place: p.Place, Label: p.Label, DirectOf: p.DirectOf}
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
			del = append(del, placeDelete{Place: p.Place, Label: p.Label, DirectOf: p.DirectOf, SnapshotIDs: ids})
			continue
		}
		others = append(others, other)
	}
	return del, others, nil
}

// timelineDelete forgets the confirmed ids of a row at each place, under the
// domain lock. A place that holds other backups of the row than the question
// showed refuses the whole delete before anything goes, and a place that is
// append-only by now is left out and named.
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
	type confirmed struct {
		p   placeRef
		own []restic.Snapshot
		ids []string
	}
	var todo []confirmed
	for _, d := range del {
		// askedPlaces just confirmed every d.Place names one of refs, so the
		// index below is never -1.
		p := refs[slices.IndexFunc(refs, func(r placeRef) bool { return r.Place == d.Place })]
		if p.AppendOnly {
			skipped = append(skipped, otherPlace{Place: p.Place, Label: p.Label, DirectOf: p.DirectOf, State: "append-only"})
			continue
		}
		own, err := s.ownedAt(ctx, it, p)
		if err != nil {
			return deleted, skipped, err
		}
		held, ids := slices.Sorted(slices.Values(it.rowIDs(own, rowKey))), slices.Sorted(slices.Values(d.SnapshotIDs))
		if !slices.Equal(held, ids) {
			return deleted, skipped, errPlacementStale
		}
		if len(ids) > 0 {
			todo = append(todo, confirmed{p, own, ids})
		}
	}
	for _, c := range todo {
		p, own, ids := c.p, c.own, c.ids
		forget := func() error { return s.engine.Forget(ctx, p.repo, ids, false, p.mode) }
		if err := s.retryAfterUnlock(ctx, p.repo, p.mode, forget); err != nil {
			return deleted, skipped, fmt.Errorf("delete at %s: %w", cmp.Or(p.Label, "the item's location"), err)
		}
		deleted = append(deleted, placeDelete{Place: p.Place, Label: p.Label, DirectOf: p.DirectOf, SnapshotIDs: ids})
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
	valid := backup.ValidSnapshotID
	if domain == zfsDomain {
		valid = zfs.IsBombVaultSnapshot
	}
	if !valid(rowKey) {
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
	if !h.requireAuthForSecrets(w, r, "deleting backups") {
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
	if errors.Is(err, errPairedFolderInUse) {
		refusePaired(w)
		return
	}
	if offsiteTargetIDFromSource(source) != "" && errors.As(err, new(notInListing)) {
		err = fmt.Errorf("%w: %w", errSnapshotMissing, err)
	}
	placementFail(w, err, nil)
}
