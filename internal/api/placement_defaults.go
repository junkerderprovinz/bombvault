package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

type defaultCounts struct {
	Follow         int `json:"follow"`         // items without an own rule
	Own            int `json:"own"`            // items with an own rule
	Open           int `json:"open"`           // repo_chosen = 0
	ChosenNoBackup int `json:"chosenNoBackup"` // chosen, no successful run
}

type defaultRow struct {
	Domain      string        `json:"domain"`
	Exists      bool          `json:"exists"`
	Home        string        `json:"home"`
	HomeKind    homeKind      `json:"homeKind"`
	HomeOff     bool          `json:"homeOff"`
	Skip        []string      `json:"skip"`
	Paused      bool          `json:"paused"`
	ConfirmedAt int64         `json:"confirmedAt"`
	Counts      defaultCounts `json:"counts"`
	Unreadable  bool          `json:"unreadable"`
}

// defaultImpact is what a default change does to the items that follow it.
type defaultImpact struct {
	Dropped      []targetImpact `json:"dropped"`
	Added        []targetImpact `json:"added"`
	OpenTakeHome int            `json:"openTakeHome"` // open items that take the new home at their first backup
}

type targetImpact struct {
	TargetID    string   `json:"targetId"`
	Name        string   `json:"name"`
	Items       int      `json:"items"`       // items and stack folders
	Snapshots   int      `json:"snapshots"`   // dropped: copies that stay there; added: at most this many uploaded
	Unknown     bool     `json:"unknown"`     // dropped: target never listed for the domain
	Uncheckable []string `json:"uncheckable"` // added: copy sources that could not be listed
}

// sameCounts compares what the question named, not the snapshot numbers: every
// backup changes those, and comparing them would refuse a PUT during any run.
func (d defaultImpact) sameCounts(o defaultImpact) bool {
	same := func(a, b []targetImpact) bool {
		return slices.EqualFunc(a, b, func(x, y targetImpact) bool {
			return x.TargetID == y.TargetID && x.Items == y.Items
		})
	}
	return d.OpenTakeHome == o.OpenTakeHome && same(d.Dropped, o.Dropped) && same(d.Added, o.Added)
}

type defaultChange struct {
	Home   *string        `json:"home"`
	Skip   *[]string      `json:"skip"`
	Expect *defaultImpact `json:"expect"`
}

// domainItem is one row of a domain as the defaults card and its buttons see it.
type domainItem struct {
	ref      store.ItemRef
	rowID    string
	label    string
	identity string
	home     store.HomeState
}

func (s *Service) domainItems(domain string) ([]domainItem, error) {
	var out []domainItem
	switch domain {
	case "containers":
		rows, err := s.store.ListTargets()
		if err != nil {
			return nil, err
		}
		for _, t := range rows {
			out = append(out, domainItem{
				ref: store.ItemRef{Domain: domain, Key: t.ContainerName}, rowID: t.ID, label: t.ContainerName,
				identity: "container:" + t.ContainerName,
				home:     store.HomeState{Exists: true, Repo: t.Repo, Choice: t.RepoChosen},
			})
		}
	case "vms":
		rows, err := s.store.ListVMTargets()
		if err != nil {
			return nil, err
		}
		for _, v := range rows {
			out = append(out, domainItem{
				ref: store.ItemRef{Domain: domain, Key: v.Name}, rowID: v.ID, label: v.Name,
				identity: "vm:" + v.Name,
				home:     store.HomeState{Exists: true, Repo: v.Repo, Choice: v.RepoChosen},
			})
		}
	case "files":
		rows, err := s.store.ListFileSets()
		if err != nil {
			return nil, err
		}
		for _, fs := range rows {
			out = append(out, domainItem{
				ref: store.ItemRef{Domain: domain, Key: fs.ID}, rowID: fs.ID, label: fs.Name,
				identity: "fileset:" + fs.Name,
				home:     store.HomeState{Exists: true, Repo: fs.Repo, Choice: fs.RepoChosen},
			})
		}
	}
	return out, nil
}

