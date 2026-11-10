package api

import (
	"context"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestTheKitNamesTheSharedRepositoryWithThisServersHost(t *testing.T) {
	f := encryptedFixture(t)
	memberOfAtlas(t, f)
	here := f.svc.resticHost()

	text, err := f.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"## Shared repositories",
		"- Atlas: " + atlasShared,
		"restic -r " + atlasShared + " snapshots --host " + here,
		"Cube, host " + cubeHost,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the kit lacks %q", want)
		}
	}
	kit, err := readRecoveryKit(text)
	if err != nil {
		t.Fatal(err)
	}
	var shared []kitLocation
	for _, loc := range kit.Locations {
		if loc.Kind == kitShared {
			shared = append(shared, loc)
		}
	}
	if len(shared) != 1 || shared[0].Location != atlasShared || shared[0].Owner != "atlas-id" || strings.Join(shared[0].Sections, ",") != "containers,files" {
		t.Fatalf("the shared repository read back as %+v", shared)
	}
	if len(shared[0].Others) != 1 || shared[0].Others[0] != cube {
		t.Errorf("the other servers read back as %+v", shared[0].Others)
	}
}

func TestATakeOverOffersOnlyThisServersEntriesOfASharedRepository(t *testing.T) {
	old := encryptedFixture(t)
	old.svc.cfg.AppKey = oldKey
	old.svc.forgetBackupKey()
	memberOfAtlas(t, old)
	oldHost := old.svc.resticHost()
	text, err := old.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}

	f := newPlacementFixture(t)
	f.eng.opens[atlasShared] = true
	f.eng.snaps[atlasShared] = old.eng.snaps[atlasShared]
	_, view := f.emergency("POST", "/api/emergency/read", map[string]any{"kit": text})
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["ok"] != true {
		t.Fatalf("adopt: %v", res)
	}
	if f.svc.resticHost() != oldHost {
		t.Fatalf("the new server writes as %q, the old one as %q", f.svc.resticHost(), oldHost)
	}
	if res, _ := f.emergency("POST", "/api/emergency/attach", map[string]any{"session": view.Session.ID}); res["ok"] != true {
		t.Fatalf("attach: %v", res)
	}

	sr, ok, err := f.st.SharedRepoAt(atlasShared)
	if err != nil || !ok || sr.Owner != "atlas-id" {
		t.Fatalf("the shared repository is recorded as %+v, %v, %v", sr, ok, err)
	}
	if f.svc.sharingAt(atlasShared) != restic.SharingMember {
		t.Error("the new server does not take part in the shared repository as a member")
	}
	got, err := f.svc.EmergencyOffsiteEntries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.FileSets["docs"] == 0 || got.FileSets["photos"] != 0 || got.FileSets["attic"] != 0 || len(got.Containers) != 0 {
		t.Fatalf("the take-over offers %+v", got)
	}
	targets, err := f.st.OffsiteTargetsForDomain("files")
	if err != nil || len(targets) != 1 || targets[0].Repo != atlasShared || !targets[0].Immutable {
		t.Fatalf("the folder sets copy to %+v, %v", targets, err)
	}

	if err := f.svc.UndoAdoption(); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := f.st.SharedRepoAt(atlasShared); err != nil || ok {
		t.Errorf("after the undo the shared repository is still recorded: %v, %v", ok, err)
	}
}

const ownedShared = "bv-shared/shared"

