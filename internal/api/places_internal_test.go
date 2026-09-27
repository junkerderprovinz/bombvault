package api

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAPlaceWriteKeepsTheCredentialSetsItDoesNotTouch(t *testing.T) {
	f := newPlacementFixture(t)
	pull := CloudCredSet{ID: "pull", Name: "Pull source", CloudCreds: CloudCreds{S3KeyID: "k1", S3Secret: "s1"}}
	if err := f.svc.SetCloudCredSets([]CloudCredSet{pull}); err != nil {
		t.Fatal(err)
	}
	own := CloudCredSet{ID: "b2", Name: "B2", CloudCreds: CloudCreds{S3KeyID: "k2", S3Secret: "s2"}}
	p := store.Place{Name: "B2", Provider: "b2", Kind: "s3", Base: "s3:https://s3.example.com/bucket",
		Folders: map[string]string{"containers": "container"}, CredsRef: own.ID, Enabled: true}

	stored, err := f.svc.writePlace(store.PlaceWrite{Place: p}, func(sets []CloudCredSet) []CloudCredSet {
		return append(sets, own)
	})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	sets, err := f.svc.decodeCloudCredSets(settings)
	if err != nil || len(sets) != 2 || sets[0] != pull || sets[1] != own {
		t.Fatalf("sets = %+v, %v, want the pull source's set untouched and the place's added", sets, err)
	}
	if stored.ID == "" || stored.CredsRef != own.ID {
		t.Fatalf("place = %+v", stored)
	}
}

func TestAPlaceWriteThatLeavesTheCredentialSetsAsTheyWereDoesNotRewriteThem(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "a", Name: "A", CloudCreds: CloudCreds{S3KeyID: "k"}}}); err != nil {
		t.Fatal(err)
	}
	before, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	p := store.Place{Name: "NAS", Provider: "share", Kind: "local", Base: "remotes/nas", Folders: map[string]string{"vms": "vms"}, Enabled: true}
	if _, err := f.svc.writePlace(store.PlaceWrite{Place: p}, func(sets []CloudCredSet) []CloudCredSet { return sets }); err != nil {
		t.Fatal(err)
	}
	after, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if after.CloudCredSets != before.CloudCredSets {
		t.Fatal("an edit that changed nothing encrypted the credential sets again")
	}
}

func TestCredentialWritersWaitWhileAPlaceWriteHoldsTheSets(t *testing.T) {
	f := newPlacementFixture(t)
	f.svc.credSetsMu.Lock()
	done := make(chan error, 2)
	go func() { done <- f.svc.SetCloudCredSets([]CloudCredSet{{ID: "a", Name: "A"}}) }()
	go func() {
		done <- f.svc.editCloudCredSets(func(sets []CloudCredSet) []CloudCredSet {
			return append(sets, CloudCredSet{ID: "b", Name: "B"})
		})
	}()
	select {
	case err := <-done:
		t.Fatalf("a credential writer went ahead during a place write: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	f.svc.credSetsMu.Unlock()
	for range 2 {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("a credential writer never finished")
		}
	}
}

func providerOf(t *testing.T, id string) places.Provider {
	t.Helper()
	p, ok := places.ProviderByID(id)
	if !ok {
		t.Fatalf("no provider %s", id)
	}
	return p
}

func TestAPlaceFormFillsACredentialSetTheWayItsKindReadsIt(t *testing.T) {
	dav := withPlaceCreds(CloudCredSet{ID: "dav", Name: "Cloud", Kind: "webdav"}, places.CredsFromFields(providerOf(t, "nextcloud"),
		map[string]string{"url": "https://cloud.example.com", "user": "anna", "password": "app-pass"}))
	want := CloudCredSet{ID: "dav", Name: "Cloud", Kind: "webdav", WebDAVURL: "https://cloud.example.com/remote.php/dav/files/anna/",
		WebDAVVendor: "nextcloud", WebDAVUser: "anna", WebDAVPass: "app-pass"}
	if dav != want {
		t.Fatalf("webdav set = %+v\nwant %+v", dav, want)
	}
	r2 := withPlaceCreds(CloudCredSet{ID: "r2"}, places.CredsFromFields(providerOf(t, "r2"),
		map[string]string{"keyId": "K", "secret": "S", "account": "abc"}))
	if r2.S3KeyID != "K" || r2.S3Secret != "S" || r2.S3Region != "auto" {
		t.Fatalf("r2 set = %+v, want the key and the signing region auto", r2)
	}
}

func TestAPlaceFormKeepsWhatItLeavesBlank(t *testing.T) {
	stored := CloudCredSet{ID: "b2", Name: "B2", Kind: "s3",
		CloudCreds: CloudCreds{S3KeyID: "k1", S3Secret: "s1", S3Region: "us-west-004", S3StorageClass: "STANDARD"}}
	typed := places.CredsFromFields(providerOf(t, "b2"), map[string]string{"keyId": "k2", "secret": ""})

	got := withPlaceCreds(stored, overlayCreds(placeCredsOf(stored), typed))

	want := stored
	want.S3KeyID = "k2"
	if got != want {
		t.Fatalf("set = %+v\nwant %+v", got, want)
	}
}

func TestThePlacesListShowsWhatEachPlaceIsUsedForFromTheDatabaseAlone(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	target := f.placeTarget(b2, "containers", "")
	f.listing("containers", target.ID, 100, copiesRow("container:nginx", 4, 90))
	nginx := f.container("nginx", "")
	f.backupRun(nginx.ID, 100)

	res := f.do(http.MethodGet, "/api/places", nil)

	unraid := placeByName(t, res, "Unraid")
	usage := unraid["usage"].(map[string]any)
	if !reflect.DeepEqual(usage["homeDomains"], []any{"containers", "vms", "files"}) || usage["repositories"] != float64(3) {
		t.Errorf("Unraid usage = %v, want three domain paths", usage)
	}
	if locked := unraid["locked"].(map[string]any); locked["containers"] != true || locked["vms"] != false {
		t.Errorf("Unraid locked = %v, want containers only", locked)
	}
	cloud := placeByName(t, res, "B2")
	usage = cloud["usage"].(map[string]any)
	if !reflect.DeepEqual(usage["copyDomains"], []any{"containers"}) || usage["copies"] != float64(4) || usage["repositories"] != float64(1) {
		t.Errorf("B2 usage = %v, want copies of containers, 4 snapshots", usage)
	}
	if locked := cloud["locked"].(map[string]any); locked["containers"] != true || locked["vms"] != false {
		t.Errorf("B2 locked = %v", locked)
	}
	if cloud["repository"] != false || cloud["kind"] != "s3" || cloud["offPremises"] != true {
		t.Errorf("B2 = %v", cloud)
	}
	if len(f.eng.lists) != 0 || len(f.eng.opened) != 0 {
		t.Fatalf("listing places reached restic: lists %v, opened %v", f.eng.lists, f.eng.opened)
	}
}

func TestAPlaceThatIsARepositoryListsTheDefaultsAndItemsOnIt(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.storePlace(store.Place{Name: "NAS", Provider: "share", Kind: string(places.KindLocal), Base: "nas",
		Folders: map[string]string{}, Enabled: true})
	repo := f.namedRepo("NAS", "nas")
	f.linkRow(repo.ID, nas, "", "")
	f.setDefault("containers", repo.ID)
	f.container("nginx", repo.ID)

	got := placeByName(t, f.do(http.MethodGet, "/api/places", nil), "NAS")

	usage := got["usage"].(map[string]any)
	if got["repository"] != true || !reflect.DeepEqual(usage["defaults"], []any{"containers"}) || usage["items"] != float64(1) {
		t.Fatalf("NAS = %v, want a repository holding the containers default and one item", got)
	}
}

func TestThePlacesListShowsCredentialsWithoutTheirSecrets(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCreds(CloudCreds{S3KeyID: "shared-key", S3Secret: "shared-secret"}); err != nil {
		t.Fatal(err)
	}
	own := CloudCredSet{ID: "set-b2", Name: "B2", Kind: "s3",
		CloudCreds: CloudCreds{S3KeyID: "own-key", S3Secret: "own-secret", S3Region: "eu-central-003"}}
	if err := f.svc.SetCloudCredSets([]CloudCredSet{own}); err != nil {
		t.Fatal(err)
	}
	b2 := s3Place("B2", "s3:https://s3.example.com/bucket")
	b2.CredsRef = own.ID
	f.storePlace(b2)
	f.storePlace(s3Place("MinIO", "s3:http://minio.lan:9000/backups"))
	f.storePlace(localPlace("Unraid", "backups"))

	res := f.do(http.MethodGet, "/api/places", nil)

	want := map[string]any{
		"B2":     map[string]any{"shared": false, "fields": map[string]any{"keyId": "own-key", "region": "eu-central-003"}, "set": []any{"secret"}},
		"MinIO":  map[string]any{"shared": true, "fields": map[string]any{"keyId": "shared-key"}, "set": []any{"secret"}},
		"Unraid": map[string]any{"shared": false, "fields": map[string]any{}, "set": []any{}},
	}
	for name, creds := range want {
		if got := placeByName(t, res, name)["creds"]; !reflect.DeepEqual(got, creds) {
			t.Errorf("%s creds = %v, want %v", name, got, creds)
		}
	}
	body, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "own-secret") || strings.Contains(string(body), "shared-secret") {
		t.Fatalf("the places list gave away a secret: %s", body)
	}
}