// placementDefaultRows is the three defaults as the card shows them. A domain
// whose placement cannot be read comes back marked unreadable.
func (s *Service) placementDefaultRows() ([]defaultRow, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, err
	}
	named, err := s.namedRepoIndex()
	if err != nil {
		return nil, err
	}
	rows := make([]defaultRow, 0, len(store.PlacementDomains))
	for _, domain := range store.PlacementDomains {
		row, err := s.defaultRowFor(settings, named, domain)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *Service) defaultRowFor(settings store.Settings, named map[string]store.OffsiteTarget, domain string) (defaultRow, error) {
	row := defaultRow{Domain: domain, Skip: []string{}}
	p, err := s.readPlacement(settings, domain)
	if err != nil {
		log.Printf("api: placement %s: could not read its placement, marking the default row unreadable: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		row.Unreadable = true
		return row, nil
	}
	d := p.State.Default
	repo, known := named[d.Home]
	row.Exists = p.State.HasDefault
	row.Home = d.Home
	row.HomeKind = s.homeKindOf(settings, domain, d.Home, named)
	row.HomeOff = known && !repo.Enabled
	row.Skip = append(row.Skip, d.Skip...)
	row.Paused = p.State.Paused()
	row.ConfirmedAt = d.ConfirmedAt
	row.Counts, err = s.defaultCountsFor(p)
	return row, err
}

func (s *Service) defaultCountsFor(p placementRead) (defaultCounts, error) {
	var c defaultCounts
	items, err := s.domainItems(p.Domain)
	if err != nil {
		return c, err
	}
	for _, it := range items {
		if _, own := p.State.Rules[it.identity]; own {
			c.Own++
		} else {
			c.Follow++
		}
		if it.home.Choice == store.RepoOpen {
			c.Open++
			continue
		}
		run, err := s.store.LastSuccessfulBackup(it.rowID)
		if err != nil {
			return c, err
		}
		if run == nil {
			c.ChosenNoBackup++
		}
	}
	return c, nil
}

// checkDefaultChange refuses a default the card could not have offered: a
// repository that cannot take backups, or a skip that is malformed or names a
// target of another domain.
func (s *Service) checkDefaultChange(p placementRead, change defaultChange) error {
	if change.Home != nil {
		if err := s.validateItemRepoID(strings.TrimSpace(*change.Home)); err != nil {
			return fmt.Errorf("%w: %w", errRepoInvalid, err)
		}
	}
	if change.Skip != nil {
		return checkSkip(p, *change.Skip)
	}
	return nil
}

func (s *Service) defaultImpactFor(ctx context.Context, domain string, change defaultChange) (defaultImpact, error) {
	impact := defaultImpact{Dropped: []targetImpact{}, Added: []targetImpact{}}
	settings, err := s.store.GetSettings()
	if err != nil {
		return impact, err
	}
	p, err := s.readPlacement(settings, domain)
	if err != nil {
		return impact, err
	}
	if err := s.checkDefaultChange(p, change); err != nil {
		return impact, err
	}
	named, err := s.namedRepoIndex()
	if err != nil {
		return impact, err
	}
	if s.setsOnlyHome(settings, domain, named, change) {
		change.Skip = nil
	}
	items, err := s.domainItems(domain)
	if err != nil {
		return impact, err
	}
	if change.Home != nil && strings.TrimSpace(*change.Home) != p.State.Default.Home {
		if impact.OpenTakeHome, err = s.openTakers(ctx, settings, domain, items); err != nil {
			return impact, err
		}
	}
	if change.Skip == nil {
		return impact, nil
	}
	listing, err := s.listCopySources(ctx, settings, domain)
	if err != nil {
		return impact, err
	}
	observed, err := s.store.ItemCopiesForDomain(domain)
	if err != nil {
		return impact, err
	}
	listed, err := s.store.TargetObservationsForDomain(domain)
	if err != nil {
		return impact, err
	}
	next := p.withDefaultSkip(*change.Skip)
	dropped, added := map[string]*targetImpact{}, map[string]*targetImpact{}
	for _, id := range s.copySubjects(settings, p, named, items, listing, observed, false) {
		before, after := p.effectiveTargets(id), next.effectiveTargets(id)
		for _, t := range before {
			if !containsTarget(after, t.ID) {
				ti := impactFor(dropped, t)
				ti.Items++
				ti.Snapshots += observedCount(observed, id, t.ID)
			}
		}
		for _, t := range after {
			if !containsTarget(before, t.ID) {
				ti := impactFor(added, t)
				ti.Items++
				ti.Snapshots += listing.uploadEstimate(id, observedFor(observed, id), t.ID)
			}
		}
	}
	for _, t := range p.Targets {
		if ti, ok := dropped[t.ID]; ok {
			_, known := listed[t.ID]
			ti.Unknown = !known
			impact.Dropped = append(impact.Dropped, *ti)
		}
		if ti, ok := added[t.ID]; ok {
			ti.Uncheckable = append(ti.Uncheckable, listing.Unreadable...)
			impact.Added = append(impact.Added, *ti)
		}
	}
	return impact, nil
}

// setsOnlyHome reports whether change moves the default onto a remote named
// repository. Such a change sets only the location; the copy rule stays for
// the items whose location is a copy source.
func (s *Service) setsOnlyHome(settings store.Settings, domain string, named map[string]store.OffsiteTarget, change defaultChange) bool {
	if change.Home == nil {
		return false
	}
	return s.homeKindOf(settings, domain, strings.TrimSpace(*change.Home), named) == homeRemote
}

// copySubjects are the identities a copy rule decides for: items whose
// effective home is a copy source, without their own rule unless withOwn, and
// for containers the project folders found in the copy sources or at a
// target. An open item takes the default's home, not its own empty repo
// field, so it is judged by where its next backup actually lands.
func (s *Service) copySubjects(settings store.Settings, p placementRead, named map[string]store.OffsiteTarget, items []domainItem, listing sourceListing, observed []store.ItemCopies, withOwn bool) []string {
	var out []string
	for _, it := range items {
		if _, own := p.State.Rules[it.identity]; own && !withOwn {
			continue
		}
		repo, _ := p.effectiveHome(it.home)
		if s.homeKindOf(settings, p.Domain, repo, named).copySource() {
			out = append(out, it.identity)
		}
	}
	if p.Domain != "containers" {
		return out
	}
	stacks := map[string]bool{}
	for id := range listing.ByIdentity {
		if strings.HasPrefix(id, "stack:") {
			stacks[id] = true
		}
	}
	for _, c := range observed {
		if strings.HasPrefix(c.Identity, "stack:") {
			stacks[c.Identity] = true
		}
	}
	return append(out, slices.Sorted(maps.Keys(stacks))...)
}

// openTakers counts the open items that take the default's location at their
// first backup: those without snapshots of their name on a local domain path.
// A remote domain path is not listed here, so every open item counts there.
func (s *Service) openTakers(ctx context.Context, settings store.Settings, domain string, items []domainItem) (int, error) {
	var open []domainItem
	for _, it := range items {
		if it.home.Choice == store.RepoOpen {
			open = append(open, it)
		}
	}
	if len(open) == 0 {
		return 0, nil
	}
	repo, err := s.repoFor(settings, domain, "local")
	if err != nil {
		return 0, err
	}
	if restic.IsRemoteRepo(repo) || localRepoMissing(repo) {
		return len(open), nil
	}
	snaps, err := s.listSnapshots(ctx, repo, s.primaryModeFor(settings, domain, repo))
	if err != nil {
		return len(open), nil //nolint:nilerr // an unreadable domain path decides nothing, so every open item may take the home
	}
	held := map[string]bool{}
	for _, sn := range snaps {
		for _, tag := range sn.Tags {
			held[tag] = true
		}
	}
	n := 0
	for _, it := range open {
		if !held[it.identity] {
			n++
		}
	}
	return n, nil
}

func impactFor(m map[string]*targetImpact, t store.OffsiteTarget) *targetImpact {
	if ti, ok := m[t.ID]; ok {
		return ti
	}
	ti := &targetImpact{TargetID: t.ID, Name: placementTargetName(t), Uncheckable: []string{}}
	m[t.ID] = ti
	return ti
}

func observedFor(observed []store.ItemCopies, identity string) []store.ItemCopies {
	var out []store.ItemCopies
	for _, c := range observed {
		if c.Identity == identity {
			out = append(out, c)
		}
	}
	return out
}

func observedCount(observed []store.ItemCopies, identity, targetID string) int {
	for _, c := range observed {
		if c.Identity == identity && c.TargetID == targetID {
			return c.SnapshotCount
		}
	}
	return 0
}

// putDefault writes a default once the numbers the card confirmed still hold,
// recounted under placementMu so no other rule or default write slips between.
func (s *Service) putDefault(ctx context.Context, domain string, change defaultChange) (defaultRow, defaultImpact, error) {
	s.placementMu.Lock()
	defer s.placementMu.Unlock()
	impact, err := s.defaultImpactFor(ctx, domain, change)
	if err != nil {
		return defaultRow{}, impact, err
	}
	expect := defaultImpact{}
	if change.Expect != nil {
		expect = *change.Expect
	}
	if !impact.sameCounts(expect) {
		return defaultRow{}, impact, errPlacementStale
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return defaultRow{}, impact, err
	}
	named, err := s.namedRepoIndex()
	if err != nil {
		return defaultRow{}, impact, err
	}
	current, _, err := s.store.PlacementDefaultFor(domain)
	if err != nil {
		return defaultRow{}, impact, err
	}
	home, skip := current.Home, current.Skip
	if change.Home != nil {
		home = strings.TrimSpace(*change.Home)
	}
	if change.Skip != nil && !s.setsOnlyHome(settings, domain, named, change) {
		skip = *change.Skip
	}
	if _, err := s.store.PutPlacementDefault(domain, home, skip); err != nil {
		return defaultRow{}, impact, err
	}
	for _, t := range impact.Dropped {
		if t.Unknown {
			s.listTargetInBackground(domain, t.TargetID)
		}
	}
	row, err := s.defaultRowFor(settings, named, domain)
	return row, impact, err
}

// applyCandidate is one item the "apply to entries without backups" button
// would reset, with what it would give up and pick up.
type applyCandidate struct {
	Key       string           `json:"key"`
	Label     string           `json:"label"`
	LosesHome bool             `json:"losesHome"`
	LosesRule bool             `json:"losesRule"`
	Uploads   []uploadEstimate `json:"uploads"`
}

// keptItem is one item the button leaves alone, with why.
type keptItem struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Reason string `json:"reason"` // "has-backups" | "unreadable" | "changed"
}

