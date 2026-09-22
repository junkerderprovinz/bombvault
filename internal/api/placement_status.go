package api

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

type placementPlan struct {
	Kind    string   `json:"kind"`    // "home" | "stays-domain" | "default-home" | "decides-at-first-backup" | "paused" | "not-backed-up"
	Home    string   `json:"home"`    // label of the home, "" for the domain path
	Targets []string `json:"targets"` // names of the targets it is copied to
	Warn    bool     `json:"warn"`
	Reason  string   `json:"reason"` // not-backed-up: "default-off" | "default-missing"
	NoCopy  bool     `json:"noCopy"`
}

type observedPlace struct {
	Place  string `json:"place"` // "local" | "offsite:<id>"
	Label  string `json:"label"`
	Count  int    `json:"count"`
	Latest int64  `json:"latest"`
	SeenAt int64  `json:"seenAt"` // when the item's copies were last seen there, 0 while it has none
	Stale  bool   `json:"stale"`
	State  string `json:"state"` // "counts" | "unreachable" | "unknown" | "old-copy" | "no-copy" | "off"
	Since  int64  `json:"since"`
	Counts bool   `json:"counts"`
}

type olderCopies struct {
	TargetID   string `json:"targetId"`
	Name       string `json:"name"`
	Count      int    `json:"count"`
	SeenAt     int64  `json:"seenAt"`
	AppendOnly bool   `json:"appendOnly"`
}

type placementObserved struct {
	NoBackup bool            `json:"noBackup"`
	Places   []observedPlace `json:"places"`
	Sites    int             `json:"sites"`
	Tone     string          `json:"tone"`    // "ok" | "warn" | "unconfirmed"
	Rule321  string          `json:"rule321"` // "met" | "one-copy" | "nothing-off-premises" | "unconfirmed"
	Older    []olderCopies   `json:"older"`
}

type stackNote struct {
	Project string   `json:"project"`
	Home    string   `json:"home"`
	Targets []string `json:"targets"`
}

// statusFacts is what the result line of every item in one list needs, read
// once per request.
type statusFacts struct {
	now        int64
	grace      int64 // how far a listing or a copy may lag before a target stops counting, 0 for the strict reading
	named      map[string]store.OffsiteTarget
	copies     map[string][]store.ItemCopies      // by identity
	observed   map[string]store.TargetObservation // by target id
	failedAt   map[string]int64                   // first failed run after the last listing, by target id
	domainTags func() (map[string]bool, error)    // tags at the local domain path, listed at most once
}

// statusFactsFor reads what every item of one list is judged against. It runs
// only behind a certain target list, so every target it asks about has a row
// and an id of its own.
func (s *Service) statusFactsFor(ctx context.Context, settings store.Settings, p placementRead, named map[string]store.OffsiteTarget) (*statusFacts, error) {
	f := &statusFacts{
		now:      time.Now().Unix(),
		grace:    s.statusGrace(settings, p.Domain),
		named:    named,
		copies:   map[string][]store.ItemCopies{},
		failedAt: map[string]int64{},
		domainTags: sync.OnceValues(func() (map[string]bool, error) {
			return s.domainPathTags(ctx, settings, p.Domain)
		}),
	}
	rows, err := s.store.ItemCopiesForDomain(p.Domain)
	if err != nil {
		return nil, err
	}
	for _, c := range rows {
		f.copies[c.Identity] = append(f.copies[c.Identity], c)
	}
	if f.observed, err = s.store.TargetObservationsForDomain(p.Domain); err != nil {
		return nil, err
	}
	for _, t := range p.Targets {
		at, err := s.store.FirstOffsiteFailureAfter(t.ID, f.observed[t.ID].ListedAt)
		if err != nil {
			return nil, err
		}
		f.failedAt[t.ID] = at
	}
	return f, nil
}

// statusGrace is how far a listing or a copy may lag: twice the domain's
// off-site cadence, else twice how often it is backed up, and nothing at all
// when it has neither.
func (s *Service) statusGrace(settings store.Settings, domain string) int64 {
	if period := cadencePeriodSeconds(s.offsiteScheduleFor(domain, settings)); period > 0 {
		return 2 * period
	}
	period, _ := domainCoverage(digestBackupScheduleFor(domain, settings), settings.EverythingSchedule)
	return 2 * period
}

