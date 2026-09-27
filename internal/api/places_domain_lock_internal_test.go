package api

import (
	"maps"
	"net/http"
	"testing"
)

// holdDomain takes the domain's lock the way a running backup or copy does and
// gives it back when the test ends.
func (f *placementFixture) holdDomain(domain, reason string) {
	f.t.Helper()
	unlock, ok := f.svc.tryLockDomainFor(domain, reason)
	if !ok {
		f.t.Fatalf("the %s lock is taken", domain)
	}
	f.t.Cleanup(unlock)
}

func TestAPlaceEditDoesNotMoveADomainPathWhileItsBackupRuns(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	f.holdDomain("containers", "backup")
	folders := maps.Clone(unraid.Folders)
	folders["containers"] = "ct"

	res := f.do(http.MethodPatch, "/api/places/"+unraid.ID, map[string]any{"folders": folders})

	if res["ok"] != false || res["code"] != "domain-busy" {
		t.Fatalf("PATCH = %v, want domain-busy", res)
	}
	if settings, err := f.st.GetSettings(); err != nil || settings.ContainersPath != "backups/containers" {
		t.Fatalf("containers path = %q, %v, want it unmoved", settings.ContainersPath, err)
	}
}

func TestAPlaceEditMovesADomainPathWhileAnotherDomainIsBusy(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	f.holdDomain("containers", "backup")
	folders := maps.Clone(unraid.Folders)
	folders["vms"] = "virtual"

	if res := f.do(http.MethodPatch, "/api/places/"+unraid.ID, map[string]any{"folders": folders}); res["ok"] != true {
		t.Fatalf("PATCH = %v", res)
	}
	if settings, err := f.st.GetSettings(); err != nil || settings.VMsPath != "backups/virtual" {
		t.Fatalf("vms path = %q, %v", settings.VMsPath, err)
	}
}

func TestAPlaceEditDoesNotMoveATargetWhileItsCopyRuns(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	target := f.placeTarget(b2, "vms", "")
	f.eng.opens["s3:https://s3.example.com/other/vms"] = false
	f.holdDomain("vms", "replicate")

	res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"address": map[string]string{"endpoint": "s3.example.com", "bucket": "other"}})

	if res["ok"] != false || res["code"] != "domain-busy" {
		t.Fatalf("PATCH = %v, want domain-busy", res)
	}
	if row := f.storedTarget(target.ID); row.Repo != target.Repo {
		t.Fatalf("target = %q, want it at %q", row.Repo, target.Repo)
	}
}

func TestARepositoryServingEveryDomainIsNotMovedWhileAnyDomainIsBusy(t *testing.T) {
	f := newPlacementFixture(t)
	pool := localPlace("Pool", "pool")
	pool.Folders = map[string]string{"containers": "", "vms": ""}
	pool = f.storePlace(pool)
	repo, err := f.st.CreatePlaceRepo(pool.ID, "", "")
	if err != nil {
		t.Fatal(err)
	}
	f.holdDomain("files", "backup")

	res := f.do(http.MethodPatch, "/api/places/"+pool.ID, map[string]any{"address": map[string]string{"path": "moved"}})

	if res["ok"] != false || res["code"] != "domain-busy" {
		t.Fatalf("PATCH = %v, want domain-busy", res)
	}
	if row, err := f.st.GetNamedRepo(repo.ID); err != nil || row.Repo != repo.Repo {
		t.Fatalf("repository = %+v, %v, want it at %q", row, err, repo.Repo)
	}
}

func TestADomainPathIsNotAdoptedOntoANewAddressWhileItsBackupRuns(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.storePlace(localPlace("NAS", "nas"))
	f.holdDomain("flash", "backup")

	if res := f.adopt(nas.ID, "", "flash"); res["ok"] != false || res["code"] != "domain-busy" {
		t.Fatalf("adopt = %v, want domain-busy", res)
	}
	if homes, err := f.st.DomainPlaces(); err != nil || homes["flash"] != "" {
		t.Fatalf("homes = %v, %v, want flash without a home place", homes, err)
	}
}

func TestATargetIsNotAdoptedOntoANewAddressWhileItsCopyRuns(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	loose := f.target("vms", "Old", "s3:https://s3.example.com/old/vms")
	f.eng.opens["s3:https://s3.example.com/bucket/vms"] = false
	f.holdDomain("vms", "replicate")

	if res := f.adopt(b2.ID, loose.ID, ""); res["ok"] != false || res["code"] != "domain-busy" {
		t.Fatalf("adopt = %v, want domain-busy", res)
	}
	if row := f.storedTarget(loose.ID); row.PlaceID != "" || row.Repo != loose.Repo {
		t.Fatalf("row = %+v, want it untouched", row)
	}
}

func TestATargetAtThePlacesAddressIsAdoptedWhileItsCopyRuns(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	loose := f.target("containers", "B2", "s3:https://s3.example.com/bucket/container")
	f.holdDomain("containers", "replicate")

	if res := f.adopt(b2.ID, loose.ID, ""); res["ok"] != true {
		t.Fatalf("adopt = %v, want the row at its own address to join", res)
	}
}
