package api

import (
	"context"
	"maps"
	"net/http"
	"slices"
	"strings"
)

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

// placementDomainQuery reads ?domain of a placement route that takes it as a
// query parameter rather than a path segment, and writes the 400 itself.
func placementDomainQuery(w http.ResponseWriter, r *http.Request) (string, bool) {
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	if !validPlacementDomain(domain) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return "", false
	}
	return domain, true
}

// newTargetPreview is what a target receives at its first run: every item whose
// location is a copy source and whose rule leaves it in, with the project folders
// for containers. targetID names the target when it already exists or comes from
// a settings file; from is that file's placement, nil for this instance's own.
func (s *Service) newTargetPreview(ctx context.Context, domain, targetID string, from *placementRead) (targetPreview, error) {
	pv := targetPreview{FormerlyExcluded: []excludedItem{}, Unreadable: []string{}}
	settings, err := s.store.GetSettings()
	if err != nil {
		return pv, err
	}
	p := from
	if p == nil {
		read, err := s.readPlacement(settings, domain)
		if err != nil {
			return pv, err
		}
		if targetID != "" && !containsTarget(read.Targets, targetID) {
			return pv, errNotATarget
		}
		p = &read
	}
	named, err := s.namedRepoIndex()
	if err != nil {
		return pv, err
	}
	items, err := s.domainItems(domain)
	if err != nil {
		return pv, err
	}
	listing, err := s.listCopySources(ctx, settings, domain)
	if err != nil {
		return pv, err
	}
	observed, err := s.store.ItemCopiesForDomain(domain)
	if err != nil {
		return pv, err
	}
	homes := make(map[string]string, len(items))
	for _, it := range items {
		homes[it.identity] = it.home.Repo
	}
	measured := true
	for _, id := range s.copySubjects(settings, *p, named, items, listing, observed, true) {
		skip, _ := p.resolvedSkip(id)
		if skipsEverything(skip) || slices.Contains(skip, targetID) {
			continue
		}
		pv.Items++
		pv.Snapshots += len(listing.ByIdentity[id])
		if s.homeKindOf(settings, domain, homes[id], named) == homeLocal {
			measured = false
		}
	}
	pv.Unreadable = append(pv.Unreadable, listing.Unreadable...)
	for _, id := range slices.Sorted(maps.Keys(p.State.Rules)) {
		skip := p.State.Rules[id].Skip
		if len(skip) > 0 && !skipsEverything(skip) && !slices.Contains(skip, targetID) {
			pv.FormerlyExcluded = append(pv.FormerlyExcluded, excludedItem{Identity: id, Skip: skip})
		}
	}
	skip := p.State.Default.Skip
	pv.DefaultExcludes = len(skip) > 0 && !skipsEverything(skip) && !slices.Contains(skip, targetID)
	if measured {
		stat, found, err := s.store.LatestRepoStat(domain, "local")
		if err != nil {
			return pv, err
		}
		if found {
			pv.Bytes = &stat.RawSize
		}
	}
	return pv, nil
}

func (h *Handler) handleNewTargetPreview(w http.ResponseWriter, r *http.Request) {
	domain, ok := placementDomainQuery(w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(r.URL.Query().Get("repo")) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "repo is required"})
		return
	}
	pv, err := h.svc.newTargetPreview(r.Context(), domain, r.URL.Query().Get("target"), nil)
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"preview": pv}))
}