func (f *statusFacts) label(repoID string) string {
	if repoID == "" {
		return ""
	}
	if r, ok := f.named[repoID]; ok {
		return r.Name
	}
	return repoID
}

// domainPathTags lists the local domain path and returns every tag found
// there, the question the step before a first backup asks.
func (s *Service) domainPathTags(ctx context.Context, settings store.Settings, domain string) (map[string]bool, error) {
	repo, err := s.repoFor(settings, domain, "local")
	if err != nil {
		return nil, err
	}
	snaps, err := s.listRepo(ctx, repo, s.primaryModeFor(settings, domain, repo))
	if err != nil {
		return nil, err
	}
	tags := map[string]bool{}
	for _, snap := range snaps {
		for _, tag := range snap.Tags {
			tags[tag] = true
		}
	}
	return tags, nil
}

// homeOffPremises reports whether a home is a site of its own: a remote
// domain path, a direct repository at its target, or a repository marked off
// the premises.
func homeOffPremises(kind homeKind, repoID string, named map[string]store.OffsiteTarget) bool {
	switch kind {
	case homeDomainRemote, homeDirect:
		return true
	case homeLocal, homeRemote:
		return named[repoID].OffPremises
	}
	return false
}

// placementCoverage answers what placement adds to a domain's status: whether
// anything is copied to an enabled target, and whether every item already lives
// off the premises. Without copy rules the first answer is yes, as it was before
// there were any.
func (s *Service) placementCoverage(settings store.Settings, domain string) (copied, offPremises bool, err error) {
	p, err := s.readPlacement(settings, domain)
	if err != nil {
		return false, false, err
	}
	named, err := s.namedRepoIndex()
	if err != nil {
		return false, false, err
	}
	homes, err := s.itemHomes(domain)
	if err != nil {
		return false, false, err
	}
	copied = !p.State.HasRules()
	offPremises = len(homes) > 0
	for identity, state := range homes {
		repoID, _ := p.effectiveHome(state)
		home := s.homeKindOf(settings, domain, repoID, named)
		if home.copySource() && len(p.effectiveTargets(identity)) > 0 {
			copied = true
		}
		if !homeOffPremises(home, repoID, named) {
			offPremises = false
		}
	}
	if copied {
		return true, offPremises, nil
	}
	observed, err := s.store.ItemCopiesForDomain(domain)
	if err != nil {
		return false, false, err
	}
	for _, c := range observed {
		if _, isItem := homes[c.Identity]; isItem {
			continue
		}
		if slices.ContainsFunc(p.effectiveTargets(c.Identity), func(t store.OffsiteTarget) bool { return t.ID == c.TargetID }) {
			return true, offPremises, nil
		}
	}
	return false, offPremises, nil
}

// itemHomes returns the home columns of every item row in a placement domain, by
// identity.
func (s *Service) itemHomes(domain string) (map[string]store.HomeState, error) {
	out := map[string]store.HomeState{}
	switch domain {
	case "containers":
		rows, err := s.store.ListTargets()
		if err != nil {
			return nil, err
		}
		for _, t := range rows {
			out["container:"+t.ContainerName] = store.HomeState{Exists: true, Repo: t.Repo, Choice: t.RepoChosen}
		}
	case "vms":
		rows, err := s.store.ListVMTargets()
		if err != nil {
			return nil, err
		}
		for _, v := range rows {
			out["vm:"+v.Name] = store.HomeState{Exists: true, Repo: v.Repo, Choice: v.RepoChosen}
		}
	case "files":
		rows, err := s.store.ListFileSets()
		if err != nil {
			return nil, err
		}
		for _, fs := range rows {
			out["fileset:"+fs.Name] = store.HomeState{Exists: true, Repo: fs.Repo, Choice: fs.RepoChosen}
		}
	}
	return out, nil
}