func TestAPlaceNamingAMissingSetShowsTheSharedCredentialsItRunsOn(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCreds(CloudCreds{S3KeyID: "shared-key", S3Secret: "shared-secret"}); err != nil {
		t.Fatal(err)
	}
	b2 := s3Place("B2", "s3:https://s3.example.com/bucket")
	b2.CredsRef = "gone"
	f.storePlace(b2)

	got := placeByName(t, f.do(http.MethodGet, "/api/places", nil), "B2")["creds"]

	want := map[string]any{"shared": true, "fields": map[string]any{"keyId": "shared-key"}, "set": []any{"secret"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("creds = %v, want %v", got, want)
	}
}

func TestAFailedCopyRunToAPlaceOutranksAnEarlierSuccess(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	target := f.placeTarget(b2, "vms", "")
	lastTest := func() any { return placeByName(t, f.do(http.MethodGet, "/api/places", nil), "B2")["lastTest"] }
	run := func(id string, at int64, errText string) {
		t.Helper()
		row, err := f.st.RecordOffsiteRunForTarget("vms", id, at)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.st.FinishOffsiteRun(row, errText == "", errText); err != nil {
			t.Fatal(err)
		}
	}

	if got := lastTest(); got != nil {
		t.Fatalf("lastTest before any run = %v", got)
	}
	run(target.ID, 100, "")
	if got := lastTest(); !reflect.DeepEqual(got, map[string]any{"at": float64(100), "ok": true, "source": "run"}) {
		t.Fatalf("lastTest after a success = %v", got)
	}
	run(target.ID, 200, "timeout")
	if got := lastTest(); !reflect.DeepEqual(got, map[string]any{"at": float64(200), "ok": false, "error": "timeout", "source": "run"}) {
		t.Fatalf("lastTest after a failure = %v", got)
	}
	run(target.ID, 300, "connection refused")
	if got := lastTest(); !reflect.DeepEqual(got, map[string]any{"at": float64(300), "ok": false, "error": "connection refused", "source": "run"}) {
		t.Fatalf("lastTest after a second failure = %v, want the newest with its reason", got)
	}
}

func TestTheNewestFailureAmongAPlacesTargetsIsItsLastRun(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	vms := f.placeTarget(b2, "vms", "")
	flash := f.placeTarget(b2, "flash", "")
	for _, r := range []struct {
		domain, id string
		at         int64
		errText    string
	}{{"vms", vms.ID, 100, "timeout"}, {"flash", flash.ID, 300, "access denied"}} {
		row, err := f.st.RecordOffsiteRunForTarget(r.domain, r.id, r.at)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.st.FinishOffsiteRun(row, false, r.errText); err != nil {
			t.Fatal(err)
		}
	}

	got := placeByName(t, f.do(http.MethodGet, "/api/places", nil), "B2")["lastTest"]

	if !reflect.DeepEqual(got, map[string]any{"at": float64(300), "ok": false, "error": "access denied", "source": "run"}) {
		t.Fatalf("lastTest = %v, want the flash failure at 300", got)
	}
}

func TestRowsWithoutAPlaceWaitInTheirOwnGroup(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	loose := f.target("vms", "Old NAS", "remotes/oldnas/vms")
	nas := f.namedRepo("NAS", "nas")

	rows := rowsOf(f.do(http.MethodGet, "/api/places", nil)["unplaced"])

	want := []map[string]any{
		{"rowId": "", "domain": "flash", "role": "path", "name": "", "repo": "user/bombvault/flash", "immutable": false, "protectable": false, "items": float64(1)},
		{"rowId": "", "domain": "config", "role": "path", "name": "", "repo": "user/bombvault/config", "immutable": false, "protectable": false, "items": float64(1)},
		{"rowId": loose.ID, "domain": "vms", "role": "target", "name": "Old NAS", "repo": "remotes/oldnas/vms", "immutable": false, "protectable": false, "items": float64(0)},
		{"rowId": nas.ID, "domain": "", "role": "repository", "name": "NAS", "repo": "nas", "immutable": false, "protectable": false, "items": float64(0)},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("unplaced = %v, want %v", rows, want)
	}
}

func TestTheCatalogRouteServesEveryProviderInTileOrder(t *testing.T) {
	f := newPlacementFixture(t)
	providers := rowsOf(f.do(http.MethodGet, "/api/places/catalog", nil)["providers"])
	if len(providers) != len(places.Catalog) {
		t.Fatalf("%d providers, want %d", len(providers), len(places.Catalog))
	}
	for i, p := range places.Catalog {
		if providers[i]["id"] != p.ID || providers[i]["kind"] != string(p.Kind) || providers[i]["group"] != string(p.Group) {
			t.Errorf("provider %d = %v, want %s", i, providers[i], p.ID)
		}
	}
	b2 := providers[slices.IndexFunc(places.Catalog, func(p places.Provider) bool { return p.ID == "b2" })]
	if !slices.ContainsFunc(rowsOf(b2["fields"]), func(field map[string]any) bool { return field["secret"] == true }) {
		t.Fatalf("B2 fields = %v, want its key marked secret", b2["fields"])
	}
}

func TestAddingAFolderPlaceWritesItWithoutCredentials(t *testing.T) {
	f := newPlacementFixture(t)
	asked := f.probeAnswers(places.ProbeResult{OK: true, Base: "nas"})

	res := f.do(http.MethodPost, "/api/places", map[string]any{
		"provider": "unraid-folder", "fields": map[string]string{"path": "nas"}, "name": "NAS",
	})

	if res["ok"] != true {
		t.Fatalf("POST /api/places = %v", res)
	}
	if len(*asked) != 1 || (*asked)[0].Fields["path"] != "nas" {
		t.Fatalf("the probe was asked %+v", *asked)
	}
	all, err := f.st.ListPlaces()
	if err != nil || len(all) != 1 {
		t.Fatalf("places = %+v, %v", all, err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	p := all[0]
	if p.Base != "nas" || p.Kind != "local" || p.CredsRef != "" || p.OffPremises ||
		p.RetentionKeepDaily != settings.RetentionKeepDaily || !maps.Equal(p.Folders, places.DefaultFolders()) {
		t.Fatalf("place = %+v", p)
	}
	if view := res["place"].(map[string]any); view["id"] != p.ID || view["name"] != "NAS" {
		t.Fatalf("answer = %v", view)
	}
}

func TestAddingACloudPlaceKeepsItsKeysInASetOfItsOwn(t *testing.T) {
	f := newPlacementFixture(t)
	base := "s3:https://s3.eu-central-1.wasabisys.com/bv"
	f.probeAnswers(places.ProbeResult{OK: true, Base: base})

	res := f.do(http.MethodPost, "/api/places", map[string]any{
		"provider": "wasabi", "name": "Wasabi",
		"fields": map[string]string{"keyId": "AKIA1", "secret": "s3cret", "region": "eu-central-1", "bucket": "bv"},
	})

	all, err := f.st.ListPlaces()
	if res["ok"] != true || err != nil || len(all) != 1 {
		t.Fatalf("POST /api/places = %v; places %+v, %v", res, all, err)
	}
	p := all[0]
	if p.Base != base || !p.OffPremises || p.CredsRef == "" {
		t.Fatalf("place = %+v", p)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	sets, err := f.svc.decodeCloudCredSets(settings)
	i := slices.IndexFunc(sets, func(c CloudCredSet) bool { return c.ID == p.CredsRef })
	if err != nil || i < 0 || sets[i].S3KeyID != "AKIA1" || sets[i].S3Secret != "s3cret" ||
		sets[i].S3Region != "eu-central-1" || sets[i].Kind != "s3" || sets[i].Name != "Wasabi" {
		t.Fatalf("sets = %+v, %v", sets, err)
	}
	creds := res["place"].(map[string]any)["creds"].(map[string]any)
	if creds["shared"] != false || !reflect.DeepEqual(creds["set"], []any{"secret"}) || creds["fields"].(map[string]any)["keyId"] != "AKIA1" {
		t.Fatalf("creds = %v, want the key id and no secret", creds)
	}
}

func TestANextcloudPlaceIsReachedThroughARemoteNamedAfterIt(t *testing.T) {
	f := newPlacementFixture(t)
	f.probeAnswers(places.ProbeResult{OK: true, Base: "rclone:" + places.RemoteName("probe") + ":bombvault"})

	res := f.do(http.MethodPost, "/api/places", map[string]any{
		"provider": "nextcloud", "name": "Cloud", "offPremises": true,
		"fields": map[string]string{"url": "https://cloud.example.com", "user": "anna", "password": "app-pass", "path": "bombvault"},
	})

	all, err := f.st.ListPlaces()
	if res["ok"] != true || err != nil || len(all) != 1 {
		t.Fatalf("POST /api/places = %v; places %+v, %v", res, all, err)
	}
	p := all[0]
	if want := "rclone:" + places.RemoteName(p.ID) + ":bombvault"; p.Base != want {
		t.Fatalf("base = %q, want %q", p.Base, want)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	sets, err := f.svc.decodeCloudCredSets(settings)
	i := slices.IndexFunc(sets, func(c CloudCredSet) bool { return c.ID == p.CredsRef })
	if err != nil || i < 0 || sets[i].Kind != "webdav" || sets[i].WebDAVURL != "https://cloud.example.com/remote.php/dav/files/anna/" ||
		sets[i].WebDAVVendor != "nextcloud" || sets[i].WebDAVUser != "anna" || sets[i].WebDAVPass != "app-pass" {
		t.Fatalf("sets = %+v, %v", sets, err)
	}
}

func TestAnAddressThatHoldsARepositoryBecomesAPlaceThatIsOne(t *testing.T) {
	f := newPlacementFixture(t)
	f.probeAnswers(places.ProbeResult{OK: true, Base: "rest:https://nas:8000/tower", RepoIDs: map[string]string{"": "id-tower"}})
	body := map[string]any{
		"provider": "rest-server", "name": "Tower", "offPremises": false,
		"fields": map[string]string{"url": "https://nas:8000", "user": "tower", "password": "pw"},
	}

	res := f.do(http.MethodPost, "/api/places", body)

	view, _ := res["place"].(map[string]any)
	if res["ok"] != true || view["repository"] != true || view["base"] != "rest:https://nas:8000/tower" {
		t.Fatalf("POST /api/places = %v, want a place that is itself a repository", res)
	}
	folders, _ := view["folders"].(map[string]any)
	for _, d := range places.Domains {
		if folder, ok := folders[d]; !ok || folder != "" {
			t.Fatalf("folders = %v, want every domain at the base", folders)
		}
	}
	body["name"], body["folders"] = "Tower 2", map[string]string{"containers": "container"}
	if res := f.do(http.MethodPost, "/api/places", body); res["ok"] != false || res["code"] != "place-is-repository" {
		t.Fatalf("POST with a folder below the repository = %v, want place-is-repository", res)
	}
}

func TestANewPlaceTakesNoFolderBesideItsBase(t *testing.T) {
	f := newPlacementFixture(t)
	f.probeAnswers(places.ProbeResult{OK: true, Base: "nas"})
	res := f.do(http.MethodPost, "/api/places", map[string]any{
		"provider": "unraid-folder", "fields": map[string]string{"path": "nas"}, "name": "NAS",
		"folders": map[string]string{"containers": "", "vms": "vms"},
	})
	if res["ok"] != false || res["code"] != "place-is-repository" {
		t.Fatalf("POST /api/places = %v, want place-is-repository", res)
	}
	if all, err := f.st.ListPlaces(); err != nil || len(all) != 0 {
		t.Fatalf("places = %+v, %v, want none", all, err)
	}
}

func TestAPlaceNameIsTakenOnce(t *testing.T) {
	f := newPlacementFixture(t)
	f.probeAnswers(places.ProbeResult{OK: true, Base: "nas"})
	body := map[string]any{"provider": "unraid-folder", "fields": map[string]string{"path": "nas"}, "name": "NAS"}
	f.do(http.MethodPost, "/api/places", body)
	if res := f.do(http.MethodPost, "/api/places", body); res["ok"] != false || res["code"] != "place-name-taken" {
		t.Fatalf("second POST = %v, want place-name-taken", res)
	}
}

func TestAFailedProbeAddsNothing(t *testing.T) {
	f := newPlacementFixture(t)
	f.probeAnswers(places.ProbeResult{Code: "direct-access-denied", Error: "the key cannot read this place"})

	res := f.do(http.MethodPost, "/api/places", map[string]any{
		"provider": "wasabi", "name": "Wasabi", "fields": map[string]string{"keyId": "AKIA1", "secret": "s3cret"},
	})

	if res["ok"] != false || res["code"] != "place-probe-failed" || res["probe"].(map[string]any)["code"] != "direct-access-denied" {
		t.Fatalf("POST /api/places = %v", res)
	}
	all, err := f.st.ListPlaces()
	if err != nil || len(all) != 0 {
		t.Fatalf("places = %+v, %v, want none", all, err)
	}
	settings, err := f.st.GetSettings()
	if err != nil || settings.CloudCredSets != "" {
		t.Fatalf("a failed add wrote credentials: %q, %v", settings.CloudCredSets, err)
	}
}

func TestADeviceHasToBeToldWhereItStands(t *testing.T) {
	f := newPlacementFixture(t)
	f.probeAnswers(places.ProbeResult{OK: true, Base: "remotes/syno/bombvault"})
	body := map[string]any{"provider": "synology", "name": "Synology", "fields": map[string]string{"path": "remotes/syno/bombvault"}}
	if res := f.do(http.MethodPost, "/api/places", body); res["ok"] != false || res["code"] != nil {
		t.Fatalf("POST without the answer = %v, want a plain refusal", res)
	}
	body["offPremises"] = true
	if res := f.do(http.MethodPost, "/api/places", body); res["ok"] != true || res["place"].(map[string]any)["offPremises"] != true {
		t.Fatalf("POST with the answer = %v", res)
	}
}

func TestTheProbeRouteAnswersWithTheProbe(t *testing.T) {
	f := newPlacementFixture(t)
	f.probeAnswers(places.ProbeResult{OK: true, Base: "nas", Folders: map[string]places.FolderState{"containers": places.FolderEmpty}})
	res := f.do(http.MethodPost, "/api/places/probe", map[string]any{"provider": "unraid-folder", "fields": map[string]string{"path": "nas"}})
	if res["ok"] != true || res["base"] != "nas" || res["folders"].(map[string]any)["containers"] != "empty" {
		t.Fatalf("POST /api/places/probe = %v", res)
	}
}

func TestAProbeOfAnUnknownProviderIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	if res := f.do(http.MethodPost, "/api/places/probe", map[string]any{"provider": "dropbox"}); res["ok"] != false || res["error"] == nil {
		t.Fatalf("POST /api/places/probe = %v, want a refusal", res)
	}
	if len(f.eng.opened) != 0 {
		t.Fatalf("a refused probe opened %v", f.eng.opened)
	}
}

func TestEditingAPlaceMirrorsIntoTheRowsItHolds(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	target := f.placeTarget(b2, "containers", "")

	res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"name": "B2 EU", "retentionKeepLast": 5, "limitUpload": 800,
		"storageClass": "standard_ia", "growthBudgetGb": 50, "offPremises": false})

	view, _ := res["place"].(map[string]any)
	if res["ok"] != true || view["name"] != "B2 EU" || view["offPremises"] != false || view["storageClass"] != "STANDARD_IA" {
		t.Fatalf("PATCH = %v", res)
	}
	row := f.storedTarget(target.ID)
	if row.RetentionKeepLast != 5 || row.LimitUpload != 800 || row.StorageClass != "STANDARD_IA" || row.GrowthBudgetGB != 50 {
		t.Fatalf("target = %+v, want the place's retention, limit, class and budget", row)
	}
}

func TestAStorageClassARestoreCannotReadIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	if res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"storageClass": "DEEP_ARCHIVE"}); res["ok"] != false || res["code"] != nil {
		t.Fatalf("PATCH = %v, want a plain refusal", res)
	}
	if p, err := f.st.GetPlace(b2.ID); err != nil || p.StorageClass != "" {
		t.Fatalf("place = %+v, %v, want its class unchanged", p, err)
	}
}

