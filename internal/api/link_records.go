package api

import (
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"slices"
	"sort"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// definitionAlias is one link as a mirrored definition records it: a former
// name of the entry and the time it was linked, in unix seconds. A VM link
// also carries the definition the entry had under that name, since the old
// name's own mirror is rewritten by whatever VM is backed up under it next.
type definitionAlias struct {
	Name           string `json:"name"`
	LinkedAt       int64  `json:"linked_at"`
	PrevDefinition string `json:"prev_definition,omitempty"`
}

// withAliasRecords returns defJSON with aliases as its link records and every
// other field as it was.
func withAliasRecords(defJSON []byte, aliases []store.Alias) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(defJSON, &fields); err != nil {
		return nil, fmt.Errorf("read the definition: %w", err)
	}
	delete(fields, "aliases")
	if len(aliases) > 0 {
		records := make([]definitionAlias, 0, len(aliases))
		for _, a := range aliases {
			records = append(records, definitionAlias{Name: a.OldName, LinkedAt: a.LinkedAt, PrevDefinition: a.PrevDefinition})
		}
		fields["aliases"], _ = json.Marshal(records)
	}
	return json.Marshal(fields)
}

// aliasOldNames is the old name of each alias, in order.
func aliasOldNames(aliases []store.Alias) []string {
	var names []string
	for _, a := range aliases {
		names = append(names, a.OldName)
	}
	return names
}

// writeDefMirror writes name's definition mirror for domain ("container" or
// "vm") beside repo, with aliases as its link records.
func (s *Service) writeDefMirror(domain string, settings store.Settings, name, repo, definition string, aliases []store.Alias) error {
	if domain == "vm" {
		return s.writeVMDefToStorage(settings, name, repo, []byte(definition), aliases)
	}
	return s.writeDefToStorage(settings, name, repo, []byte(definition), aliases)
}

// dropLinkRecords rewrites the definition mirror of name, which an entry is
// about to leave, without link records. It runs before the store changes and
// a failure stops the change, because a record that outlives its link makes
// Discover link the name again after a /config loss.
func (s *Service) dropLinkRecords(domain string, settings store.Settings, name, repo, definition string) error {
	if definition == "" {
		return nil
	}
	if err := s.writeDefMirror(domain, settings, name, repo, definition, nil); err != nil {
		return fmt.Errorf("the links recorded for %q on the backup storage could not be removed: %w", name, err)
	}
	return nil
}

// recordLinks writes the definition mirror of name, the name entry targetID
// answers to, with every alias of the entry as a link record. A failure is
// logged only: without the record a rebuild after a /config loss leaves the
// link to be redone by hand, and the entry's next backup writes it anyway.
func (s *Service) recordLinks(domain string, settings store.Settings, targetID, name, repo, definition string) {
	if definition == "" {
		return
	}
	aliases, err := s.store.TargetAliasesWithDefinitions(domain, targetID)
	if err == nil {
		err = s.writeDefMirror(domain, settings, name, repo, definition, aliases)
	}
	if err != nil {
		log.Printf("api: the links of %q could not be recorded on the backup storage; its next backup records them: %v", name, err) //nolint:gosec // G706: %q-quoted
	}
}

// formerNameClaim is the record, in owner's stored definition, that a name was
// owner's former name.
type formerNameClaim struct {
	owner  string
	record definitionAlias
}

// recordedFormerNames maps every name a stored definition records as a former
// name to the claims on it. aliasesOf returns the records in the stored
// definition of a discovered name, nil when it cannot be read.
func recordedFormerNames(names map[string]string, aliasesOf func(name, repoID string) []definitionAlias) map[string][]formerNameClaim {
	claims := map[string][]formerNameClaim{}
	for name, repoID := range names {
		for _, a := range aliasesOf(name, repoID) {
			claims[a.Name] = append(claims[a.Name], formerNameClaim{owner: name, record: a})
		}
	}
	return claims
}

// logUnrecordedFormerNames names every former name the backups' formerly:
// tags carry that no readable stored definition records, since Discover
// leaves such a name unlinked.
func logUnrecordedFormerNames(settingsDomain string, formerNames map[string][]string, claims map[string][]formerNameClaim) {
	namedBy := map[string][]string{}
	for cur, olds := range formerNames {
		for _, old := range olds {
			if len(claims[old]) == 0 {
				namedBy[old] = append(namedBy[old], cur)
			}
		}
	}
	for _, old := range slices.Sorted(maps.Keys(namedBy)) {
		curs := namedBy[old]
		sort.Strings(curs)
		log.Printf("api: discover %s: %q is a former name on the backups of %q, but no readable stored definition records the link, so it is not linked", settingsDomain, old, curs) //nolint:gosec // G706: names %q-quoted, the domain a fixed literal
	}
}
