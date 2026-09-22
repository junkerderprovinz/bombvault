package api

import (
	"context"
	"sync"

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
	SeenAt int64  `json:"seenAt"`
	Stale  bool   `json:"stale"`
	State  string `json:"state"` // "counts" | "unreachable" | "unknown" | "old-copy" | "off"
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
	named      map[string]store.OffsiteTarget
	domainTags func() (map[string]bool, error) // tags at the local domain path, listed at most once
}

func (s *Service) statusFactsFor(ctx context.Context, settings store.Settings, p placementRead, named map[string]store.OffsiteTarget) *statusFacts {
	return &statusFacts{
		named: named,
		domainTags: sync.OnceValues(func() (map[string]bool, error) {
			return s.domainPathTags(ctx, settings, p.Domain)
		}),
	}
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

func (s *Service) placementStatus(settings store.Settings, p placementRead, item placementItem, f *statusFacts) (*placementPlan, *placementObserved) {
	return s.planFor(settings, p, item, f), nil
}

// planFor is the plan half of the result line: where the next backup goes and
// where it is copied, asked the way the step before the first backup asks.
func (s *Service) planFor(settings store.Settings, p placementRead, item placementItem, f *statusFacts) *placementPlan {
	repoID, fromDefault := p.effectiveHome(item.Home)
	kind := "home"
	switch {
	case fromDefault && repoID != "":
		kind = s.openKind(settings, p.Domain, item.Identity, f)
	case fromDefault:
		kind = "default-home"
	}
	if kind == "stays-domain" {
		repoID = ""
	}
	home := s.homeKindOf(settings, p.Domain, repoID, f.named)
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