func TestAnUnknownPlaceIsNotFound(t *testing.T) {
	f := newPlacementFixture(t)
	if code, res := f.doStatus(http.MethodPatch, "/api/places/nosuchplace", map[string]any{"name": "x"}); code != http.StatusNotFound || res["ok"] != false {
		t.Fatalf("PATCH an unknown place = %d %v, want 404", code, res)
	}
}

func TestAHomePlaceStaysSwitchedOn(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "containers")
	if res := f.do(http.MethodPatch, "/api/places/"+unraid.ID, map[string]any{"enabled": false}); res["ok"] != false || res["code"] != "place-home-domain" {
		t.Fatalf("switching a home place off = %v", res)
	}
	if p, err := f.st.GetPlace(unraid.ID); err != nil || !p.Enabled {
		t.Fatalf("place = %+v, %v, want it still on", p, err)
	}
}

func TestOnlyARemotePlaceTakesTheAppendOnlySwitch(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"))
	if res := f.do(http.MethodPatch, "/api/places/"+unraid.ID, map[string]any{"immutable": true}); res["ok"] != false || res["code"] != "place-no-append-only" {
		t.Fatalf("append-only on a local place = %v, want place-no-append-only", res)
	}
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	target := f.placeTarget(b2, "containers", "")
	if res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"immutable": true}); res["ok"] != true {
		t.Fatalf("append-only on a remote place = %v", res)
	}
	if row := f.storedTarget(target.ID); !row.Immutable {
		t.Fatalf("target = %+v, want it append-only", row)
	}
}