// keptReason is why the button leaves an item alone, "" when it may reset it.
func (s *Service) keptReason(ctx context.Context, item store.ItemRef) (string, error) {
	presence, err := s.itemBackups(ctx, item)
	switch {
	case presence == backupsUnreadable:
		return "unreadable", nil
	case err != nil:
		return "", err
	case presence == backupsPresent:
		return "has-backups", nil
	}
	return "", nil
}

// resetCopyTargets is the item's copy targets before and after the reset
// applyDefault makes: open, with no rule of its own. Judged by effective home
// rather than the row's raw repo field, since an open item already takes the
// default's location, not its own empty one.
func (s *Service) resetCopyTargets(settings store.Settings, p placementRead, named map[string]store.OffsiteTarget, it domainItem) (before, after []store.OffsiteTarget) {
	beforeRepo, _ := p.effectiveHome(it.home)
	before = s.itemCopyTargets(settings, p, named, beforeRepo, it.identity)
	after = s.itemCopyTargets(settings, p.withCopies(it.identity, &store.CopiesWrite{Follow: true}), named, p.State.Default.Home, it.identity)
	return before, after
}

// applyDefaultPreview is every item the button would touch: what it would
// reset, and what it leaves alone because it has backups, sits at a location
// that could not be read, or has neither its own rule nor a chosen home.
func (s *Service) applyDefaultPreview(ctx context.Context, domain string) ([]applyCandidate, []keptItem, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, nil, err
	}
	p, err := s.readPlacement(settings, domain)
	if err != nil {
		return nil, nil, err
	}
	named, err := s.namedRepoIndex()
	if err != nil {
		return nil, nil, err
	}
	items, err := s.domainItems(domain)
	if err != nil {
		return nil, nil, err
	}
	var listing *sourceListing
	var observed []store.ItemCopies
	reset, kept := []applyCandidate{}, []keptItem{}
	for _, it := range items {
		rule, own := p.State.Rules[it.identity]
		if it.home.Choice == store.RepoOpen && !own {
			continue
		}
		why, err := s.keptReason(ctx, it.ref)
		if err != nil {
			return nil, nil, err
		}
		if why != "" {
			kept = append(kept, keptItem{Key: it.ref.Key, Label: it.label, Reason: why})
			continue
		}
		c := applyCandidate{
			Key:       it.ref.Key,
			Label:     it.label,
			LosesHome: it.home.Choice != store.RepoOpen && it.home.Repo != p.State.Default.Home,
			LosesRule: own && !slices.Equal(rule.Skip, p.State.Default.Skip),
			Uploads:   []uploadEstimate{},
		}
		before, after := s.resetCopyTargets(settings, p, named, it)
		for _, t := range after {
			if containsTarget(before, t.ID) {
				continue
			}
			if listing == nil {
				l, err := s.listCopySources(ctx, settings, domain)
				if err != nil {
					return nil, nil, err
				}
				if observed, err = s.store.ItemCopiesForDomain(domain); err != nil {
					return nil, nil, err
				}
				listing = &l
			}
			c.Uploads = append(c.Uploads, uploadEstimate{
				TargetID:    t.ID,
				Name:        placementTargetName(t),
				Snapshots:   listing.uploadEstimate(it.identity, observedFor(observed, it.identity), t.ID),
				Uncheckable: append([]string{}, listing.Unreadable...),
			})
		}
		reset = append(reset, c)
	}
	return reset, kept, nil
}

