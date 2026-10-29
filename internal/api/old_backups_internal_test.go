package api

import (
	"context"
	"slices"
	"testing"
)

func TestADestinationSavedOverOldBackupsListsThemWithoutCopyingThere(t *testing.T) {
	f := newPlacementFixture(t)
	mani := f.destination("Mani", vaultRepo)
	f.hold(vaultRepo+"/containers", snap("old1", 100, "container:web"))

	f.svc.takeUpOldBackups(context.Background(), mani)

	targets, err := f.st.DestinationTargets(mani.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].Domain != "containers" {
		t.Fatalf("targets = %+v, want one for the containers that were backed up there", targets)
	}
	d, _, err := f.st.PlacementDefaultFor("containers")
	if err != nil {
		t.Fatal(err)
	}
	if !skipsTarget(d.Skip, targets[0].ID) {
		t.Fatalf("default skip = %v, want the new target left unticked", d.Skip)
	}
	f.container("web", "")
	tl, err := f.svc.Timeline(context.Background(), "containers", "web")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(tl.Places, func(p timelinePlace) bool { return p.Place == offsiteSourcePrefix+targets[0].ID }) {
		t.Fatalf("places = %+v, want Mani offered for restore", tl.Places)
	}
}

func TestADestinationHoldingTheOwnFolderTakesUpNoCopyOfIt(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{"domains": []string{"containers"}, "location": "destination:" + vault.ID}))
	f.hold(vaultRepo+"/containers", snap("a1", 100, "container:web"))

	f.svc.takeUpOldBackups(context.Background(), vault)

	if targets, _ := f.st.DestinationTargets(vault.ID); len(targets) != 0 {
		t.Fatalf("targets = %+v, want none over the own folder", targets)
	}
}

func TestACopyOfASectionAtItsOwnFolderIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{"domains": []string{"containers"}, "location": "destination:" + vault.ID}))

	res := f.do("POST", "/api/offsite/destinations/"+vault.ID+"/domains/containers", nil)

	if res["ok"] != false || res["code"] != "copy-on-own-folder" {
		t.Fatalf("answer = %v, want the copy onto the own folder refused", res)
	}
	if targets, _ := f.st.DestinationTargets(vault.ID); len(targets) != 0 {
		t.Fatalf("targets = %+v, want none", targets)
	}
}
