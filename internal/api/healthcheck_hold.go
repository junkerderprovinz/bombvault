package api

import (
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// healthcheckHoldNames is how many failed backups a held ping lists.
const healthcheckHoldNames = 5

// healthcheckHold names the failed backups a success on domain's check must not
// hide. Sections on one Ping URL share one check, and a section that succeeds
// after another failed would turn that check green with the failure unsolved.
// A backup counts until its entry has a newer good run or somebody acknowledged
// the failure under Needs you. Empty means the check may hear the success.
func (s *Service) healthcheckHold(c notify.Config, domain string) string {
	checks := c.Checks(domain)
	if len(checks) == 0 {
		return ""
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		log.Printf("api: healthchecks: read settings: %v", err)
		return ""
	}
	runs, err := s.store.LastRunsOfKind("backup")
	if err != nil {
		log.Printf("api: healthchecks: read the last runs: %v", err)
		return ""
	}
	var failed []string
	for _, section := range s.statusDomains(settings) {
		if !section.enabled || !slices.Equal(c.Checks(section.name), checks) {
			continue
		}
		for _, entry := range s.backupEntriesOf(section.name) {
			if run, found := runs[entry.id]; found && run.Status == "failed" && !run.Acknowledged {
				failed = append(failed, section.name+" "+entry.name)
			}
		}
	}
	if len(failed) == 0 {
		return ""
	}
	slices.Sort(failed)
	shown := failed[:min(len(failed), healthcheckHoldNames)]
	held := "Still failing, with no good backup since: " + strings.Join(shown, ", ")
	if more := len(failed) - len(shown); more > 0 {
		held += fmt.Sprintf(" (+%d more)", more)
	}
	return held
}

// backupEntry is an entry of a section by the id its runs carry.
type backupEntry struct {
	id, name string
}

// backupEntriesOf lists what a section backs up. Flash and the self-backup are
// their own one entry.
func (s *Service) backupEntriesOf(domain string) []backupEntry {
	var out []backupEntry
	var err error
	switch domain {
	case "containers":
		var ts []store.Target
		ts, err = s.store.ListTargets()
		for _, t := range ts {
			out = append(out, backupEntry{t.ID, t.ContainerName})
		}
	case "vms":
		var vs []store.VMTarget
		vs, err = s.store.ListVMTargets()
		for _, v := range vs {
			out = append(out, backupEntry{v.ID, v.Name})
		}
	case "files":
		var sets []store.FileSet
		sets, err = s.store.ListFileSets()
		for _, set := range sets {
			out = append(out, backupEntry{set.ID, set.Name})
		}
	case zfsDomain:
		var ds []store.ZFSDataset
		ds, err = s.store.ListZFSDatasets()
		for _, d := range ds {
			out = append(out, backupEntry{d.ID, d.Dataset})
		}
	case "flash":
		out = append(out, backupEntry{store.FlashTargetID, "backup"})
	case "config":
		out = append(out, backupEntry{store.ConfigTargetID, "backup"})
	}
	if err != nil {
		log.Printf("api: healthchecks: list %s: %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
	}
	return out
}
