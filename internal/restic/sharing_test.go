package restic_test

import (
	"context"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// hostsIn returns the values of every --host flag in args.
func hostsIn(args []string) []string {
	var hosts []string
	for i, a := range args {
		if a == "--host" && i+1 < len(args) {
			hosts = append(hosts, args[i+1])
		}
	}
	return hosts
}

func TestASharedRepositoryListsOnlyTheSnapshotsOfThisServer(t *testing.T) {
	for _, sharing := range []restic.Sharing{restic.SharingMember, restic.SharingOwner} {
		args := restic.SnapshotsArgs("/repo", restic.Mode{Host: ownHost, Sharing: sharing})
		if got := hostsIn(args); !slices.Equal(got, []string{ownHost}) {
			t.Errorf("sharing %v: hosts %v in %v, want only %s", sharing, got, args, ownHost)
		}
	}
}

// A repository only this server writes to may hold snapshots under the legacy
// host, under a container's hostname from before there was a fixed one, or
// imported ones, and all of them are its own.
func TestARepositoryOfThisServerAloneListsEveryHost(t *testing.T) {
	if got := hostsIn(restic.SnapshotsArgs("/repo", restic.Mode{Host: ownHost})); len(got) != 0 {
		t.Fatalf("hosts %v, want no filter", got)
	}
}

const otherHost = "bv-ba9876543210"

// sharedRepo is a repository in which this server and another one each backed
// up the same folder under the tag container:plex, this server twice.
func sharedRepo(t *testing.T) (repo string, own, other []string) {
	t.Helper()
	repo, src := hostSwitchRepo(t)
	backup := func(host string) string {
		sum, err := restic.Restic{Bin: "restic"}.Backup(context.Background(), repo, []string{src}, []string{"container:plex"}, restic.Mode{Host: host})
		if err != nil {
			t.Fatal(err)
		}
		return sum.SnapshotID
	}
	own = append(own, backup(ownHost))
	other = append(other, backup(otherHost))
	own = append(own, backup(ownHost))
	return repo, own, other
}

func snapshotIDs(snaps []restic.Snapshot) []string {
	ids := make([]string, 0, len(snaps))
	for _, sn := range snaps {
		ids = append(ids, sn.ID)
	}
	slices.Sort(ids)
	return ids
}

func sorted(ids ...[]string) []string {
	out := slices.Concat(ids...)
	slices.Sort(out)
	return out
}

func TestAMemberSeesOnlyItsOwnSnapshotsInASharedRepository(t *testing.T) {
	repo, own, other := sharedRepo(t)
	r, ctx := restic.Restic{Bin: "restic"}, context.Background()

	snaps, err := r.Snapshots(ctx, repo, restic.Mode{Host: ownHost, Sharing: restic.SharingMember})
	if err != nil {
		t.Fatal(err)
	}
	if got := snapshotIDs(snaps); !slices.Equal(got, sorted(own)) {
		t.Fatalf("listed %v, want this server's %v", got, sorted(own))
	}
	every, err := r.SnapshotsOfEveryHost(ctx, repo, restic.Mode{Host: ownHost, Sharing: restic.SharingMember})
	if err != nil {
		t.Fatal(err)
	}
	if got := snapshotIDs(every); !slices.Equal(got, sorted(own, other)) {
		t.Fatalf("every host listed %v, want %v", got, sorted(own, other))
	}
}

func TestListingEveryHostOfASharedRepositoryLeavesOutTheFilter(t *testing.T) {
	m := restic.Mode{Host: ownHost, Sharing: restic.SharingOwner}
	if got := hostsIn(restic.EveryHostSnapshotsArgs("/repo", m)); len(got) != 0 {
		t.Fatalf("hosts %v, want no filter", got)
	}
}