func TestLoweringTheRetentionOfAPlaceWarnsForADirectRepositoryInUse(t *testing.T) {
	f := newPlacementFixture(t)
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.RetentionKeepLast = 10
	b2 := f.storePlace(p)
	target := f.placeTarget(b2, "containers", "")
	direct := f.direct(target)
	f.container("nginx", direct.ID)

	res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"retentionKeepLast": 3})

	warnings := rowsOf(res["warnings"])
	if res["ok"] != true || len(warnings) != 1 || warnings[0]["code"] != "direct-retention-lowered" || warnings[0]["items"] != float64(1) {
		t.Fatalf("PATCH = %v, want one direct-retention-lowered warning for one item", res)
	}
}

func TestSwitchingAppendOnlyOffWarnsForADirectRepositoryInUse(t *testing.T) {
	f := newPlacementFixture(t)
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.Immutable = true
	b2 := f.storePlace(p)
	target := f.placeTarget(b2, "containers", "")
	direct := f.direct(target)
	f.container("nginx", direct.ID)

	res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"immutable": false})

	warnings := rowsOf(res["warnings"])
	if res["ok"] != true || len(warnings) != 1 || warnings[0]["code"] != "direct-append-only-off" || warnings[0]["targetId"] != target.ID {
		t.Fatalf("PATCH = %v, want one direct-append-only-off warning for the target", res)
	}
}

func TestARenameOntoAnotherPlacesNameIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"))
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	if res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"name": "Unraid"}); res["ok"] != false || res["code"] != "place-name-taken" {
		t.Fatalf("PATCH = %v, want place-name-taken", res)
	}
}

func TestRenamingAFolderWithNothingInItMovesTheDomainPath(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	folders := maps.Clone(unraid.Folders)
	folders["vms"] = "virtual"

	if res := f.do(http.MethodPatch, "/api/places/"+unraid.ID, map[string]any{"folders": folders}); res["ok"] != true {
		t.Fatalf("PATCH = %v", res)
	}
	if settings, err := f.st.GetSettings(); err != nil || settings.VMsPath != "backups/virtual" {
		t.Fatalf("vms path = %q, %v", settings.VMsPath, err)
	}
}

func TestRenamingAFolderAwayFromBackupsIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nginx := f.container("nginx", "")
	f.backupRun(nginx.ID, 100)
	if err := f.st.AddRepoStat(store.RepoStat{Domain: "containers", Source: "local", At: 100, Snapshots: 9}); err != nil {
		t.Fatal(err)
	}
	folders := maps.Clone(unraid.Folders)
	folders["containers"] = "ct"

	res := f.do(http.MethodPatch, "/api/places/"+unraid.ID, map[string]any{"folders": folders})

	if res["ok"] != false || res["code"] != "place-location-established" || res["snapshots"] != float64(9) ||
		!reflect.DeepEqual(res["domains"], []any{"containers"}) {
		t.Fatalf("PATCH = %v, want place-location-established with 9 snapshots of containers", res)
	}
	if settings, err := f.st.GetSettings(); err != nil || settings.ContainersPath != "backups/containers" {
		t.Fatalf("containers path = %q, %v, want it unmoved", settings.ContainersPath, err)
	}
}

func TestAHandMadeMoveOfTheWholePlaceIsAccepted(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nginx := f.container("nginx", "")
	f.backupRun(nginx.ID, 100)
	f.eng.ids[f.domainPath("containers")] = "r1"
	f.eng.ids[f.localRepo("moved/containers")] = "r1"

	res := f.do(http.MethodPatch, "/api/places/"+unraid.ID, map[string]any{"address": map[string]string{"path": "moved"}})

	if res["ok"] != true || res["place"].(map[string]any)["base"] != "moved" {
		t.Fatalf("PATCH = %v", res)
	}
	settings, err := f.st.GetSettings()
	if err != nil || settings.ContainersPath != "moved/containers" || settings.VMsPath != "moved/vms" {
		t.Fatalf("paths = %q %q, %v", settings.ContainersPath, settings.VMsPath, err)
	}
}

