package api

import (
	"context"
	"errors"
	"fmt"
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

// copySubjects are the identities a copy rule decides for: items whose location
// is a copy source, without their own rule unless withOwn, and for containers
// the project folders found in the copy sources or at a target.
func (s *Service) copySubjects(settings store.Settings, p placementRead, named map[string]store.OffsiteTarget, items []domainItem, listing sourceListing, observed []store.ItemCopies, withOwn bool) []string {
	var out []string
	for _, it := range items {
		if _, own := p.State.Rules[it.identity]; own && !withOwn {
			continue
		}
		if s.homeKindOf(settings, p.Domain, it.home.Repo, named).copySource() {
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