// applyDefault resets both axes of the named items that still have no backups.
// Each write compares against the row as read under the lock, so an item that
// changed since is reported instead of overwritten.
func (s *Service) applyDefault(ctx context.Context, domain string, keys []string) ([]string, []keptItem, error) {
	unlock, ok := s.tryLockDomainFor(domain, placementLockReason)
	if !ok {
		return nil, nil, errPlacementBusy
	}
	defer unlock()
	s.placementMu.Lock()
	defer s.placementMu.Unlock()

	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, nil, err
	}
	p, err := s.readPlacement(settings, domain)
	if err != nil {
		return nil, nil, err
	}
	named, err := s.namedRepoIndex()
	if err != nil {
		return nil, nil, err
	}
	items, err := s.domainItems(domain)
	if err != nil {
		return nil, nil, err
	}
	byKey := make(map[string]domainItem, len(items))
	for _, it := range items {
		byKey[it.ref.Key] = it
	}
	reset, kept := []string{}, []keptItem{}
	for _, key := range keys {
		it, found := byKey[key]
		if !found {
			kept = append(kept, keptItem{Key: key, Label: key, Reason: "changed"})
			continue
		}
		why, err := s.keptReason(ctx, it.ref)
		if err != nil {
			return nil, nil, err
		}
		if why != "" {
			kept = append(kept, keptItem{Key: key, Label: it.label, Reason: why})
			continue
		}
		follow := &store.CopiesWrite{Follow: true}
		before, after := s.resetCopyTargets(settings, p, named, it)
		dropped, err := s.droppedTargets(domain, it.identity, before, after)
		if err != nil {
			return nil, nil, err
		}
		wrote, err := s.store.WritePlacement(it.ref, &store.HomeWrite{Choice: store.RepoOpen}, follow, &it.home)
		if err != nil {
			return nil, nil, err
		}
		if !wrote {
			kept = append(kept, keptItem{Key: key, Label: it.label, Reason: "changed"})
			continue
		}
		reset = append(reset, key)
		for _, d := range dropped {
			if d.Copies == nil {
				s.listTargetInBackground(domain, d.TargetID)
			}
		}
	}
	return reset, kept, nil
}

