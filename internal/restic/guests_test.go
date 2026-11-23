package restic_test

import (
	"context"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

const guestHost = "bv-feedfacecafe"

// guestRepo is a repository of this server that also holds a guest's copies:
// container:plex twice from this server, once under a container's hostname
// from before there was a fixed host and once under its own, and twice from
// the guest, which also has a container:mail of its own.
func guestRepo(t *testing.T) (repo string, own, guest []string) {
	t.Helper()
	repo, src := hostSwitchRepo(t)
	backup := func(host, tag string) string {
		sum, err := restic.Restic{Bin: "restic"}.Backup(context.Background(), repo, []string{src}, []string{tag}, restic.Mode{Host: host})
		if err != nil {
			t.Fatal(err)
		}
		return sum.SnapshotID
	}
	own = append(own, backup("plex-4f1c", "container:plex"))
	guest = append(guest, backup(guestHost, "container:plex"))
	own = append(own, backup(ownHost, "container:plex"))
	guest = append(guest, backup(guestHost, "container:plex"), backup(guestHost, "container:mail"))
	return repo, own, guest
}

func everySnapshot(t *testing.T, repo string) []string {
	t.Helper()
	snaps, err := restic.Restic{Bin: "restic"}.Snapshots(context.Background(), repo, restic.Mode{Host: ownHost})
	if err != nil {
		t.Fatal(err)
	}
	return snapshotIDs(snaps)
}

func TestAListingLeavesOutTheSnapshotsOfTheGuests(t *testing.T) {
	repo, own, _ := guestRepo(t)
	snaps, err := restic.Restic{Bin: "restic"}.Snapshots(context.Background(), repo, restic.Mode{Host: ownHost, Guests: []string{guestHost}})
	if err != nil {
		t.Fatal(err)
	}
	if got := snapshotIDs(snaps); !slices.Equal(got, sorted(own)) {
		t.Fatalf("listed %v, want this server's %v", got, sorted(own))
	}
}

func TestAKeepPolicyOfAnEntryLeavesTheGuestsCopiesAlone(t *testing.T) {
	repo, own, guest := guestRepo(t)
	m := restic.Mode{Host: ownHost, Guests: []string{guestHost}}

	if err := (restic.Restic{Bin: "restic"}).ForgetPolicy(context.Background(), repo, restic.RetentionPolicy{KeepLast: 1}, m, []string{"container:plex"}, false); err != nil {
		t.Fatal(err)
	}
	if got, want := everySnapshot(t, repo), sorted(own[1:], guest); !slices.Equal(got, want) {
		t.Fatalf("keep-last 1 left %v, want this server's newest across its hosts and every copy of the guest %v", got, want)
	}
}

func TestAKeepPolicyOverTheWholeRepositoryLeavesTheGuestsCopiesAlone(t *testing.T) {
	repo, own, guest := guestRepo(t)
	m := restic.Mode{Host: ownHost, Guests: []string{guestHost}}

	if err := (restic.Restic{Bin: "restic"}).ForgetPolicy(context.Background(), repo, restic.RetentionPolicy{KeepLast: 1}, m, nil, false); err != nil {
		t.Fatal(err)
	}
	if got, want := everySnapshot(t, repo), sorted(own[1:], guest); !slices.Equal(got, want) {
		t.Fatalf("keep-last 1 left %v, want %v", got, want)
	}
}

func TestAKeepPolicyOfAnEntryOnlyAGuestHasForgetsNothing(t *testing.T) {
	repo, own, guest := guestRepo(t)
	m := restic.Mode{Host: ownHost, Guests: []string{guestHost}}

	if err := (restic.Restic{Bin: "restic"}).ForgetPolicy(context.Background(), repo, restic.RetentionPolicy{KeepLast: 1}, m, []string{"container:mail"}, false); err != nil {
		t.Fatal(err)
	}
	if got, want := everySnapshot(t, repo), sorted(own, guest); !slices.Equal(got, want) {
		t.Fatalf("left %v, want everything %v", got, want)
	}
}

func TestAKeepPolicyPreviewLeavesOutTheGuestsCopies(t *testing.T) {
	repo, own, _ := guestRepo(t)
	m := restic.Mode{Host: ownHost, Guests: []string{guestHost}}

	groups, err := restic.Restic{Bin: "restic"}.ForgetPreview(context.Background(), repo, restic.RetentionPolicy{KeepLast: 1}, m, "container:plex")
	if err != nil {
		t.Fatal(err)
	}
	var seen []restic.Snapshot
	for _, g := range groups {
		seen = append(seen, g.Keep...)
		seen = append(seen, g.Remove...)
	}
	if got := snapshotIDs(seen); !slices.Equal(got, sorted(own)) {
		t.Fatalf("the preview covered %v, want this server's %v", got, sorted(own))
	}
	if groups, err = (restic.Restic{Bin: "restic"}).ForgetPreview(context.Background(), repo, restic.RetentionPolicy{KeepLast: 1}, m, "container:mail"); err != nil || len(groups) != 0 {
		t.Fatalf("a preview of the guest's own entry = %v, %v, want nothing", groups, err)
	}
}
