package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestATakeOverRestoresOnlyThisServersEntriesOfASharedRepositoryWithRestic(t *testing.T) {
	old, _, dest, _ := memberWithRestic(t, oldKey)
	text, err := old.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}
	// The new server reaches the old one's shared repository by the same path.
	f := newResticFixture(t)
	f.svc.cfg.HostMountRoot = old.root
	ctx := context.Background()
	if err := f.svc.ReadEmergency(ctx, emergencyRead{Kit: text}); err != nil {
		t.Fatal(err)
	}
	sess, err := f.svc.currentEmergency()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.AdoptEmergencyKey(sess.id); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.AttachEmergencyPlaces(ctx, sess.id); err != nil {
		t.Fatal(err)
	}
	// A folder is no place a kit gives a copy target, so the person adds it.
	if _, err := f.st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "files", Name: "Shared with Atlas", Repo: "backups/shared", Enabled: true, Immutable: true}); err != nil {
		t.Fatal(err)
	}

	got, err := f.svc.EmergencyOffsiteEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.FileSets) != 1 || got.FileSets["docs"] == 0 {
		t.Fatalf("the take-over offers %+v from %s", got, dest)
	}
	var servers []sharedOtherServer
	decodeInto(t, f.do("GET", "/api/shared-repo/others", nil), "servers", &servers)
	if len(servers) != 1 || servers[0].Host != cubeHost {
		t.Fatalf("other servers after the take-over %+v", servers)
	}
}

func TestAServerThatHeldASharedRepositoryKeepsItAgainAfterATakeOverWithRestic(t *testing.T) {
	old, _, dest, _ := sharedRestic(t)
	if err := old.st.SaveSharedRepoServer(store.SharedRepoServer{
		ContainerName: sharedServerName, Folder: "backups", HostPath: "/mnt/user/backups", Port: 8011, User: "here", Repo: "backups/shared",
	}); err != nil {
		t.Fatal(err)
	}
	text, err := old.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}

	f := newResticFixture(t)
	f.svc.cfg.AppKey = strings.Repeat("c", 64)
	f.svc.forgetBackupKey()
	f.svc.cfg.HostMountRoot = old.root
	f.svc.docker = &receiverDocker{}
	ctx := context.Background()
	if err := f.svc.ReadEmergency(ctx, emergencyRead{Kit: text}); err != nil {
		t.Fatal(err)
	}
	sess, err := f.svc.currentEmergency()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.AdoptEmergencyKey(sess.id); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.AttachEmergencyPlaces(ctx, sess.id); err != nil {
		t.Fatal(err)
	}

	sr, ok, err := f.st.SharedRepoAt("backups/shared")
	if err != nil || !ok || !f.svc.ownsSharedRepo(sr) {
		t.Fatalf("the new server does not hold the shared repository: %+v, %v, %v", sr, ok, err)
	}
	// The rules of the shared repository are not in the kit, so the person sets them again.
	if err := f.st.PutSharedUpkeepRules(store.SharedUpkeep{RepoID: sr.ID, KeepSet: true, Keep: store.RetentionKeep{KeepLast: 1},
		PruneCadence: "weekly sun 03:00", PruneHours: 6, CheckSubset: 100}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.keepSharedRepo(ctx, sr, &fakeMembers{held: map[string][]string{cubeHost: {"fileset:docs"}}}, time.Now(), true); err != nil {
		t.Fatal(err)
	}

	got := countByHost(t, f, dest, f.svc.sharedOwnerMode())
	want := map[string]int{
		towerHost + " container:plex": 1, towerHost + " fileset:docs": 1,
		cubeHost + " container:plex": 1, cubeHost + " fileset:docs": 3,
		strangerHost + " container:plex": 3, restic.LegacyHost + " fileset:docs": 3,
	}
	for k, n := range want {
		if got[k] != n {
			t.Errorf("%s: %d snapshots, want %d", k, got[k], n)
		}
	}
	u, err := f.st.SharedUpkeepOf(sr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if u.Prune == nil || u.Prune.Error != "" {
		t.Fatalf("the prune after the take-over: %+v", u.Prune)
	}
}