type excludedItem struct {
	Identity string   `json:"identity"`
	Skip     []string `json:"skip"`
}

// targetPreview is what a target receives at its next run.
type targetPreview struct {
	Items            int            `json:"items"`
	FormerlyExcluded []excludedItem `json:"formerlyExcluded"`
	DefaultExcludes  bool           `json:"defaultExcludes"`
	Snapshots        int            `json:"snapshots"`
	Bytes            *int64         `json:"bytes"`
	Unreadable       []string       `json:"unreadable"`
}

type targetPreviewRow struct {
	TargetID string        `json:"targetId"`
	Name     string        `json:"name"`
	Preview  targetPreview `json:"preview"`
}

type unmatchedName struct {
	Identity  string `json:"identity"`
	Snapshots int    `json:"snapshots"`
}

// confirmPreview is what the domain copies once its default is confirmed, per
// enabled target, and the names in its copy sources that no row knows.
func (s *Service) confirmPreview(ctx context.Context, domain string) ([]targetPreviewRow, []unmatchedName, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, nil, err
	}
	p, err := s.readPlacement(settings, domain)
	if err != nil {
		return nil, nil, err
	}
	named, err := s.namedRepoIndex()
	if err != nil {
		return nil, nil, err
	}
	items, err := s.domainItems(domain)
	if err != nil {
		return nil, nil, err
	}
	listing, err := s.listCopySources(ctx, settings, domain)
	if err != nil {
		return nil, nil, err
	}
	observed, err := s.store.ItemCopiesForDomain(domain)
	if err != nil {
		return nil, nil, err
	}
	subjects := s.copySubjects(settings, p, named, items, listing, observed, true)
	rules := slices.SortedFunc(maps.Values(p.State.Rules), func(a, b store.CopyRule) int { return strings.Compare(a.Identity, b.Identity) })
	rows := []targetPreviewRow{}
	for _, t := range p.enabledTargets() {
		pv := targetPreview{FormerlyExcluded: []excludedItem{}, Unreadable: append([]string{}, listing.Unreadable...)}
		for _, id := range subjects {
			if containsTarget(p.effectiveTargets(id), t.ID) {
				pv.Items++
				pv.Snapshots += listing.uploadEstimate(id, observedFor(observed, id), t.ID)
			}
		}
		for _, r := range rules {
			if len(r.Skip) > 0 && !skipsEverything(r.Skip) && !slices.Contains(r.Skip, t.ID) {
				pv.FormerlyExcluded = append(pv.FormerlyExcluded, excludedItem{Identity: r.Identity, Skip: r.Skip})
			}
		}
		skip := p.State.Default.Skip
		pv.DefaultExcludes = len(skip) > 0 && !skipsEverything(skip) && !slices.Contains(skip, t.ID)
		rows = append(rows, targetPreviewRow{TargetID: t.ID, Name: placementTargetName(t), Preview: pv})
	}
	return rows, unmatchedNames(p, items, listing), nil
}