func TestAPlaceThatIsARepositoryTakesNoFolder(t *testing.T) {
	f := newPlacementFixture(t)
	p := s3Place("B2 root", "s3:https://s3.example.com/bucket")
	p.Folders = map[string]string{"containers": ""}
	root := f.storePlace(p)
	res := f.do(http.MethodPatch, "/api/places/"+root.ID, map[string]any{"folders": map[string]string{"containers": "container"}})
	if res["ok"] != false || res["code"] != "place-is-repository" {
		t.Fatalf("PATCH = %v, want place-is-repository", res)
	}
}

func TestAFolderBesideTheBaseItselfIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.storePlace(localPlace("NAS", "nas"))
	folders := maps.Clone(nas.Folders)
	folders["vms"] = ""
	if res := f.do(http.MethodPatch, "/api/places/"+nas.ID, map[string]any{"folders": folders}); res["ok"] != false || res["code"] != "place-is-repository" {
		t.Fatalf("PATCH = %v, want place-is-repository", res)
	}
	if p, err := f.st.GetPlace(nas.ID); err != nil || p.Folders["vms"] != "vms" {
		t.Fatalf("place = %+v, %v, want its folders unchanged", p, err)
	}
}

func TestAnUnreachableNewAddressLeavesThePlaceAlone(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	f.placeTarget(b2, "containers", "")
	moved := "s3:https://s3.example.com/other/container"
	f.eng.opens[moved] = false
	f.eng.openErr[moved] = errors.New("dial tcp 203.0.113.9:443: connect: connection refused")

	res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"address": map[string]string{"endpoint": "s3.example.com", "bucket": "other"}})

	if res["ok"] != false || res["code"] != "place-probe-failed" {
		t.Fatalf("PATCH = %v, want place-probe-failed", res)
	}
	if p, err := f.st.GetPlace(b2.ID); err != nil || p.Base != b2.Base {
		t.Fatalf("place = %+v, %v, want its base unchanged", p, err)
	}
}

func TestAFolderCannotTurnAPlaceIntoARepository(t *testing.T) {
	f := newPlacementFixture(t)
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.Folders = map[string]string{"containers": "container"}
	b2 := f.storePlace(p)
	target := f.placeTarget(b2, "containers", "")
	// The bucket root is empty, so only the folder rule can refuse.
	f.eng.opens["s3:https://s3.example.com/bucket"] = false

	res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"folders": map[string]string{"containers": ""}})

	if res["ok"] != false || res["code"] != "place-folder-blank" {
		t.Fatalf("PATCH = %v, want place-folder-blank", res)
	}
	if row := f.storedTarget(target.ID); row.Repo != target.Repo {
		t.Fatalf("target = %q, want it at %q", row.Repo, target.Repo)
	}
}

func TestAPlaceCannotMoveIntoAnotherRepository(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "vms")
	f.namedRepo("NAS", "nas/containers")

	res := f.do(http.MethodPatch, "/api/places/"+unraid.ID, map[string]any{"address": map[string]string{"path": "nas/containers"}})

	if res["ok"] != false || res["code"] != "nested-location" {
		t.Fatalf("PATCH = %v, want nested-location", res)
	}
	if settings, err := f.st.GetSettings(); err != nil || settings.VMsPath != "backups/vms" {
		t.Fatalf("vms path = %q, %v, want it unmoved", settings.VMsPath, err)
	}
}

func TestAPlaceCannotMoveItsTargetsIntoAnotherRepository(t *testing.T) {
	f := newPlacementFixture(t)
	usb := f.storePlace(localPlace("USB", "usb"))
	target := f.placeTarget(usb, "flash", "")
	f.namedRepo("NAS", "nas")

	res := f.do(http.MethodPatch, "/api/places/"+usb.ID, map[string]any{"address": map[string]string{"path": "nas/usb"}})

	if res["ok"] != false || res["code"] != "nested-location" {
		t.Fatalf("PATCH = %v, want nested-location", res)
	}
	if row := f.storedTarget(target.ID); row.Repo != "usb/flash" {
		t.Fatalf("target = %q, want it unmoved", row.Repo)
	}
}

func TestAPlaceCannotMoveOntoAShareWithNothingMounted(t *testing.T) {
	f := newPlacementFixture(t)
	usb := f.storePlace(localPlace("USB", "usb"))
	if err := os.MkdirAll(filepath.FromSlash(f.root+"/remotes/nas/bombvault"), 0o750); err != nil {
		t.Fatal(err)
	}
	writeMountFixture(t, "/")

	res := f.do(http.MethodPatch, "/api/places/"+usb.ID, map[string]any{"address": map[string]string{"path": "remotes/nas/bombvault"}})

	probe, _ := res["probe"].(map[string]any)
	if res["ok"] != false || res["code"] != "place-probe-failed" || probe["error"] != "nothing is mounted at this share; mount it on the server first" {
		t.Fatalf("PATCH = %v, want place-probe-failed naming the missing mount once", res)
	}
	if p, err := f.st.GetPlace(usb.ID); err != nil || p.Base != "usb" {
		t.Fatalf("place = %+v, %v, want its base unchanged", p, err)
	}
}

func TestAnAddressFormMissingAFieldIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"address": map[string]string{"bucket": "other"}})
	if res["ok"] != false || res["code"] != nil || res["error"] != "the field endpoint is required" {
		t.Fatalf("PATCH = %v, want a plain refusal naming the missing endpoint", res)
	}
}

func TestAPlaceEditWaitsForTheOneInProgress(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	f.svc.placeEditMu.Lock()
	done := make(chan error, 1)
	go func() {
		keep := 5
		_, _, err := f.svc.patchPlace(context.Background(), b2.ID, patchPlaceBody{RetentionKeepLast: &keep})
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("a place edit went ahead during another: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	f.svc.placeEditMu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the place edit never finished")
	}
	if p, err := f.st.GetPlace(b2.ID); err != nil || p.RetentionKeepLast != 5 {
		t.Fatalf("place = %+v, %v, want the waiting edit saved", p, err)
	}
}

