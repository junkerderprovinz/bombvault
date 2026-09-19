package api

// matchVMRenamesByUUID is matchRenames for VMs. Unraid undefines and redefines
// a domain with the same libvirt UUID when it is renamed, so a live domain and
// a not-installed target sharing a UUID are the same VM.
//
// liveUUIDs maps live libvirt names (not display names) to UUIDs, orphanUUIDs
// maps not-installed target names to UUIDs, and the result is keyed by the
// live libvirt name. Matches are one-to-one as in matchRenames. An empty UUID
// never matches: libvirt always assigns one, so "" only means it was not read.
func matchVMRenamesByUUID(liveUUIDs, orphanUUIDs map[string]string) map[string]RenameCandidate {
	liveNamesByUUID := make(map[string][]string, len(liveUUIDs))
	for name, uuid := range liveUUIDs {
		if uuid == "" {
			continue
		}
		liveNamesByUUID[uuid] = append(liveNamesByUUID[uuid], name)
	}
	orphanNamesByUUID := make(map[string][]string, len(orphanUUIDs))
	for name, uuid := range orphanUUIDs {
		if uuid == "" {
			continue
		}
		orphanNamesByUUID[uuid] = append(orphanNamesByUUID[uuid], name)
	}

	out := make(map[string]RenameCandidate)
	for uuid, liveNames := range liveNamesByUUID {
		if len(liveNames) != 1 {
			continue // this UUID is live under more than one name: ambiguous
		}
		orphanNames, ok := orphanNamesByUUID[uuid]
		if !ok || len(orphanNames) != 1 {
			continue // no not-installed match, or this UUID is claimed by more than one entry
		}
		out[liveNames[0]] = RenameCandidate{OldName: orphanNames[0], Reason: reasonLibvirtUUID}
	}
	return out
}
