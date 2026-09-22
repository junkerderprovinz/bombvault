package api

import (
	"context"
	"log"
	"maps"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Segment ids of the placement bar, shared by the card view, the options and the defaults.
const (
	segmentLocal        = "local"
	segmentLocalOffsite = "local-offsite"
	segmentOffsiteOnly  = "offsite-only"
)

const (
	lockNoTarget       = "no-target"
	lockOwnCredentials = "own-credentials"
	lockAtTarget       = "at-target"
	lockHomeFixed      = "home-fixed"
)

// placementView is the placement block a list row or an item PATCH answers
// with: where the item lives, what it follows, and what the placement bar
// must show as locked.
type placementView struct {
	Segment      string            `json:"segment"`
	Repo         string            `json:"repo"`      // effectiveHome
	RepoLabel    string            `json:"repoLabel"` // name of the repository, "" for the domain path
	RepoKind     homeKind          `json:"repoKind"`
	RepoOff      bool              `json:"repoOff"`
	HomeFollows  bool              `json:"homeFollows"`  // repo_chosen = 0
	CopiesFollow bool              `json:"copiesFollow"` // no own rule
	Skip         []string          `json:"skip"`         // resolvedSkip, never null
	Locked       bool              `json:"locked"`       // a successful run exists
	LockReason   string            `json:"lockReason"`   // "first-backup" | ""
	SegmentLocks map[string]string `json:"segmentLocks"` // segment -> reason, never null
	Paused       bool              `json:"paused"`
	Unreadable   bool              `json:"unreadable"`
}

// placementItem is one item as a placement read needs it: its location, its
// snapshot identity and the start of its last successful run, 0 without one.
type placementItem struct {
	Key         string
	Identity    string
	Home        store.HomeState
	LastSuccess int64
}

// placementViews builds the block for every item of a list in one pass: one
// readPlacement, one ListNamedRepos. Without a readable placement and a
// certain target list every card is unreadable, since chips and locks need
// target ids.
func (s *Service) placementViews(ctx context.Context, settings store.Settings, domain string, items []placementItem) (map[string]placementView, error) {
	p, err := s.readPlacement(settings, domain)
	if err != nil || p.TargetsUncertain {
		return unreadablePlacements(items), nil
	}
	repos, err := s.store.ListNamedRepos()
	if err != nil {
		return nil, err
	}
	named := namedReposByID(repos)
	locks := domainSegmentLocks(p, s.sendToOptions(settings, p, repos, named))
	views := make(map[string]placementView, len(items))
	for _, it := range items {
		views[it.Key] = s.itemPlacementView(settings, p, named, locks, it)
	}
	return views, nil
}

func (s *Service) itemPlacementView(settings store.Settings, p placementRead, named map[string]store.OffsiteTarget, domainLocks map[string]string, it placementItem) placementView {
	repo, _ := p.effectiveHome(it.Home)
	kind := s.homeKindOf(settings, p.Domain, repo, named)
	skip, own := p.resolvedSkip(it.Identity)
	v := placementView{
		Segment:      segmentOf(kind, skip, len(p.Targets) > 0),
		Repo:         repo,
		RepoLabel:    repo,
		RepoKind:     kind,
		HomeFollows:  it.Home.Choice == store.RepoOpen,
		CopiesFollow: !own,
		Skip:         skip,
		Locked:       it.LastSuccess != 0,
		SegmentLocks: maps.Clone(domainLocks),
		Paused:       p.State.Paused(),
	}
	if r, ok := named[repo]; ok {
		v.RepoLabel, v.RepoOff = r.Name, !r.Enabled
	}
	switch kind {
	case homeRemote:
		v.SegmentLocks[segmentLocalOffsite] = lockOwnCredentials
	case homeDirect:
		v.SegmentLocks[segmentLocalOffsite] = lockAtTarget
	}
	if v.Locked {
		v.LockReason = "first-backup"
		for _, seg := range segmentsMovingHome(v.Segment) {
			if _, taken := v.SegmentLocks[seg]; !taken {
				v.SegmentLocks[seg] = lockHomeFixed
			}
		}
	}
	return v
}

// segmentOf reads the segment off the stored rule, never off the targets it
// resolves to, so an item whose only target is switched off keeps its choice.
func segmentOf(kind homeKind, skip []string, hasTargets bool) string {
	switch {
	case kind == homeRemote || kind == homeDirect:
		return segmentOffsiteOnly
	case !hasTargets || skipsEverything(skip):
		return segmentLocal
	}
	return segmentLocalOffsite
}

func segmentsMovingHome(segment string) []string {
	if segment == segmentOffsiteOnly {
		return []string{segmentLocal, segmentLocalOffsite}
	}
	return []string{segmentOffsiteOnly}
}

// domainSegmentLocks is what no item of the domain can choose: copies need an
// enabled target, off-site only somewhere to send to. It reads that step off
// the list the options route offers, so the bar cannot open a step whose
// window would be empty.
func domainSegmentLocks(p placementRead, sendTo []sendToOption) map[string]string {
	locks := map[string]string{}
	if len(p.enabledTargets()) == 0 {
		locks[segmentLocalOffsite] = lockNoTarget
	}
	if len(sendTo) == 0 {
		locks[segmentOffsiteOnly] = lockNoTarget
	}
	return locks
}

func unreadablePlacement() placementView {
	return placementView{Skip: []string{}, SegmentLocks: map[string]string{}, Unreadable: true}
}

func unreadablePlacements(items []placementItem) map[string]placementView {
	views := make(map[string]placementView, len(items))
	for _, it := range items {
		views[it.Key] = unreadablePlacement()
	}
	return views
}

// listPlacements is the placement block of every row of a list. A list is never
// refused over its placement; a failure shows every card as unreadable.
func (s *Service) listPlacements(ctx context.Context, domain string, items []placementItem) map[string]placementView {
	settings, err := s.store.GetSettings()
	if err == nil {
		var views map[string]placementView
		if views, err = s.placementViews(ctx, settings, domain, items); err == nil {
			return views
		}
	}
	log.Printf("api: %s placement: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
	return unreadablePlacements(items)
}

// placementItemOf reads one item the way the list routes see it.
func (s *Service) placementItemOf(item store.ItemRef) (placementItem, error) {
	home, err := s.store.ItemHome(item)
	if err != nil {
		return placementItem{}, err
	}
	identity, err := s.itemIdentity(item)
	if err != nil {
		return placementItem{}, err
	}
	it := placementItem{Key: item.Key, Identity: identity, Home: home}
	rowID, err := s.itemRowID(item)
	if err != nil || rowID == "" {
		return it, err
	}
	run, err := s.store.LastSuccessfulBackup(rowID)
	if err != nil {
		return placementItem{}, err
	}
	if run != nil {
		it.LastSuccess = run.StartedAt
	}
	return it, nil
}

// placementViewOf is one item's card view after a change.
func (s *Service) placementViewOf(ctx context.Context, item store.ItemRef) placementView {
	it, err := s.placementItemOf(item)
	if err != nil {
		log.Printf("api: %s placement of %q: %v", item.Domain, item.Key, err) //nolint:gosec // G706: the key is %q-quoted
		return unreadablePlacement()
	}
	return s.listPlacements(ctx, item.Domain, []placementItem{it})[item.Key]
}