func TestAPlaceOnTheSharedCredentialsGetsASetOfItsOwnOnItsFirstChange(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCreds(CloudCreds{S3KeyID: "shared-id", S3Secret: "shared-secret"}); err != nil {
		t.Fatal(err)
	}
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	target := f.placeTarget(b2, "containers", "")
	f.probeAnswers(places.ProbeResult{OK: true, Base: b2.Base})

	res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"fields": map[string]string{"keyId": "own-id", "secret": ""}})

	p, err := f.st.GetPlace(b2.ID)
	if res["ok"] != true || err != nil || p.CredsRef == "" {
		t.Fatalf("PATCH = %v; place %+v, %v", res, p, err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	sets, err := f.svc.decodeCloudCredSets(settings)
	if err != nil || len(sets) != 1 || sets[0].ID != p.CredsRef || sets[0].S3KeyID != "own-id" ||
		sets[0].S3Secret != "shared-secret" || sets[0].Kind != "s3" {
		t.Fatalf("sets = %+v, %v, want one set with the new key and the shared secret", sets, err)
	}
	if shared, err := f.svc.CloudConfig(); err != nil || shared.S3KeyID != "shared-id" {
		t.Fatalf("shared credentials = %+v, %v, want them untouched", shared, err)
	}
	if row := f.storedTarget(target.ID); row.CredsRef != p.CredsRef {
		t.Fatalf("target = %+v, want it on the place's new set", row)
	}
}

func TestAPlaceWithASetOfItsOwnChangesItInPlace(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "b2-set", Name: "B2", CloudCreds: CloudCreds{S3KeyID: "k1", S3Secret: "s1"}}}); err != nil {
		t.Fatal(err)
	}
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.CredsRef = "b2-set"
	b2 := f.storePlace(p)
	f.placeTarget(b2, "containers", "")
	f.probeAnswers(places.ProbeResult{OK: true, Base: b2.Base})

	res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"fields": map[string]string{"keyId": "k2", "secret": "s2"}})

	stored, err := f.st.GetPlace(b2.ID)
	if res["ok"] != true || err != nil || stored.CredsRef != "b2-set" {
		t.Fatalf("PATCH = %v; place %+v, %v", res, stored, err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if sets, err := f.svc.decodeCloudCredSets(settings); err != nil || len(sets) != 1 || sets[0].S3KeyID != "k2" || sets[0].S3Secret != "s2" {
		t.Fatalf("sets = %+v, %v, want b2-set changed in place", sets, err)
	}
}

func TestANextcloudPlaceGivenANewUserPointsAtTheirFiles(t *testing.T) {
	f := newPlacementFixture(t)
	dav := f.storeDavPlace()
	f.probeAnswers(places.ProbeResult{OK: true, Base: dav.Base})

	res := f.do(http.MethodPatch, "/api/places/"+dav.ID, map[string]any{"fields": map[string]string{"user": "ben"}})

	settings, err := f.st.GetSettings()
	if res["ok"] != true || err != nil {
		t.Fatalf("PATCH = %v, %v", res, err)
	}
	sets, err := f.svc.decodeCloudCredSets(settings)
	if err != nil || len(sets) != 1 || sets[0].WebDAVUser != "ben" || sets[0].WebDAVURL != "https://cloud.example.com/remote.php/dav/files/ben/" {
		t.Fatalf("sets = %+v, %v, want ben and his files", sets, err)
	}
}

func TestASetListThatLeavesOutTheSetOfAPlaceKeepsIt(t *testing.T) {
	f := newPlacementFixture(t)
	f.storeDavPlace()
	oldBox := CloudCredSet{ID: "old-box", Name: "Old box", CloudCreds: CloudCreds{S3KeyID: "k", S3Secret: "s"}}
	if err := f.svc.SetCloudCredSets([]CloudCredSet{davSet(), oldBox}); err != nil {
		t.Fatal(err)
	}

	// A Pull page loaded before the place was made removes Old box.
	if err := f.svc.SetCloudCredSets(nil); err != nil {
		t.Fatal(err)
	}

	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if sets, err := f.svc.decodeCloudCredSets(settings); err != nil || !slices.Equal(sets, []CloudCredSet{davSet()}) {
		t.Fatalf("sets = %+v, %v, want the place's set kept whole and Old box gone", sets, err)
	}
}

func TestASetSomethingElseNamesIsForkedNotEdited(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "b2-set", Name: "B2", CloudCreds: CloudCreds{S3KeyID: "k1", S3Secret: "s1"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.CreatePullSource(store.PullSource{Name: "Old box", Repo: "s3:https://s3.example.com/other", CredsRef: "b2-set", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.CredsRef = "b2-set"
	b2 := f.storePlace(p)
	f.probeAnswers(places.ProbeResult{OK: true, Base: b2.Base})

	res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"fields": map[string]string{"keyId": "k2", "secret": ""}})

	stored, err := f.st.GetPlace(b2.ID)
	if res["ok"] != true || err != nil || stored.CredsRef == "b2-set" {
		t.Fatalf("PATCH = %v; place %+v, %v, want it on a set of its own", res, stored, err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	sets, err := f.svc.decodeCloudCredSets(settings)
	shared := slices.IndexFunc(sets, func(c CloudCredSet) bool { return c.ID == "b2-set" })
	own := slices.IndexFunc(sets, func(c CloudCredSet) bool { return c.ID == stored.CredsRef })
	if err != nil || shared < 0 || own < 0 || sets[shared].S3KeyID != "k1" || sets[own].S3KeyID != "k2" || sets[own].S3Secret != "s1" {
		t.Fatalf("sets = %+v, %v, want the pull source's set untouched and a fork with the new key", sets, err)
	}
}

func TestASetARemoteDomainPathRunsOnIsForkedNotEdited(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "b2-set", Name: "B2", CloudCreds: CloudCreds{S3KeyID: "k1", S3Secret: "s1"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.UpsertPrimaryRemoteTarget("vms", store.OffsiteTarget{Repo: "s3:https://s3.example.com/other/vms", CredsRef: "b2-set", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.CredsRef = "b2-set"
	b2 := f.storePlace(p)
	f.probeAnswers(places.ProbeResult{OK: true, Base: b2.Base})

	res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"fields": map[string]string{"keyId": "k2", "secret": ""}})

	stored, err := f.st.GetPlace(b2.ID)
	if res["ok"] != true || err != nil || stored.CredsRef == "b2-set" {
		t.Fatalf("PATCH = %v; place %+v, %v, want it on a set of its own", res, stored, err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	sets, err := f.svc.decodeCloudCredSets(settings)
	shared := slices.IndexFunc(sets, func(c CloudCredSet) bool { return c.ID == "b2-set" })
	if err != nil || shared < 0 || sets[shared].S3KeyID != "k1" {
		t.Fatalf("sets = %+v, %v, want the set of the vms path untouched", sets, err)
	}
}

func TestASetAnotherPlaceRunsOnIsForkedNotEdited(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "b2-set", Name: "B2", CloudCreds: CloudCreds{S3KeyID: "k1", S3Secret: "s1"}}}); err != nil {
		t.Fatal(err)
	}
	first, second := s3Place("B2", "s3:https://s3.example.com/bucket"), s3Place("B2 later", "s3:https://s3.example.com/bucket-2")
	first.CredsRef, second.CredsRef = "b2-set", "b2-set"
	b2 := f.storePlace(first)
	// The other place holds no row yet, so only the place itself names the set.
	later := f.storePlace(second)
	f.probeAnswers(places.ProbeResult{OK: true, Base: b2.Base})

	res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"fields": map[string]string{"keyId": "k2", "secret": ""}})

	if stored, err := f.st.GetPlace(b2.ID); res["ok"] != true || err != nil || stored.CredsRef == "b2-set" {
		t.Fatalf("PATCH = %v; place %+v, %v, want it on a set of its own", res, stored, err)
	}
	if other, err := f.st.GetPlace(later.ID); err != nil || other.CredsRef != "b2-set" {
		t.Fatalf("the other place = %+v, %v, want it still on b2-set", other, err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	sets, err := f.svc.decodeCloudCredSets(settings)
	shared := slices.IndexFunc(sets, func(c CloudCredSet) bool { return c.ID == "b2-set" })
	if err != nil || shared < 0 || sets[shared].S3KeyID != "k1" {
		t.Fatalf("sets = %+v, %v, want b2-set untouched", sets, err)
	}
}

func TestNewCredentialsThePlaceDoesNotOpenWithAreRefused(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "b2-set", Name: "B2", CloudCreds: CloudCreds{S3KeyID: "k1", S3Secret: "s1"}}}); err != nil {
		t.Fatal(err)
	}
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.CredsRef = "b2-set"
	b2 := f.storePlace(p)
	asked := f.probeAnswers(places.ProbeResult{Code: "direct-access-denied", Error: "the key cannot read this place"})

	res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"fields": map[string]string{"keyId": "k2", "secret": "s2"}})

	if res["ok"] != false || res["code"] != "place-probe-failed" || len(*asked) != 1 || (*asked)[0].PlaceID != b2.ID {
		t.Fatalf("PATCH = %v, probe asked %+v, want place-probe-failed from a probe of this place", res, *asked)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if sets, err := f.svc.decodeCloudCredSets(settings); err != nil || len(sets) != 1 || sets[0].S3KeyID != "k1" {
		t.Fatalf("sets = %+v, %v, want b2-set untouched", sets, err)
	}
}

func TestNewCredentialsAreTriedWhenTheAddressFormKeepsTheBase(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "b2-set", Name: "B2", CloudCreds: CloudCreds{S3KeyID: "k1", S3Secret: "s1"}}}); err != nil {
		t.Fatal(err)
	}
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.CredsRef = "b2-set"
	b2 := f.storePlace(p)
	f.probeAnswers(places.ProbeResult{Code: "direct-access-denied", Error: "the key cannot read this place"})

	res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{
		"address": map[string]string{"endpoint": "s3.example.com", "bucket": "bucket"},
		"fields":  map[string]string{"keyId": "k2", "secret": "s2"},
	})

	if res["ok"] != false || res["code"] != "place-probe-failed" {
		t.Fatalf("PATCH = %v, want place-probe-failed", res)
	}
}

