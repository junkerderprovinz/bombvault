package api

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAnOffSiteTargetAtASharedRepositoryCarriesThisServersPartInIt(t *testing.T) {
	s := unraidNotifyService(t, nil)
	g, err := s.store.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	for _, sr := range []store.SharedRepo{
		{Repo: "rest:http://nas:8001/shared", Owner: "nas-id"},
		{Repo: "/mnt/user/shared", Owner: g.InstanceID},
	} {
		if _, err := s.store.PutSharedRepo(sr); err != nil {
			t.Fatal(err)
		}
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	for repo, want := range map[string]restic.Sharing{
		"rest:http://nas:8001/shared": restic.SharingMember,
		"/mnt/user/shared":            restic.SharingOwner,
		"rest:http://nas:8000/tower":  restic.SharingNone,
	} {
		target := store.OffsiteTarget{Domain: "containers", Repo: repo, Enabled: true}
		if got := s.offsiteModeForTarget(settings, target).Sharing; got != want {
			t.Errorf("%s: sharing %v, want %v", repo, got, want)
		}
	}
}

// A listing made for one part in a repository is not the answer for another:
// a member sees only its own host there.
func TestASnapshotListingIsCachedApartForEachPartInARepository(t *testing.T) {
	alone := restic.Mode{Host: "bv-0123456789ab"}
	member := alone
	member.Sharing = restic.SharingMember
	owner := alone
	owner.Sharing = restic.SharingOwner
	other := member
	other.Host = "bv-ba9876543210"
	keys := map[string]bool{}
	for _, m := range []restic.Mode{alone, member, owner, other} {
		keys[repoAccessKey(m)] = true
	}
	if len(keys) != 4 {
		t.Fatalf("%d distinct keys for four parts, want 4", len(keys))
	}
}

// The part in a shared repository reaches restic from the store, through the
// mode of the off-site target that copies there.
func TestAMemberListsOnlyItsOwnSnapshotsAtASharedTarget(t *testing.T) {
	f := newResticFixture(t)
	ctx := context.Background()
	f.write("plex", map[string]string{"a.txt": "a"})
	src := filepath.Join(filepath.FromSlash(f.root), "src", "plex")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	target, err := f.st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Shared", Repo: "backups/shared", Role: "offsite", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	dest, err := f.svc.resolveRepo(target.Repo)
	if err != nil {
		t.Fatal(err)
	}
	mode := f.svc.offsiteModeForTarget(settings, target)
	if err := f.svc.engine.Init(ctx, dest, mode); err != nil {
		t.Fatal(err)
	}
	other := mode
	other.Host = "bv-ba9876543210"
	for _, m := range []restic.Mode{mode, other} {
		if _, err := f.svc.engine.BackupDir(ctx, dest, src, []string{"container:plex"}, m); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.st.PutSharedRepo(store.SharedRepo{Repo: target.Repo, Owner: "nas-id"}); err != nil {
		t.Fatal(err)
	}

	snaps, err := f.svc.listTarget(ctx, settings, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].Hostname != f.svc.resticHost() {
		t.Fatalf("listed %+v, want the one snapshot under this server's host %s", snaps, f.svc.resticHost())
	}
}