// stackNoteFor names the project folder when its copies differ from the member's.
// Project folders always land on the domain path and follow the containers default.
func stackNoteFor(p placementRead, item placementItem, plan *placementPlan) *stackNote {
	if item.Stack == "" || plan.Kind == "paused" {
		return nil
	}
	targets := []string{}
	for _, t := range p.effectiveTargets("stack:" + item.Stack) {
		targets = append(targets, placementTargetName(t))
	}
	if slices.Equal(targets, plan.Targets) {
		return nil
	}
	return &stackNote{Project: item.Stack, Home: "", Targets: targets}
}

// placementStatus is one item's result line. The home is resolved once here and
// handed to both halves, so the sentence cannot name one repository while the
// state scores another.
func (s *Service) placementStatus(settings store.Settings, p placementRead, item placementItem, f *statusFacts) (*placementPlan, *placementObserved) {
	kind, repoID := s.plannedHome(settings, p, item, f)
	home := s.homeKindOf(settings, p.Domain, repoID, f.named)
	return planFor(p, item, f, kind, repoID, home), observedState(p, item, f, repoID, home)
}

// plannedHome is where an item's next backup goes and how the plan phrases it:
// its own repository once chosen, otherwise what the default and the local
// domain path make of an open one.
func (s *Service) plannedHome(settings store.Settings, p placementRead, item placementItem, f *statusFacts) (kind, repoID string) {
	repoID, fromDefault := p.effectiveHome(item.Home)
	kind = "home"
	switch {
	case fromDefault && repoID != "":
		kind = s.openKind(settings, p.Domain, item.Identity, f)
	case fromDefault:
		kind = "default-home"
	}
	if kind == "stays-domain" {
		repoID = ""
	}
	return kind, repoID
}

// planFor is the plan half of the result line: where the next backup goes and
// where it is copied, asked the way the step before the first backup asks.
func planFor(p placementRead, item placementItem, f *statusFacts, kind, repoID string, home homeKind) *placementPlan {
	if kind == "default-home" && repoID != "" {
		switch {
		case home == homeMissing:
			return &placementPlan{Kind: "not-backed-up", Targets: []string{}, Warn: true, Reason: "default-missing"}
		case !f.named[repoID].Enabled:
			return &placementPlan{Kind: "not-backed-up", Home: f.label(repoID), Targets: []string{}, Warn: true, Reason: "default-off"}
		}
	}
	plan := &placementPlan{Kind: kind, Home: f.label(repoID), Targets: []string{}}
	if home.copySource() {
		if p.State.Paused() {
			return &placementPlan{Kind: "paused", Home: plan.Home, Targets: []string{}, Warn: true}
		}
		for _, t := range p.effectiveTargets(item.Identity) {
			plan.Targets = append(plan.Targets, placementTargetName(t))
		}
	}
	plan.NoCopy = kind != "decides-at-first-backup" && len(plan.Targets) == 0 && !homeOffPremises(home, repoID, f.named)
	plan.Warn = plan.NoCopy
	return plan
}

// openKind answers the first-backup question for an open item whose default
// names a repository: a remote or unreadable domain path cannot tell, a
// local one that holds the name keeps it, otherwise the default applies.
func (s *Service) openKind(settings store.Settings, domain, identity string, f *statusFacts) string {
	if s.homeKindOf(settings, domain, "", f.named) == homeDomainRemote {
		return "decides-at-first-backup"
	}
	tags, err := f.domainTags()
	switch {
	case err != nil:
		return "decides-at-first-backup"
	case tags[identity]:
		return "stays-domain"
	}
	return "default-home"
}

