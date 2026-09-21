package api

import (
	"cmp"
	"context"
	"log"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/store"
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

type newTargetRow struct {
	ID      string        `json:"id"`
	Domain  string        `json:"domain"`
	Name    string        `json:"name"`
	Preview targetPreview `json:"preview"`
}

// importNewTargets previews each enabled target a settings file adds, counted with
// the defaults and rules the file leaves behind.
func (s *Service) importNewTargets(ctx context.Context, exp settingsExport) []newTargetRow {
	rows := []newTargetRow{}
	settings, err := s.store.GetSettings()
	if err != nil {
		log.Printf("api: import preview: %v", err)
		return rows
	}
	for _, tv := range exp.OffsiteTargets {
		if !validPlacementDomain(tv.Domain) || !tv.Enabled {
			continue
		}
		if _, known, err := s.store.GetOffsiteTarget(tv.ID); err != nil || known {
			continue
		}
		from, err := s.importedPlacementFor(settings, exp, tv.Domain)
		var pv targetPreview
		if err == nil {
			pv, err = s.newTargetPreview(ctx, tv.Domain, tv.ID, &from)
		}
		if err != nil {
			log.Printf("api: import preview of %s: %v", tv.Domain, err) //nolint:gosec // G706: the domain passed validPlacementDomain
			continue
		}
		rows = append(rows, newTargetRow{ID: tv.ID, Domain: tv.Domain, Name: cmp.Or(tv.Name, scrubRepoLocation(tv.Repo)), Preview: pv})
	}
	return rows
}

// importedPlacementFor is a domain's placement as the file leaves it: the file's
// default and rules where it carries those blocks, this instance's otherwise.
// A paused domain keeps its row when the file does not name it, as the import does.
func (s *Service) importedPlacementFor(settings store.Settings, exp settingsExport, domain string) (placementRead, error) {
	p, err := s.readPlacement(settings, domain)
	if err != nil {
		return p, err
	}
	if exp.PlacementDefaults != nil {
		i := slices.IndexFunc(exp.PlacementDefaults, func(d placementDefaultExport) bool { return d.Domain == domain })
		switch {
		case i >= 0:
			d := exp.PlacementDefaults[i]
			p.State.HasDefault = true
			p.State.Default.Home, p.State.Default.Skip = d.Home, d.Skip
		case !p.State.Paused():
			p.State.HasDefault = false
			p.State.Default = store.PlacementDefault{Domain: domain}
		}
	}
	if exp.CopyRules != nil {
		rules := map[string]store.CopyRule{}
		for _, r := range exp.CopyRules {
			if r.Domain == domain {
				rules[r.Identity] = store.CopyRule{Domain: domain, Identity: r.Identity, Skip: r.Skip}
			}
		}
		p.State.Rules = rules
	}
	return p, nil
}

type placementExcludeBody struct {
	Domain     string   `json:"domain"`
	TargetID   string   `json:"targetId"`
	Field      bool     `json:"field"`
	Identities []string `json:"identities"`
	Default    bool     `json:"default"`
}

// handlePlacementExclude takes the new-target answer for writes whose body has no
// place for it: the off-site settings field and the settings import.
func (h *Handler) handlePlacementExclude(w http.ResponseWriter, r *http.Request) {
	var body placementExcludeBody
	if !decodeBody(w, r, &body) {
		return
	}
	if !validPlacementDomain(body.Domain) || body.Field == (body.TargetID != "") {
		placementFail(w, errInvalidPlacement, nil)
		return
	}
	targetID := body.TargetID
	if body.Field {
		row, found, err := h.store.FieldOffsiteTarget(body.Domain)
		if err != nil {
			placementFail(w, err, nil)
			return
		}
		if !found {
			placementFail(w, errNotATarget, nil)
			return
		}
		targetID = row.ID
	}
	ex := newTargetExclusion{Identities: body.Identities, Default: body.Default}
	if err := h.svc.excludeFromTarget(body.Domain, targetID, ex); err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
