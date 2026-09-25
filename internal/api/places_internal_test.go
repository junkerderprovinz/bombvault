package api

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
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
	if !reflect.DeepEqual(usage["homeDomains"], []any{"containers", "vms", "files"}) {
		t.Errorf("Unraid homeDomains = %v", usage["homeDomains"])
	}
	if locked := unraid["locked"].(map[string]any); locked["containers"] != true || locked["vms"] != false {
		t.Errorf("Unraid locked = %v, want containers only", locked)
	}
	cloud := placeByName(t, res, "B2")
	usage = cloud["usage"].(map[string]any)
	if !reflect.DeepEqual(usage["copyDomains"], []any{"containers"}) || usage["copies"] != float64(4) {
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

func TestAFailedCopyRunToAPlaceOutranksAnEarlierSuccess(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	target := f.placeTarget(b2, "vms", "")
	lastTest := func() any { return placeByName(t, f.do(http.MethodGet, "/api/places", nil), "B2")["lastTest"] }
	run := func(at int64, ok bool) {
		t.Helper()
		id, err := f.st.RecordOffsiteRunForTarget("vms", target.ID, at)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.st.FinishOffsiteRun(id, ok, ""); err != nil {
			t.Fatal(err)
		}
	}

	if got := lastTest(); got != nil {
		t.Fatalf("lastTest before any run = %v", got)
	}
	run(100, true)
	if got := lastTest(); !reflect.DeepEqual(got, map[string]any{"at": float64(100), "ok": true, "source": "run"}) {
		t.Fatalf("lastTest after a success = %v", got)
	}
	run(200, false)
	if got := lastTest(); !reflect.DeepEqual(got, map[string]any{"at": float64(200), "ok": false, "source": "run"}) {
		t.Fatalf("lastTest after a failure = %v", got)
	}
}

func TestRowsWithoutAPlaceWaitInTheirOwnGroup(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	loose := f.target("vms", "Old NAS", "remotes/oldnas/vms")
	nas := f.namedRepo("NAS", "nas")

	rows := rowsOf(f.do(http.MethodGet, "/api/places", nil)["unplaced"])

	want := []map[string]any{
		{"rowId": "", "domain": "flash", "role": "path", "name": "", "repo": "user/bombvault/flash"},
		{"rowId": "", "domain": "config", "role": "path", "name": "", "repo": "user/bombvault/config"},
		{"rowId": loose.ID, "domain": "vms", "role": "target", "name": "Old NAS", "repo": "remotes/oldnas/vms"},
		{"rowId": nas.ID, "domain": "", "role": "repository", "name": "NAS", "repo": "nas"},
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
	if res := f.do(http.MethodPatch, "/api/places/"+unraid.ID, map[string]any{"immutable": true}); res["ok"] != false || res["code"] != nil {
		t.Fatalf("append-only on a local place = %v, want a plain refusal", res)
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