// unmatchedNames are the identities in the copy sources that no row and no rule
// knows, with their snapshot counts. Project folders follow the default and are
// never listed.
func unmatchedNames(p placementRead, items []domainItem, listing sourceListing) []unmatchedName {
	known := map[string]bool{}
	for _, it := range items {
		known[it.identity] = true
	}
	for id := range p.State.Rules {
		known[id] = true
	}
	out := []unmatchedName{}
	for id, snaps := range listing.ByIdentity {
		if !known[id] && !strings.HasPrefix(id, "stack:") {
			out = append(out, unmatchedName{Identity: id, Snapshots: len(snaps)})
		}
	}
	slices.SortFunc(out, func(a, b unmatchedName) int { return strings.Compare(a.Identity, b.Identity) })
	return out
}

// confirmDefault ends the domain's pause and leaves each excluded name out of
// every target with a rule of its own. A domain without a default is left
// untouched: there is nothing yet for an exclusion to answer to. A domain
// that already has one but was never paused still gets its exclusions
// written, since the confirm route is the only place they are, but keeps its
// pause and confirmed state exactly as they are, so confirming a healthy
// domain still cannot take the rebuild check out of service.
func (s *Service) confirmDefault(_ context.Context, domain string, exclude []string) error {
	prefix := domainTagPrefix(domain)
	for _, id := range exclude {
		if !strings.HasPrefix(id, prefix) || id == prefix {
			return errInvalidPlacement
		}
	}
	s.placementMu.Lock()
	defer s.placementMu.Unlock()
	d, found, err := s.store.PlacementDefaultFor(domain)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if !d.Paused() {
		return s.store.SetExcludeRules(domain, exclude)
	}
	return s.store.ConfirmPlacement(domain, exclude)
}