func TestAMovedAddressIsOpenedWithTheCredentialsTheEditBrings(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "b2-set", Name: "B2", CloudCreds: CloudCreds{S3KeyID: "k1", S3Secret: "s1"}}}); err != nil {
		t.Fatal(err)
	}
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.CredsRef = "b2-set"
	b2 := f.storePlace(p)
	target := f.placeTarget(b2, "containers", "")
	moved := "s3:https://s3.example.com/other/container"
	f.eng.ids[target.Repo], f.eng.ids[moved] = "r1", "r1"
	f.eng.keys[moved] = "AWS_ACCESS_KEY_ID=k2"
	f.eng.openErr[moved] = errors.New("Access Denied")
	asked := f.probeAnswers(places.ProbeResult{Code: "direct-access-denied", Error: "the old bucket refuses the new key"})

	res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{
		"address": map[string]string{"endpoint": "s3.example.com", "bucket": "other"},
		"fields":  map[string]string{"keyId": "k2", "secret": "s2"},
	})

	if res["ok"] != true || len(*asked) != 0 {
		t.Fatalf("PATCH = %v, probe asked %+v, want the move accepted without a probe of the old base", res, *asked)
	}
	if row := f.storedTarget(target.ID); row.Repo != moved {
		t.Fatalf("target = %q, want it at %q", row.Repo, moved)
	}
}

func TestCredentialsTheFormLeavesAsTheyWereAreNeitherTriedNorForked(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCreds(CloudCreds{S3KeyID: "shared-id", S3Secret: "shared-secret"}); err != nil {
		t.Fatal(err)
	}
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	asked := f.probeAnswers(places.ProbeResult{Code: "direct-access-denied", Error: "the key cannot read this place"})

	res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"fields": map[string]string{"keyId": "shared-id", "secret": ""}})

	if res["ok"] != true || len(*asked) != 0 {
		t.Fatalf("PATCH = %v, probe asked %+v, want it saved untried", res, *asked)
	}
	if p, err := f.st.GetPlace(b2.ID); err != nil || p.CredsRef != "" {
		t.Fatalf("place = %+v, %v, want it still on the shared credentials", p, err)
	}
	if settings, err := f.st.GetSettings(); err != nil || settings.CloudCredSets != "" {
		t.Fatalf("credential sets = %q, %v, want none", settings.CloudCredSets, err)
	}
}

func TestADirectRepositoryThatNoLongerOpensKeepsItsOldCredentials(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "b2-set", Name: "B2", CloudCreds: CloudCreds{S3KeyID: "k1", S3Secret: "s1"}}}); err != nil {
		t.Fatal(err)
	}
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.CredsRef = "b2-set"
	b2 := f.storePlace(p)
	target := f.placeTarget(b2, "containers", "")
	direct := f.direct(target)
	f.linkRow(direct.ID, b2, "containers", "-direct")
	f.container("nginx", direct.ID)
	f.eng.keys[direct.Repo] = "AWS_ACCESS_KEY_ID=k1"
	f.probeAnswers(places.ProbeResult{OK: true, Base: b2.Base})

	res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"fields": map[string]string{"keyId": "k2", "secret": "s2"}})

	warnings := rowsOf(res["warnings"])
	if res["ok"] != true || len(warnings) != 1 || warnings[0]["code"] != "direct-creds-kept" {
		t.Fatalf("PATCH = %v, want one direct-creds-kept warning", res)
	}
	row, err := f.st.GetNamedRepo(direct.ID)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	sets, err := f.svc.decodeCloudCredSets(settings)
	kept := slices.IndexFunc(sets, func(c CloudCredSet) bool { return c.ID == row.CredsRef })
	if err != nil || kept < 0 || sets[kept].S3KeyID != "k1" {
		t.Fatalf("the direct repository runs on %q in %+v, %v, want a set with the old key", row.CredsRef, sets, err)
	}
}

func TestAHomePlaceIsNotRemovedAndTheAnswerSaysWhatHoldsIt(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "containers", "vms")

	res := f.do(http.MethodDelete, "/api/places/"+unraid.ID, nil)

	holders, _ := res["holders"].(map[string]any)
	homes := rowsOfStrings(holders["homeDomains"])
	slices.Sort(homes)
	if res["ok"] != false || res["code"] != "place-in-use" || !slices.Equal(homes, []string{"containers", "vms"}) {
		t.Fatalf("DELETE = %v, want place-in-use held by containers and vms", res)
	}
	for _, key := range []string{"defaults", "items", "directInUse"} {
		if !reflect.DeepEqual(holders[key], []any{}) {
			t.Errorf("holders %s = %v, want an empty list", key, holders[key])
		}
	}
	if _, err := f.st.GetPlace(unraid.ID); err != nil {
		t.Fatalf("the place is gone: %v", err)
	}
}

func TestARemovalRefusalNamesTheDefaultsAndItemsOnThePlace(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.storePlace(store.Place{Name: "NAS", Provider: "share", Kind: string(places.KindLocal), Base: "nas",
		Folders: map[string]string{}, Enabled: true})
	repo := f.namedRepo("NAS", "nas")
	f.linkRow(repo.ID, nas, "", "")
	f.setDefault("containers", repo.ID)
	f.container("nginx", repo.ID)

	res := f.do(http.MethodDelete, "/api/places/"+nas.ID, nil)

	holders, _ := res["holders"].(map[string]any)
	if res["code"] != "place-in-use" || !reflect.DeepEqual(holders["defaults"], []any{"containers"}) ||
		!reflect.DeepEqual(holders["items"], []any{map[string]any{"domain": "containers", "key": "nginx"}}) {
		t.Fatalf("DELETE = %v, want place-in-use held by the containers default and nginx", res)
	}
}

