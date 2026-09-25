package api

import (
	"encoding/json"
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