// ownerOfShared makes the fixture the server that holds a shared repository
// in its own folder, with its rest-server on port 8011. It copies its folder
// sets there, and Tower and Cube write to it as well.
func ownerOfShared(t *testing.T, f *placementFixture) store.SharedRepo {
	t.Helper()
	g, err := f.st.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	sr, err := f.st.PutSharedRepo(store.SharedRepo{Repo: ownedShared, Owner: g.InstanceID, Members: []store.SharedMember{
		tower, cube, {Host: f.svc.resticHost(), Instance: g.InstanceID, Name: "Here"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.SaveSharedRepoServer(store.SharedRepoServer{
		ContainerName: sharedServerName, Folder: "bv-shared", HostPath: "/mnt/user/bv-shared", Port: 8011, User: "here", Repo: ownedShared,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "files", Name: "Shared", Repo: ownedShared, Enabled: true, Immutable: true}); err != nil {
		t.Fatal(err)
	}
	return sr
}

// takeOverKit reads text into a new server on the same host mount, takes its
// key over and attaches its places.
func takeOverKit(t *testing.T, f *placementFixture, text string) {
	t.Helper()
	_, view := f.emergency("POST", "/api/emergency/read", map[string]any{"kit": text})
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["ok"] != true {
		t.Fatalf("adopt: %v", res)
	}
	if res, _ := f.emergency("POST", "/api/emergency/attach", map[string]any{"session": view.Session.ID}); res["ok"] != true {
		t.Fatalf("attach: %v", res)
	}
}

func TestAServerThatHeldASharedRepositoryHoldsItAgainAfterATakeOver(t *testing.T) {
	old := encryptedFixture(t)
	old.svc.cfg.AppKey = oldKey
	old.svc.forgetBackupKey()
	ownerOfShared(t, old)
	text, err := old.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}

	f := newPlacementFixture(t)
	f.svc.cfg.HostMountRoot = old.root
	docker := &receiverDocker{}
	f.svc.docker = docker
	takeOverKit(t, f, text)

	g, err := f.st.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	sr, ok, err := f.st.SharedRepoAt(ownedShared)
	if err != nil || !ok || sr.Owner != g.InstanceID {
		t.Fatalf("the shared repository is recorded as %+v, %v, %v, want this server as its owner", sr, ok, err)
	}
	if f.svc.sharingAt(ownedShared) != restic.SharingOwner {
		t.Error("the new server does not hold the shared repository")
	}
	for _, m := range []store.SharedMember{tower, cube} {
		if !slices.Contains(sr.Members, m) {
			t.Errorf("%s is no member any more: %+v", m.Name, sr.Members)
		}
	}
	ss, ok, err := f.st.GetSharedRepoServer()
	if err != nil || !ok || ss.Port != 8011 || ss.Folder != "bv-shared" || ss.HostPath != path.Join(f.svc.cfg.HostSourceRoot, "bv-shared") || ss.Repo != ownedShared || len(ss.PasswordEnc) == 0 {
		t.Fatalf("the shared repository's server is recorded as %+v, %v, %v", ss, ok, err)
	}
	if len(docker.created) != 1 || docker.created[0].Name != sharedServerName {
		t.Fatalf("containers created: %+v", docker.created)
	}
	targets, err := f.st.OffsiteTargetsForDomain("files")
	if err != nil || !slices.ContainsFunc(targets, func(t store.OffsiteTarget) bool { return t.Repo == ownedShared && t.Immutable }) {
		t.Fatalf("the folder sets copy to %+v, %v", targets, err)
	}
}

func TestATakeOverKeepsTheSharedServerThatStillRuns(t *testing.T) {
	old := encryptedFixture(t)
	old.svc.cfg.AppKey = oldKey
	old.svc.forgetBackupKey()
	ownerOfShared(t, old)
	text, err := old.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}

	f := newPlacementFixture(t)
	f.svc.cfg.HostMountRoot = old.root
	docker := &receiverDocker{live: map[string]bool{sharedServerName: true}}
	f.svc.docker = docker
	takeOverKit(t, f, text)

	if len(docker.created) != 0 {
		t.Fatalf("a second container was made: %+v", docker.created)
	}
	if _, ok, err := f.st.GetSharedRepoServer(); err != nil || !ok {
		t.Fatalf("the running server is not recorded: %v, %v", ok, err)
	}
}

func TestAKitOfAMemberLeavesTheSharedServerAlone(t *testing.T) {
	old := encryptedFixture(t)
	old.svc.cfg.AppKey = oldKey
	old.svc.forgetBackupKey()
	memberOfAtlas(t, old)
	text, err := old.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}
	f := newPlacementFixture(t)
	f.eng.opens[atlasShared] = true
	docker := &receiverDocker{}
	f.svc.docker = docker
	takeOverKit(t, f, text)

	if _, ok, _ := f.st.GetSharedRepoServer(); ok || len(docker.created) != 0 {
		t.Fatalf("a member's kit set up a shared server: %v, %+v", ok, docker.created)
	}
}
