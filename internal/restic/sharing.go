package restic

import (
	"context"
	"errors"
	"slices"
)

// ErrSharedRepoWide refuses a keep policy over every entry of a shared
// repository at once.
var ErrSharedRepoWide = errors.New("a shared repository is aged entry by entry and host by host, never as a whole")

// ErrSharedLocks is what a member asked to remove every lock of a shared
// repository answers with.
var ErrSharedLocks = errors.New("the locks of a shared repository are cleared by the server that holds it")

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
		return m.selected
	}
	return []string{m.backupHost()}
}

// withoutGuests drops the snapshots of the mode's guests from a listing.
func withoutGuests[S any](snaps []S, m Mode, host func(S) string) []S {
	if len(m.Guests) == 0 {
		return snaps
	}
	return slices.DeleteFunc(snaps, func(sn S) bool { return slices.Contains(m.Guests, host(sn)) })
}

// selectBesideGuests narrows a keep policy in a repository with guests to the
// other hosts that hold snapshots of the tags, since restic selects hosts but
// cannot leave one out. ours is false when only guests hold any, and nothing
// is to be aged.
func (r Restic) selectBesideGuests(ctx context.Context, repo string, m Mode, tags []string) (narrowed Mode, ours bool, err error) {
	if len(m.Guests) == 0 || m.Sharing != SharingNone {
		return m, true, nil
	}
	args := snapshotsArgs(repo, m, nil)
	for _, t := range tags {
		if t != "" {
			args = append(args, "--tag", t)
		}
	}
	snaps, err := r.snapshots(ctx, args, m)
	if err != nil {
		return m, false, err
	}
	var hosts []string
	for _, sn := range snaps {
		if !slices.Contains(hosts, sn.Hostname) {
			hosts = append(hosts, sn.Hostname)
		}
	}
	slices.Sort(hosts)
	m.selected = hosts
	return m, len(hosts) > 0, nil
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

// refuseRepoWide refuses a keep policy without tags in a shared repository.
// restic would group by paths there, across the hosts of every server.
func (m Mode) refuseRepoWide(tags []string) error {
	if m.Sharing == SharingNone || slices.ContainsFunc(tags, func(t string) bool { return t != "" }) {
		return nil
	}
	return ErrSharedRepoWide
}

// checkWithoutLock reports whether a check leaves the repository unlocked: a
// read-only session must not write a lock, and a member's exclusive check lock
// would hold up the owner's prune and every other member's copy.
func (m Mode) checkWithoutLock() bool {
	return m.NoLock || m.Sharing == SharingMember
}
