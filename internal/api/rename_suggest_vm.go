package api

import (
	"context"
	"encoding/json"
	"log"

	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// suggestVMRenames is suggestRenames for ListVMs. VMInfo carries no UUID, so
// each live domain's UUID costs a virsh dumpxml. To keep the VM page cheap the
// pass runs only when needsSuggestion reports a live VM without backups of its
// own and at least one not-installed target has a known UUID.
//
// Every live domain is read, not only those without backups, because the
// one-to-one rule has to see the whole population or a UUID shared with a
// skipped VM would look unambiguous; ListVMs applies the per-row filter. A
// virsh or parse error drops that domain from the pass and never fails the
// list.
func (s *Service) suggestVMRenames(ctx context.Context, live []virshcli.VMInfo, notInstalled []store.VMTarget, needsSuggestion bool) map[string]RenameCandidate {
	if !needsSuggestion {
		return nil // every live VM already has its own backups: nothing to offer
	}
	orphanUUIDs := make(map[string]string, len(notInstalled))
	for _, t := range notInstalled {
		if vmDefinitionHasBlockDisks(t.Definition) {
			continue // TakeOverVM refuses block-device disks
		}
		if uuid := s.vmUUID(t); uuid != "" {
			orphanUUIDs[t.Name] = uuid
		}
	}
	if len(orphanUUIDs) == 0 {
		return nil // nothing to match, so spare the virsh calls
	}

	liveUUIDs := make(map[string]string, len(live))
	for _, vm := range live {
		xml, err := s.virsh.DumpXMLInactive(ctx, vm.Name)
		if err != nil {
			log.Printf("api: list vms: rename suggestion: dump inactive xml for %q: %v", vm.Name, err) //nolint:gosec // G706: %q-quoted
			continue
		}
		info, err := virshcli.ParseDomain(xml)
		if err != nil {
			log.Printf("api: list vms: rename suggestion: parse domain xml for %q: %v", vm.Name, err) //nolint:gosec // G706: %q-quoted
			continue
		}
		if info.UUID != "" {
			liveUUIDs[vm.Name] = info.UUID
		}
	}
	return matchVMRenamesByUUID(liveUUIDs, orphanUUIDs)
}

// vmDefinitionHasBlockDisks reports whether a stored VM definition declares a
// block-device (zvol) disk. Such a disk backs up under its own
// vm:<name>:zvol:<dev> tag, which no alias follows, so a takeover would carry
// the VM's history forward without its disk's. Unraid (file-backed vdisks) and
// TrueNAS 26 (the domain name is the UUID, so no libvirt rename) are
// unaffected.
//
// An empty definition, as on a freshly discovered row, has no block disks. One
// that does not parse has an unknown layout and counts as having them, so doubt
// excludes the target rather than offering it.
func vmDefinitionHasBlockDisks(definition string) bool {
	if definition == "" {
		return false
	}
	var def vmDefinition
	if err := json.Unmarshal([]byte(definition), &def); err != nil {
		return true
	}
	info, err := virshcli.ParseDomain(def.DomainXML)
	if err != nil {
		return true
	}
	return len(info.BlockDisks) > 0
}
