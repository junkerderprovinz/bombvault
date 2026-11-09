package restic

import "context"

// Sharing is what this server may do in a repository: write to it alone, add
// to one that other servers of its group write to as well, or hold such a
// repository as its owner, the one server that forgets and prunes there.
type Sharing uint8

const (
	SharingNone Sharing = iota
	SharingMember
	SharingOwner
)

// ownHosts are the hosts whose snapshots are this server's in the repository,
// or nil when every snapshot there is. A repository this server writes to
// alone may hold the legacy host, the hostnames of containers from before
// there was a fixed host, and imported snapshots, and all of them are its own.
// In a shared repository only the host this server writes counts.
func (m Mode) ownHosts() []string {
	if m.Sharing == SharingNone {
		return nil
	}
	return []string{m.backupHost()}
}

func hostFlags(hosts []string) []string {
	var args []string
	for _, h := range hosts {
		args = append(args, "--host", h)
	}
	return args
}

// EveryHostSnapshotsArgs lists the snapshots of every server in a shared
// repository, for the owner's upkeep and the view of the other servers.
func EveryHostSnapshotsArgs(repo string, m Mode) []string {
	return snapshotsArgs(repo, m, nil)
}

// SnapshotsOfEveryHost is Snapshots without the filter a shared repository
// puts on it.
func (r Restic) SnapshotsOfEveryHost(ctx context.Context, repo string, m Mode) ([]Snapshot, error) {
	return r.snapshots(ctx, EveryHostSnapshotsArgs(repo, m), m)
}
