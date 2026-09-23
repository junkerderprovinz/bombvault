package api

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
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
	identity string
	homeID   string // named repository id, "" for the domain path
}

// placeRef is a place with what it takes to list it; openErr says why its
// location could not be worked out.
type placeRef struct {
	timelinePlace
	repo    string
	mode    restic.Mode
	openErr error
}

func (s *Service) timelineItemFor(domain, key string) (timelineItem, error) {
	ref := store.ItemRef{Domain: domain, Key: key}
	identity, err := s.itemIdentity(ref)
	if err != nil {
		return timelineItem{}, err
	}
	home, err := s.store.ItemHome(ref)
	if err != nil {
		return timelineItem{}, err
	}
	return timelineItem{domain: domain, identity: identity, homeID: home.Repo}, nil
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
		mode: s.offsiteModeForTarget(settings, t),
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

// rowsAt groups one place's snapshots by restic.Identity. Two snapshots of one
// key, as restic leaves after copying a re-tagged snapshot again, share a mark.
func rowsAt(it timelineItem, place string, own []restic.Snapshot) []timelineRow {
	rows := []timelineRow{}
	index := map[string]int{}
	for _, snap := range newestFirst(own) {
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
