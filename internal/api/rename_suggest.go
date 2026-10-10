package api

import (
	"encoding/json"
	"maps"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// suggestRenames runs matchRenames over the not-installed targets, reading the
// template lineage once for the whole pass. notInstalled must already hold only
// not-installed targets. A target without a readable stored definition is
// skipped, since it has nothing to compare against.
func (s *Service) suggestRenames(live []dockercli.ContainerInfo, notInstalled []store.Target) map[string]RenameCandidate {
	orphans := make([]orphanEntry, 0, len(notInstalled))
	for _, t := range notInstalled {
		if t.Definition == "" {
			continue
		}
		var def containerDefinition
		if err := json.Unmarshal([]byte(t.Definition), &def); err != nil {
			continue
		}
		orphans = append(orphans, orphanEntry{
			Name:        t.ContainerName,
			Inspect:     def.Inspect,
			TemplateXML: def.TemplateXML,
		})
	}
	if len(orphans) == 0 {
		return nil // nothing to match, so spare the flash read
	}
	return matchRenames(live, orphans, s.readTemplateLineage(), s.cfg.DataRootSegments)
}

// renameHints offers each live container without a backup of its own the
// not-installed entry it looks renamed from. hasBackup holds the live
// containers that have one: they keep their own history and get no offer.
func (s *Service) renameHints(live []dockercli.ContainerInfo, notInstalled []store.Target, hasBackup map[string]bool) map[string]RenameCandidate {
	lacksBackup := func(c dockercli.ContainerInfo) bool { return !hasBackup[c.Name] }
	if len(notInstalled) == 0 || !slices.ContainsFunc(live, lacksBackup) {
		return nil
	}
	hints := s.suggestRenames(live, notInstalled)
	maps.DeleteFunc(hints, func(name string, _ RenameCandidate) bool { return hasBackup[name] })
	return hints
}