// observedState is the state half of the result line: the last successful
// backup at the home and what the targets held at their last listing.
func observedState(p placementRead, item placementItem, f *statusFacts, repoID string, home homeKind) *placementObserved {
	copies := f.copies[item.Identity]
	if item.LastSuccess == 0 && len(copies) == 0 {
		return &placementObserved{NoBackup: true, Places: []observedPlace{}, Tone: "warn", Rule321: "one-copy", Older: []olderCopies{}}
	}
	o := &placementObserved{Places: []observedPlace{}, Older: []olderCopies{}}

	local := observedPlace{Place: "local", Label: f.label(repoID), Latest: item.LastSuccess, SeenAt: item.LastSuccess, State: "unknown"}
	if item.LastSuccess > 0 {
		local.State, local.Counts = "counts", true
	}
	o.Places = append(o.Places, local)

	held := make(map[string]store.ItemCopies, len(copies))
	for _, c := range copies {
		held[c.TargetID] = c
	}
	ticked := tickedTargets(p, item.Identity, home)
	for _, t := range p.Targets {
		c, has := held[t.ID]
		switch {
		case !ticked[t.ID] && has:
			o.Older = append(o.Older, olderCopies{TargetID: t.ID, Name: placementTargetName(t), Count: c.SnapshotCount, SeenAt: c.ObservedAt, AppendOnly: t.Immutable})
		case ticked[t.ID] && (t.Enabled || has):
			o.Places = append(o.Places, f.targetPlace(t, c, has, item.LastSuccess))
		}
	}

	homeSite := "host"
	switch {
	case home == homeDirect:
		homeSite = offsiteSourcePrefix + f.named[repoID].CompanionOf
	case homeOffPremises(home, repoID, f.named):
		homeSite = "home"
	}
	o.score(homeSite)
	return o
}

// tickedTargets are the targets an item's rule copies to, switched off or not.
// An item whose home is not a copy source has none.
func tickedTargets(p placementRead, identity string, home homeKind) map[string]bool {
	ticked := map[string]bool{}
	if !home.copySource() {
		return ticked
	}
	skip, _ := p.resolvedSkip(identity)
	for _, t := range p.Targets {
		ticked[t.ID] = !skipsTarget(skip, t.ID)
	}
	return ticked
}

// targetPlace judges one ticked target. Switched off, a failed run since its
// last listing, a listing that lags by more than the grace or a newest copy
// that far behind the last backup each keep it from counting. A target whose
// last listing proved it holds nothing of the item carries no copy yet.
func (f *statusFacts) targetPlace(t store.OffsiteTarget, c store.ItemCopies, holds bool, lastBackup int64) observedPlace {
	obs, listed := f.observed[t.ID]
	pl := observedPlace{
		Place: offsiteSourcePrefix + t.ID, Label: placementTargetName(t),
		Count: c.SnapshotCount, Latest: c.LatestSnapshotAt, SeenAt: c.ObservedAt,
	}
	switch {
	case !t.Enabled:
		pl.State = "off"
	case f.failedAt[t.ID] > 0:
		pl.State, pl.Since = "unreachable", f.failedAt[t.ID]
	case !listed:
		pl.State, pl.Stale = "unknown", true
	case f.grace > 0 && f.now-obs.ListedAt > f.grace:
		pl.State, pl.Since, pl.Stale = "unknown", obs.ListedAt, true
	case !holds:
		pl.State = "no-copy"
	case lastBackup-c.LatestSnapshotAt > f.grace:
		pl.State, pl.Stale = "old-copy", true
	default:
		pl.State, pl.Counts = "counts", true
	}
	return pl
}

// score fills in sites, 3-2-1 and tone. The server holding the original data is
// always a site; a counting place off the premises adds its own.
func (o *placementObserved) score(homeSite string) {
	sites := map[string]bool{"host": true}
	counting, stale := 0, 0
	offSite, staleOffSite, unreachable := false, false, false
	for _, pl := range o.Places {
		site := pl.Place
		if pl.Place == "local" {
			site = homeSite
		}
		switch {
		case pl.Counts:
			counting++
			sites[site] = true
			offSite = offSite || site != "host"
		case pl.Stale:
			stale++
			staleOffSite = staleOffSite || site != "host"
		case pl.State == "unreachable":
			unreachable = true
		}
	}
	o.Sites = len(sites)
	switch {
	case counting >= 2 && offSite:
		o.Rule321 = "met"
	case counting+stale >= 2 && (offSite || staleOffSite):
		o.Rule321 = "unconfirmed"
	case counting < 2:
		o.Rule321 = "one-copy"
	default:
		o.Rule321 = "nothing-off-premises"
	}
	switch {
	case unreachable || o.Rule321 == "one-copy" || o.Rule321 == "nothing-off-premises":
		o.Tone = "warn"
	case o.Rule321 == "unconfirmed":
		o.Tone = "unconfirmed"
	default:
		o.Tone = "ok"
	}
}