func (h *Handler) handleConfirmPreview(w http.ResponseWriter, r *http.Request) {
	domain, ok := defaultDomainParam(w, r)
	if !ok {
		return
	}
	settings, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	p, err := h.svc.readPlacement(settings, domain)
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	targets, unmatched, err := h.svc.confirmPreview(r.Context(), domain)
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"paused": p.State.Paused(), "targets": targets, "unmatched": unmatched}))
}

func (h *Handler) handleConfirmDefault(w http.ResponseWriter, r *http.Request) {
	domain, ok := defaultDomainParam(w, r)
	if !ok {
		return
	}
	var body struct {
		Exclude []string `json:"exclude"`
	}
	if !decodeOptionalBody(w, r, &body) {
		return
	}
	if err := h.svc.confirmDefault(r.Context(), domain, body.Exclude); err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// defaultDomainParam reads {domain} of a /api/placement/default route and writes
// the 400 itself.
func defaultDomainParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	domain := r.PathValue("domain")
	if !validPlacementDomain(domain) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return "", false
	}
	return domain, true
}

func (h *Handler) handleListPlacementDefaults(w http.ResponseWriter, _ *http.Request) {
	rows, err := h.svc.placementDefaultRows()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"defaults": rows}))
}

func (h *Handler) handlePreviewPlacementDefault(w http.ResponseWriter, r *http.Request) {
	domain, ok := defaultDomainParam(w, r)
	if !ok {
		return
	}
	var change defaultChange
	if !decodeBody(w, r, &change) {
		return
	}
	impact, err := h.svc.defaultImpactFor(r.Context(), domain, change)
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"impact": impact}))
}

func (h *Handler) handlePutPlacementDefault(w http.ResponseWriter, r *http.Request) {
	domain, ok := defaultDomainParam(w, r)
	if !ok {
		return
	}
	var change defaultChange
	if !decodeBody(w, r, &change) {
		return
	}
	row, impact, err := h.svc.putDefault(r.Context(), domain, change)
	if errors.Is(err, errPlacementStale) {
		placementFail(w, err, map[string]any{"impact": impact})
		return
	}
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"default": row, "impact": impact}))
}

func (h *Handler) handleApplyDefaultPreview(w http.ResponseWriter, r *http.Request) {
	domain, ok := defaultDomainParam(w, r)
	if !ok {
		return
	}
	reset, kept, err := h.svc.applyDefaultPreview(r.Context(), domain)
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"reset": reset, "kept": kept}))
}

