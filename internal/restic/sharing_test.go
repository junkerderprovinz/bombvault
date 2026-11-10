package restic_test

import (
	"context"
	"errors"
	"path/filepath"
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

func TestAKeepPolicyInASharedRepositorySelectsTheModesHost(t *testing.T) {
	m := restic.Mode{Host: ownHost, Sharing: restic.SharingOwner}
	p := restic.RetentionPolicy{KeepLast: 3}
	for name, args := range map[string][]string{
		"forget":  restic.ForgetPolicyArgs("/repo", p, m, []string{"container:plex"}, false),
		"preview": restic.ForgetPreviewArgs("/repo", p, m, "container:plex"),
	} {
		if got := hostsIn(args); !slices.Equal(got, []string{ownHost}) {
			t.Errorf("%s: hosts %v in %v, want only %s", name, got, args, ownHost)
		}
	}
}

// Old and new snapshots of an entry age together in a repository of this
// server alone, whatever host they carry.
func TestAKeepPolicyInARepositoryOfThisServerAloneSelectsNoHost(t *testing.T) {
	m := restic.Mode{Host: ownHost}
	p := restic.RetentionPolicy{KeepLast: 3}
	for name, args := range map[string][]string{
		"forget":  restic.ForgetPolicyArgs("/repo", p, m, []string{"container:plex"}, false),
		"preview": restic.ForgetPreviewArgs("/repo", p, m, "container:plex"),
	} {
		if got := hostsIn(args); len(got) != 0 {
			t.Errorf("%s: hosts %v, want none", name, got)
		}
	}
}

// Grouped by paths, a pass over the whole repository would put two servers'
// snapshots of one folder into one group.
func TestAPassOverAWholeSharedRepositoryIsRefused(t *testing.T) {
	r := restic.Restic{Bin: filepath.Join(t.TempDir(), "no-restic")}
	m := restic.Mode{Host: ownHost, Sharing: restic.SharingOwner}
	p := restic.RetentionPolicy{KeepLast: 3}
	if err := r.ForgetPolicy(context.Background(), "/repo", p, m, nil, true); !errors.Is(err, restic.ErrSharedRepoWide) {
		t.Errorf("forget: err = %v, want ErrSharedRepoWide", err)
	}
	if _, err := r.ForgetPreview(context.Background(), "/repo", p, m); !errors.Is(err, restic.ErrSharedRepoWide) {
		t.Errorf("preview: err = %v, want ErrSharedRepoWide", err)
	}
}

func TestEachServerKeepsItsOwnHistoryOfAnEntryBothBackUp(t *testing.T) {
	repo, own, other := sharedRepo(t)
	r, ctx := restic.Restic{Bin: "restic"}, context.Background()
	owner := restic.Mode{Host: ownHost, Sharing: restic.SharingOwner}

	if err := r.ForgetPolicy(ctx, repo, restic.RetentionPolicy{KeepLast: 1}, owner, []string{"container:plex"}, false); err != nil {
		t.Fatal(err)
	}
	every, err := r.SnapshotsOfEveryHost(ctx, repo, owner)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := snapshotIDs(every), sorted(own[1:], other); !slices.Equal(got, want) {
		t.Fatalf("keep-last 1 under %s left %v, want its newest and the other server's %v", ownHost, got, want)
	}
}

// check without --no-lock locks exclusively, and a member must never hold up
// the owner's prune or the other members' copies.
func TestAMemberChecksASharedRepositoryWithoutALock(t *testing.T) {
	for _, c := range []struct {
		sharing restic.Sharing
		noLock  bool
	}{
		{restic.SharingMember, true},
		{restic.SharingOwner, false},
		{restic.SharingNone, false},
	} {
		m := restic.Mode{Host: ownHost, Sharing: c.sharing}
		for name, args := range map[string][]string{
			"check": restic.CheckArgs("/repo", m),
			"data":  restic.CheckDataArgs("/repo", 5, m),
		} {
			if got := slices.Contains(args, "--no-lock"); got != c.noLock {
				t.Errorf("sharing %v, %s: %v, want --no-lock %v", c.sharing, name, args, c.noLock)
			}
			if slices.Contains(args, "--no-lock") == slices.Contains(args, "--retry-lock") {
				t.Errorf("sharing %v, %s: %v, want either --no-lock or --retry-lock", c.sharing, name, args)
			}
		}
	}
}

func TestListingEveryHostOfASharedRepositoryLeavesOutTheFilter(t *testing.T) {
	m := restic.Mode{Host: ownHost, Sharing: restic.SharingOwner}
	if got := hostsIn(restic.EveryHostSnapshotsArgs("/repo", m)); len(got) != 0 {
		t.Fatalf("hosts %v, want no filter", got)
	}
}
