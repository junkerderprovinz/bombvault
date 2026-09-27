package api

import (
	"net/http"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ZFS datasets are a domain like flash and config: a path, copies and a
// repository per item, but no placement. On places they get a folder and a
// home place, and their copies stand at places like any other domain's.

func TestTheMoveOntoPlacesTakesTheZFSDomain(t *testing.T) {
	f := newPlacementFixture(t)
	f.settings(func(s *store.Settings) { s.ZFSEnabled, s.ZFSPath = true, "backups/zfs" })
	copied := f.target("zfs", "B2", "s3:https://s3.example.com/bv/zfs")
	if err := f.svc.MigrateToPlaces(); err != nil {
		t.Fatalf("MigrateToPlaces: %v", err)
	}

	homes, err := f.st.DomainPlaces()
	if err != nil {
		t.Fatal(err)
	}
	home, err := f.st.GetPlace(homes["zfs"])
	if err != nil {
		t.Fatalf("the ZFS domain has no home place: %v (homes %v)", err, homes)
	}
	if home.Folders["zfs"] != "zfs" {
		t.Fatalf("home place %s folders = %v, want the ZFS path's folder", home.Name, home.Folders)
	}
	if got := settingsOf(t, f.svc).ZFSPath; got != "backups/zfs" {
		t.Fatalf("ZFS path = %q, want it as it was", got)
	}
	row := f.storedTarget(copied.ID)
	if row.PlaceID == "" || row.PlaceDomain != "zfs" || row.Repo != copied.Repo {
		t.Fatalf("ZFS copy target = %+v, want it at a place for the ZFS domain at its address", row)
	}
}

func TestANewPlaceOffersTheZFSDomain(t *testing.T) {
	if !slices.Contains(places.Domains, "zfs") || places.DefaultFolders()["zfs"] != "zfs" {
		t.Fatalf("domains %v, default folders %v, want zfs with its own folder", places.Domains, places.DefaultFolders())
	}
}

func TestTheDomainsCardListsZFSWhileItIsOn(t *testing.T) {
	f := newPlacementFixture(t)
	f.settings(func(s *store.Settings) { s.ZFSPath = "backups/zfs" })
	if err := f.svc.MigrateToPlaces(); err != nil {
		t.Fatalf("MigrateToPlaces: %v", err)
	}
	listed := func() []string {
		res := f.do(http.MethodGet, "/api/storage/domains", nil)
		rows, _ := res["domains"].([]any)
		var out []string
		for _, r := range rows {
			out = append(out, r.(map[string]any)["domain"].(string))
		}
		return out
	}
	if got := listed(); slices.Contains(got, "zfs") {
		t.Fatalf("rows = %v with ZFS switched off", got)
	}
	f.settings(func(s *store.Settings) { s.ZFSEnabled = true })
	if got := listed(); !slices.Contains(got, "zfs") {
		t.Fatalf("rows = %v, want a ZFS row once ZFS is on", got)
	}
}

func TestAZFSDatasetOnTheDomainPathCountsAsItsBackup(t *testing.T) {
	f := newPlacementFixture(t)
	own, err := f.st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata"})
	if err != nil {
		t.Fatal(err)
	}
	if backed, err := f.st.DomainPathBackedUp("zfs"); err != nil || backed {
		t.Fatalf("before a backup: %v, %v", backed, err)
	}
	runID, err := f.st.StartRun(own.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.FinishRun(runID, "success", "snap", 1, ""); err != nil {
		t.Fatal(err)
	}
	if backed, err := f.st.DomainPathBackedUp("zfs"); err != nil || !backed {
		t.Fatalf("after a backup on the domain path: %v, %v", backed, err)
	}
}

func TestAnUnusedZFSPathGetsNoPlace(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.MigrateToPlaces(); err != nil {
		t.Fatalf("MigrateToPlaces: %v", err)
	}
	homes, err := f.st.DomainPlaces()
	if err != nil {
		t.Fatal(err)
	}
	if home, ok := homes["zfs"]; ok {
		t.Fatalf("the ZFS domain, off and never used, got the home place %s", home)
	}
	unplacedZFS := func() bool {
		res := f.do(http.MethodGet, "/api/places", nil)
		rows, _ := res["unplaced"].([]any)
		return slices.ContainsFunc(rows, func(r any) bool { return r.(map[string]any)["domain"] == "zfs" })
	}
	if unplacedZFS() {
		t.Fatal("the path of a switched-off ZFS domain waits under Without a place")
	}
	f.settings(func(s *store.Settings) { s.ZFSEnabled = true })
	if !unplacedZFS() {
		t.Fatal("the ZFS path, switched on, is not offered to go on a place")
	}
}
