package api

import "github.com/junkerderprovinz/bombvault/internal/restic"

// underHost keeps the snapshots written under host. A copy into a shared
// repository takes nothing else: restic copy keeps a snapshot's host, and one
// under the legacy host or another server's would land there as nobody's.
func underHost(snaps []restic.Snapshot, host string) []restic.Snapshot {
	var out []restic.Snapshot
	for _, sn := range snaps {
		if host != "" && sn.Hostname == host {
			out = append(out, sn)
		}
	}
	return out
}