func (h *Handler) handleApplyDefault(w http.ResponseWriter, r *http.Request) {
	domain, ok := defaultDomainParam(w, r)
	if !ok {
		return
	}
	var body struct {
		Keys []string `json:"keys"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	reset, kept, err := h.svc.applyDefault(r.Context(), domain, body.Keys)
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"reset": reset, "kept": kept}))
}

type placementDefaultExport struct {
	Domain string   `json:"domain"`
	Home   string   `json:"home"`
	Skip   []string `json:"skip"`
}

type copyRuleExport struct {
	Domain   string   `json:"domain"`
	Identity string   `json:"identity"`
	Skip     []string `json:"skip"`
}

func placementDefaultsToExport(rows []store.PlacementDefault) []placementDefaultExport {
	out := make([]placementDefaultExport, 0, len(rows))
	for _, d := range rows {
		out = append(out, placementDefaultExport{Domain: d.Domain, Home: d.Home, Skip: append([]string{}, d.Skip...)})
	}
	return out
}

func copyRulesToExport(rules []store.CopyRule) []copyRuleExport {
	out := make([]copyRuleExport, 0, len(rules))
	for _, r := range rules {
		out = append(out, copyRuleExport{Domain: r.Domain, Identity: r.Identity, Skip: append([]string{}, r.Skip...)})
	}
	return out
}

// importedPlacement is the file's two blocks for the store. A nil block is one
// the file does not carry, which leaves its table alone.
func importedPlacement(exp settingsExport) store.PlacementImport {
	in := store.PlacementImport{HasDefaults: exp.PlacementDefaults != nil, HasRules: exp.CopyRules != nil}
	for _, d := range exp.PlacementDefaults {
		in.Defaults = append(in.Defaults, store.PlacementDefault{Domain: d.Domain, Home: strings.TrimSpace(d.Home), Skip: d.Skip})
	}
	for _, r := range exp.CopyRules {
		in.Rules = append(in.Rules, store.CopyRule{Domain: r.Domain, Identity: r.Identity, Skip: r.Skip})
	}
	return in
}

// checkImportedPlacement refuses the defaults and rules an import could not
// write whole, before anything of the file is written: one entry per domain,
// a skip that names real targets, and a home that could take a backup, all
// checked the way the PUT route checks them, but against the repositories and
// targets the file itself is about to create as well as the stored ones.
func (h *Handler) checkImportedPlacement(exp settingsExport) error {
	inFileRepo := make(map[string]bool, len(exp.NamedRepos))
	for _, tv := range exp.NamedRepos {
		inFileRepo[strings.TrimSpace(tv.ID)] = true
	}
	inFileTarget := make(map[string]map[string]bool, len(exp.OffsiteTargets))
	for _, tv := range exp.OffsiteTargets {
		if inFileTarget[tv.Domain] == nil {
			inFileTarget[tv.Domain] = map[string]bool{}
		}
		inFileTarget[tv.Domain][strings.TrimSpace(tv.ID)] = true
	}
	settings, err := h.store.GetSettings()
	if err != nil {
		return err
	}
	placements := map[string]placementRead{}
	seenDomain := map[string]bool{}
	for _, d := range exp.PlacementDefaults {
		if !validPlacementDomain(d.Domain) || !validSkip(d.Skip) {
			return errInvalidPlacement
		}
		if seenDomain[d.Domain] {
			return errInvalidPlacement
		}
		seenDomain[d.Domain] = true
		p, ok := placements[d.Domain]
		if !ok {
			if p, err = h.svc.readPlacement(settings, d.Domain); err != nil {
				return err
			}
			placements[d.Domain] = p
		}
		for _, id := range d.Skip {
			if id == store.SkipAll || containsTarget(p.Targets, id) || inFileTarget[d.Domain][id] {
				continue
			}
			return errNotATarget
		}
		home := strings.TrimSpace(d.Home)
		if home == "" || inFileRepo[home] {
			continue
		}
		if _, err := h.store.GetNamedRepo(home); errors.Is(err, sql.ErrNoRows) {
			return errDefaultRepoMissing
		} else if err != nil {
			return err
		}
		if err := h.svc.validateItemRepoID(home); err != nil {
			return fmt.Errorf("%w: %w", errRepoInvalid, err)
		}
	}
	for _, r := range exp.CopyRules {
		if err := store.CheckRuleIdentity(r.Domain, r.Identity); err != nil {
			return err
		}
		if !validSkip(r.Skip) {
			return errInvalidPlacement
		}
	}
	return nil
}

// countIfPresent is the preview count of a block, nil when the file lacks it.
func countIfPresent[T any](block []T) *int {
	if block == nil {
		return nil
	}
	n := len(block)
	return &n
}