func TestAnUnusedPlaceGoesWithItsTargetsAndItsCredentialSet(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "b2-set", Name: "B2", CloudCreds: CloudCreds{S3KeyID: "k1", S3Secret: "s1"}}}); err != nil {
		t.Fatal(err)
	}
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.CredsRef = "b2-set"
	b2 := f.storePlace(p)
	target := f.placeTarget(b2, "vms", "")

	res := f.do(http.MethodDelete, "/api/places/"+b2.ID, nil)

	if res["ok"] != true || res["removedTargets"] != float64(1) {
		t.Fatalf("DELETE = %v, want one target removed", res)
	}
	if _, err := f.st.GetPlace(b2.ID); !errors.Is(err, store.ErrPlaceNotFound) {
		t.Fatalf("GetPlace = %v, want ErrPlaceNotFound", err)
	}
	if _, found, err := f.st.GetOffsiteTarget(target.ID); err != nil || found {
		t.Fatalf("the target is still there: %v", err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if sets, err := f.svc.decodeCloudCredSets(settings); err != nil || len(sets) != 0 {
		t.Fatalf("sets = %+v, %v, want the place's set gone", sets, err)
	}
}

func TestACredentialSetAPullSourceUsesStaysWhenItsPlaceGoes(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "b2-set", Name: "B2", CloudCreds: CloudCreds{S3KeyID: "k1", S3Secret: "s1"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.CreatePullSource(store.PullSource{Name: "Old box", Repo: "s3:https://s3.example.com/other", CredsRef: "b2-set", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.CredsRef = "b2-set"
	b2 := f.storePlace(p)

	if res := f.do(http.MethodDelete, "/api/places/"+b2.ID, nil); res["ok"] != true {
		t.Fatalf("DELETE = %v", res)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if sets, err := f.svc.decodeCloudCredSets(settings); err != nil || len(sets) != 1 || sets[0].ID != "b2-set" {
		t.Fatalf("sets = %+v, %v, want the pull source's set kept", sets, err)
	}
}

func TestAnUnknownPlaceCannotBeRemoved(t *testing.T) {
	f := newPlacementFixture(t)
	if code, res := f.doStatus(http.MethodDelete, "/api/places/nosuchplace", nil); code != http.StatusNotFound || res["ok"] != false {
		t.Fatalf("DELETE an unknown place = %d %v, want 404", code, res)
	}
}

func TestAPlaceRemovalWaitsForAnEditInProgress(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	f.svc.placeEditMu.Lock()
	done := make(chan error, 1)
	go func() {
		_, err := f.svc.deletePlace(b2.ID)
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("a place was removed during an edit of it: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	f.svc.placeEditMu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the removal never finished")
	}
	if _, err := f.st.GetPlace(b2.ID); !errors.Is(err, store.ErrPlaceNotFound) {
		t.Fatalf("GetPlace = %v, want the place removed once the edit let go", err)
	}
}

func TestTestingAPlaceProbesEveryAddressItStandsFor(t *testing.T) {
	f := newPlacementFixture(t)
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.Folders = map[string]string{"containers": "container", "vms": "vms", "flash": "flash"}
	b2 := f.storePlace(p)
	f.eng.ids["s3:https://s3.example.com/bucket/container"] = "r-ct"
	f.eng.opens["s3:https://s3.example.com/bucket/vms"] = false
	flash := "s3:https://s3.example.com/bucket/flash"
	f.eng.opens[flash] = false
	f.eng.openErr[flash] = errors.New("dial tcp 203.0.113.9:443: connect: connection refused")

	res := f.do(http.MethodPost, "/api/places/"+b2.ID+"/test", nil)

	folders := res["folders"].(map[string]any)
	if res["ok"] != false || res["code"] != "place-probe-failed" || folders["containers"] != "repository" ||
		folders["vms"] != "empty" || folders["flash"] != "error" || res["repoIds"].(map[string]any)["containers"] != "r-ct" {
		t.Fatalf("POST test = %v", res)
	}
	last := placeByName(t, f.do(http.MethodGet, "/api/places", nil), "B2")["lastTest"].(map[string]any)
	if last["source"] != "test" || last["ok"] != false || last["error"] != res["error"] {
		t.Fatalf("lastTest = %v, want the failed test", last)
	}
}

func TestTestingAPlaceOpensARowUnderAnotherEnding(t *testing.T) {
	f := newPlacementFixture(t)
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.Folders = map[string]string{"containers": "container"}
	b2 := f.storePlace(p)
	f.placeTarget(b2, "containers", "-copies")
	f.eng.ids["s3:https://s3.example.com/bucket/container"] = "r-ct"
	copies := "s3:https://s3.example.com/bucket/container-copies"
	f.eng.opens[copies] = false
	f.eng.openErr[copies] = errors.New("dial tcp 203.0.113.9:443: connect: connection refused")

	res := f.do(http.MethodPost, "/api/places/"+b2.ID+"/test", nil)

	if res["ok"] != false || res["code"] != "place-probe-failed" || res["folders"].(map[string]any)["containers"] != "repository" ||
		!strings.Contains(res["error"].(string), "connection refused") {
		t.Fatalf("POST test = %v, want the -copies target's failure", res)
	}
	if f.eng.opened[copies] == 0 {
		t.Fatalf("the -copies target was never opened: %v", f.eng.opened)
	}
}

func TestTestingAPlaceOpensADirectRepositoryWithItsOwnCredentials(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{
		{ID: "b2-set", Name: "B2", CloudCreds: CloudCreds{S3KeyID: "k2", S3Secret: "s2"}},
		{ID: "old-set", Name: "B2 before", CloudCreds: CloudCreds{S3KeyID: "k1", S3Secret: "s1"}},
	}); err != nil {
		t.Fatal(err)
	}
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.Folders, p.CredsRef = map[string]string{"containers": "container"}, "b2-set"
	b2 := f.storePlace(p)
	target := f.placeTarget(b2, "containers", "")
	direct := f.direct(target)
	f.linkRow(direct.ID, b2, "containers", "-direct")
	if _, err := f.db.Exec(`UPDATE offsite_targets SET creds_ref = 'old-set' WHERE id = ?`, direct.ID); err != nil {
		t.Fatal(err)
	}
	f.eng.ids[target.Repo], f.eng.ids[direct.Repo] = "r-ct", "r-direct"
	f.eng.keys[direct.Repo] = "AWS_ACCESS_KEY_ID=k1"
	f.eng.openErr[direct.Repo] = errors.New("Access Denied")

	res := f.do(http.MethodPost, "/api/places/"+b2.ID+"/test", nil)

	if res["ok"] != true || f.eng.opened[direct.Repo] == 0 {
		t.Fatalf("POST test = %v, opened %v, want the direct repository opened on its own key", res, f.eng.opened)
	}
}

func TestAPassedTestOutranksAnOlderFailedRun(t *testing.T) {
	f := newPlacementFixture(t)
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.Folders = map[string]string{"vms": "vms"}
	b2 := f.storePlace(p)
	target := f.placeTarget(b2, "vms", "")
	f.eng.ids[target.Repo] = "r-vms"
	id, err := f.st.RecordOffsiteRunForTarget("vms", target.ID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.FinishOffsiteRun(id, false, ""); err != nil {
		t.Fatal(err)
	}

	if res := f.do(http.MethodPost, "/api/places/"+b2.ID+"/test", nil); res["ok"] != true || res["code"] != nil {
		t.Fatalf("POST test = %v, want a passed test", res)
	}
	last := placeByName(t, f.do(http.MethodGet, "/api/places", nil), "B2")["lastTest"].(map[string]any)
	if last["source"] != "test" || last["ok"] != true {
		t.Fatalf("lastTest = %v, want the passed test over the run at 100", last)
	}
}

func TestTestingAShareWithNothingMountedFails(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.storePlace(localPlace("NAS", "remotes/nas/bombvault"))
	if err := os.MkdirAll(filepath.FromSlash(f.root+"/remotes/nas/bombvault"), 0o750); err != nil {
		t.Fatal(err)
	}
	writeMountFixture(t, "/")

	res := f.do(http.MethodPost, "/api/places/"+nas.ID+"/test", nil)

	if res["ok"] != false || res["code"] != "place-probe-failed" || res["error"] != "nothing is mounted at this share; mount it on the server first" {
		t.Fatalf("POST test = %v, want place-probe-failed naming the missing mount", res)
	}
}

func TestAnUnknownPlaceCannotBeTested(t *testing.T) {
	f := newPlacementFixture(t)
	if code, res := f.doStatus(http.MethodPost, "/api/places/nosuchplace/test", nil); code != http.StatusNotFound || res["ok"] != false {
		t.Fatalf("POST test of an unknown place = %d %v, want 404", code, res)
	}
}

func TestAPlaceNamesTheCredentialSetItKeepsItsCredentialsIn(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "set-b2", Name: "B2", Kind: "s3", CloudCreds: CloudCreds{S3KeyID: "k", S3Secret: "s"}}}); err != nil {
		t.Fatal(err)
	}
	own := s3Place("B2", "s3:https://s3.example.com/bucket")
	own.CredsRef = "set-b2"
	f.storePlace(own)
	f.storePlace(s3Place("Shared", "s3:https://s3.example.com/other"))

	res := f.do(http.MethodGet, "/api/places", nil)

	if got := placeByName(t, res, "B2")["credsRef"]; got != "set-b2" {
		t.Errorf("B2 credsRef = %v, want set-b2", got)
	}
	if got := placeByName(t, res, "Shared")["credsRef"]; got != "" {
		t.Errorf("Shared credsRef = %v, want the empty name of the shared credentials", got)
	}
}
