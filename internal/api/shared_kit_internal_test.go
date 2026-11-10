package api

import (
	"context"
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
	if len(shared[0].Others) != 1 || shared[0].Others[0] != (store.SharedMember{Host: cubeHost, Name: "Cube"}) {
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
