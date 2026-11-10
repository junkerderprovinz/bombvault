package api

import (
	"context"
	"testing"

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
