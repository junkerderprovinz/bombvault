package api

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestTheOwnerCopiesItsOwnBackupsIntoItsSharedRepositoryFromTheFolder(t *testing.T) {
	nas, _, _ := sharedPair(t)
	v := setUpShared(t, nas)

	if _, err := nas.svc.CopyIntoOwnSharedRepo(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := nas.svc.CopyIntoOwnSharedRepo(context.Background()); err != nil {
		t.Fatalf("a second time: %v", err)
	}
	sr, _, err := nas.st.SharedRepoAt(v.Repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(sr.Members) != 1 || sr.Members[0].Host != nas.svc.resticHost() || sr.Members[0].Instance != nas.id(t) {
		t.Fatalf("members %+v, want the owner itself once", sr.Members)
	}
	targets, err := nas.st.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	at := slices.DeleteFunc(targets, func(tg store.OffsiteTarget) bool { return tg.Repo != v.Repo })
	if len(at) != len(offsiteConfigDomains) {
		t.Fatalf("%d targets at the folder, want one per section", len(at))
	}
	settings, err := nas.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	for _, tg := range at {
		if tg.Enabled || tg.CredsRef != "" || targetAgingPolicy(tg).Any() {
			t.Fatalf("target %+v, want it off, without a login and without a keep policy", tg)
		}
		if mode := nas.svc.offsiteModeForTarget(settings, tg); mode.Sharing != restic.SharingOwner || mode.Password != nas.svc.backupPassword() {
			t.Fatalf("the copy there runs as %+v, want the owner's part with its own password", mode.Sharing)
		}
	}
	got, err := nas.svc.sharedServerView(context.Background())
	if err != nil || !got.CopiesIn || len(got.Members) != 1 || !got.Members[0].Self {
		t.Fatalf("view %+v, %v; want the owner copying in and named as this server", got, err)
	}
}

func TestOnlyTheServerThatHoldsASharedRepositoryCopiesInFromTheFolder(t *testing.T) {
	_, tower, _ := sharedPair(t)
	if _, err := tower.svc.CopyIntoOwnSharedRepo(context.Background()); err == nil {
		t.Fatal("a server without a shared repository took one up")
	}
}

func TestTheOwnersCardStartsCopyingItsOwnBackupsIn(t *testing.T) {
	nas, _, _ := sharedPair(t)
	setUpShared(t, nas)
	code, out := nas.do(t, http.MethodPost, "/api/shared-repo/own-copies", nil)
	server, _ := out["server"].(map[string]any)
	if code != http.StatusOK || out["ok"] != true || server["copiesIn"] != true {
		t.Fatalf("POST: %d %v", code, out)
	}
}
