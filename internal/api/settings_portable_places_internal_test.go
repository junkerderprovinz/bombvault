package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

const b2Base = "s3:https://s3.eu-central-003.backblazeb2.com/bv-offsite"

// seededPlaces is what seedPlaces leaves behind.
type seededPlaces struct {
	unraid, b2          store.Place
	field, repo, copies store.OffsiteTarget
}

// seedPlaces gives f a local place that is home to containers, VMs and
// folder sets, and a B2 place holding the containers field target, a named
// repository in its VMs folder and a VM copy target beside that one.
func seedPlaces(f *placementFixture) seededPlaces {
	f.t.Helper()
	var s seededPlaces
	s.unraid = f.storePlace(store.Place{
		ID: "place-unraid", Name: "Unraid", Provider: "unraid-folder", Kind: "local", Base: "backups",
		Folders: map[string]string{"containers": "containers", "vms": "vms", "files": "files"}, Enabled: true,
	}, "containers", "vms", "files")
	s.b2 = f.storePlace(store.Place{
		ID: "place-b2", Name: "B2", Provider: "b2", Kind: "s3", Base: b2Base,
		Folders:     map[string]string{"containers": "container", "vms": "vms"},
		OffPremises: true, RetentionKeepDaily: 30, Enabled: true,
	})
	s.field = f.fieldTarget("containers", b2Base+"/container")
	f.linkRow(s.field.ID, s.b2, "containers", "")
	s.repo = f.namedRepo("B2 VMs", b2Base+"/vms")
	f.linkRow(s.repo.ID, s.b2, "vms", "")
	s.copies = f.target("vms", "B2 copies", b2Base+"/vms-copies")
	f.linkRow(s.copies.ID, s.b2, "vms", "-copies")
	return s
}

// exportedRow finds a row of an exported block by id.
func exportedRow(t *testing.T, exp map[string]any, block, id string) map[string]any {
	t.Helper()
	rows, _ := exp[block].([]any)
	for _, row := range rows {
		if m := row.(map[string]any); m["id"] == id {
			return m
		}
	}
	t.Fatalf("%s has no row %s: %v", block, id, exp[block])
	return nil
}

func TestTheExportCarriesPlacesAndWhereEachRowSits(t *testing.T) {
	f := newPlacementFixture(t)
	seed := seedPlaces(f)
	exp := f.do(http.MethodGet, "/api/settings/export", nil)

	if exp["schemaVersion"] != float64(1) {
		t.Errorf("schemaVersion = %v, want 1: the new blocks are additions an older build skips", exp["schemaVersion"])
	}
	list, _ := exp["places"].([]any)
	if len(list) != 2 {
		t.Fatalf("places = %v, want the two seeded", exp["places"])
	}
	var b2 map[string]any
	for _, p := range list {
		if m := p.(map[string]any); m["id"] == seed.b2.ID {
			b2 = m
		}
	}
	wantKeys := []string{
		"base", "createdAt", "credsRef", "enabled", "folders", "growthBudgetGb", "id", "immutable", "kind",
		"limitDownload", "limitUpload", "name", "offPremises", "provider", "retentionKeepDaily", "retentionKeepLast",
		"retentionKeepMonthly", "retentionKeepWeekly", "sortOrder", "storageClass", "updatedAt",
	}
	if got := slices.Sorted(maps.Keys(b2)); !slices.Equal(got, wantKeys) {
		t.Errorf("place keys = %v, want %v", got, wantKeys)
	}
	if b2["base"] != b2Base || b2["retentionKeepDaily"] != float64(30) || b2["offPremises"] != true {
		t.Errorf("B2 place = %v", b2)
	}
	if folders, _ := b2["folders"].(map[string]any); len(folders) != 2 || folders["containers"] != "container" || folders["vms"] != "vms" {
		t.Errorf("B2 folders = %v", b2["folders"])
	}

	homes, _ := exp["storageDomainPlaces"].(map[string]any)
	want := map[string]any{"containers": seed.unraid.ID, "vms": seed.unraid.ID, "files": seed.unraid.ID}
	if !maps.Equal(homes, want) {
		t.Errorf("storageDomainPlaces = %v, want %v", homes, want)
	}

	for _, tc := range []struct{ block, id, domain, suffix string }{
		{"offsiteTargets", seed.field.ID, "containers", ""},
		{"offsiteTargets", seed.copies.ID, "vms", "-copies"},
		{"namedRepos", seed.repo.ID, "vms", ""},
	} {
		row := exportedRow(t, exp, tc.block, tc.id)
		suffix, _ := row["placeSuffix"].(string)
		if row["placeId"] != seed.b2.ID || row["placeDomain"] != tc.domain || suffix != tc.suffix {
			t.Errorf("%s %s = %v, want it on %s, %s%s", tc.block, tc.id, row, seed.b2.ID, tc.domain, tc.suffix)
		}
	}
}

func TestAnInstanceWithoutPlacesStillExportsBothBlocks(t *testing.T) {
	f := newPlacementFixture(t)
	exp := f.do(http.MethodGet, "/api/settings/export", nil)
	if list, ok := exp["places"].([]any); !ok || len(list) != 0 {
		t.Errorf("places = %v, want an empty list", exp["places"])
	}
	if homes, ok := exp["storageDomainPlaces"].(map[string]any); !ok || len(homes) != 0 {
		t.Errorf("storageDomainPlaces = %v, want an empty object", exp["storageDomainPlaces"])
	}
}

// restPlaceBase holds its credential in the URL, a form restic accepts and
// the migration keeps in a place's base.
const restPlaceBase = "rest:https://backupuser:Tr0ub4dor&3@storage.example.com:8000" //nolint:gosec // G101: fake credential, the same one as locWithCreds

// seedCredentialedPlace makes a rest-server place with that base the home of
// the VMs.
func seedCredentialedPlace(t *testing.T, st *store.Repo) {
	t.Helper()
	if _, err := st.WritePlace(store.PlaceWrite{
		Place: store.Place{
			ID: "place-rest", Name: "Rest server", Provider: "rest-server", Kind: "rest", Base: restPlaceBase,
			Folders: map[string]string{"vms": "vms"}, OffPremises: true, Enabled: true,
		},
		HomeDomains: map[string]string{"vms": "place-rest"},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPlainExportRedactsPlaceBasesAndDomainPaths(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	seedCredentialedPlace(t, srcStore)

	body, exp := doExport(t, src, "")
	for _, secret := range []string{locRepoPass, locRepoUser} {
		if bytes.Contains(body, jsonText(t, secret)) {
			t.Fatalf("the plain export leaked %q:\n%s", secret, body)
		}
	}
	redacted := "rest:https://" + redactedLocationMarker + "storage.example.com:8000"
	if len(exp.Places) != 1 || exp.Places[0].Base != redacted {
		t.Fatalf("places = %+v, want the base with its credential replaced by the marker", exp.Places)
	}
	if exp.Settings.VMsPath != redacted+"/vms" {
		t.Fatalf("vmsPath = %q, want it redacted like its place's base", exp.Settings.VMsPath)
	}

	_, full := doExport(t, src, "?includeCredentials=true")
	if full.Places[0].Base != restPlaceBase || full.Settings.VMsPath != restPlaceBase+"/vms" {
		t.Fatalf("the credentialed export must carry base and path whole: %q, %q", full.Places[0].Base, full.Settings.VMsPath)
	}
}

func TestImportKeepsTheCredentialInAWorkingDomainPath(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	seedCredentialedPlace(t, srcStore)
	body, _ := doExport(t, src, "")

	dst, dstStore := newPortableHandler(t, appKeyB)
	const working = "rest:https://backupuser:dst-pass@storage.example.com:8000/vms" //nolint:gosec // G101: fake credential
	s, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.VMsPath = working
	if err := dstStore.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}
	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.VMsPath != working {
		t.Fatalf("vmsPath = %q, want this instance's working path kept", got.VMsPath)
	}
}

func TestImportOnFreshInstanceTakesTheRedactedDomainPath(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	seedCredentialedPlace(t, srcStore)
	body, _ := doExport(t, src, "")

	dst, dstStore := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}
	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.VMsPath, redactedLocationMarker) || strings.Contains(got.VMsPath, locRepoPass) {
		t.Fatalf("vmsPath = %q, want the redacted remote path, neither the local default nor the password", got.VMsPath)
	}
}

func TestASelfImportOfAPlainExportKeepsTheCredentialedPlaceWhole(t *testing.T) {
	h, st := newPortableHandler(t, appKeyA)
	seedSource(t, h, st)
	seedCredentialedPlace(t, st)
	body, _ := doExport(t, h, "")

	if env := doImport(t, h, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}
	placesNow, err := st.ListPlaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(placesNow) != 1 {
		t.Fatalf("places = %+v, want the rest place alone", placesNow)
	}
	if placesNow[0].Base != restPlaceBase {
		t.Errorf("base = %q, want the one it had here", placesNow[0].Base)
	}
	homes, err := st.DomainPlaces()
	if err != nil {
		t.Fatal(err)
	}
	if homes["vms"] != "place-rest" {
		t.Errorf("vms is stored at %q, want place-rest", homes["vms"])
	}
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.VMsPath != restPlaceBase+"/vms" {
		t.Errorf("vmsPath = %q, want the working path with its credential", s.VMsPath)
	}
}

func TestImportOnFreshInstanceTakesAStorageBoxPathWhole(t *testing.T) {
	const box = "sftp:u123456@u123456.your-storagebox.de:/bv/vms"
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	s, err := srcStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.VMsPath = box
	if err := srcStore.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	body, _ := doExport(t, src, "")

	dst, dstStore := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}
	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.VMsPath != box {
		t.Fatalf("vmsPath = %q, want %q: it holds no password to strip", got.VMsPath, box)
	}
}

// seedCredentialSetPlace gives h a credential set and a B2 place that uses it.
func seedCredentialSetPlace(t *testing.T, h *Handler, st *store.Repo) {
	t.Helper()
	if err := h.svc.SetCloudCredSets([]CloudCredSet{
		{ID: "set-b2", Name: "B2 key", CloudCreds: CloudCreds{S3KeyID: "K005", S3Secret: "b2secret"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.WritePlace(store.PlaceWrite{Place: store.Place{
		ID: "place-b2", Name: "B2", Provider: "b2", Kind: "s3", Base: b2Base,
		Folders: map[string]string{"containers": "container"}, CredsRef: "set-b2", OffPremises: true, Enabled: true,
	}}); err != nil {
		t.Fatal(err)
	}
}

// storedCredSet reads one credential set of h with its secrets.
func storedCredSet(t *testing.T, h *Handler, id string) (CloudCredSet, bool) {
	t.Helper()
	s, err := h.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	sets, err := h.svc.decodeCloudCredSets(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, set := range sets {
		if set.ID == id {
			return set, true
		}
	}
	return CloudCredSet{}, false
}

func TestTheCredentialedExportCarriesTheCredentialSets(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	seedCredentialSetPlace(t, src, srcStore)

	_, full := doExport(t, src, "?includeCredentials=true")
	if full.Credentials == nil || len(full.Credentials.CredSets) != 1 {
		t.Fatalf("credentials = %+v, want the one credential set", full.Credentials)
	}
	if set := full.Credentials.CredSets[0]; set.ID != "set-b2" || set.S3KeyID != "K005" || set.S3Secret != "b2secret" {
		t.Fatalf("credential set = %+v, want set-b2 with its secret", set)
	}
	if len(full.Places) != 1 || full.Places[0].CredsRef != "set-b2" {
		t.Fatalf("places = %+v, want the B2 place naming set-b2", full.Places)
	}

	body, _ := doExport(t, src, "")
	if bytes.Contains(body, []byte("b2secret")) {
		t.Fatal("a credential set's secret leaked into the plain export")
	}
}

func TestImportAddsTheFilesCredentialSetsAndKeepsTheOthers(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	seedCredentialSetPlace(t, src, srcStore)
	body, _ := doExport(t, src, "?includeCredentials=true")

	dst, _ := newPortableHandler(t, appKeyB)
	if err := dst.svc.SetCloudCredSets([]CloudCredSet{
		{ID: "set-pull", Name: "Pull source", CloudCreds: CloudCreds{S3KeyID: "PULL", S3Secret: "pullsecret"}},
		{ID: "set-b2", Name: "Stale", CloudCreds: CloudCreds{S3KeyID: "OLD", S3Secret: "oldsecret"}},
	}); err != nil {
		t.Fatal(err)
	}

	preview := doImport(t, dst, body, "")
	summary, _ := preview["summary"].(map[string]any)
	if creds, _ := summary["credentials"].(map[string]any); creds["credSets"] != float64(1) {
		t.Fatalf("preview = %v, want the one credential set counted", preview)
	}
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}

	if b2, ok := storedCredSet(t, dst, "set-b2"); !ok || b2.Name != "B2 key" || b2.S3KeyID != "K005" || b2.S3Secret != "b2secret" {
		t.Errorf("set-b2 = %+v (found %v), want the file's set, readable with this instance's key", b2, ok)
	}
	if pull, ok := storedCredSet(t, dst, "set-pull"); !ok || pull.S3Secret != "pullsecret" {
		t.Errorf("set-pull = %+v (found %v), want the set the file does not carry kept", pull, ok)
	}
}

func TestImportKeepsAStoredSecretTheFileLeavesBlank(t *testing.T) {
	dst, _ := newPortableHandler(t, appKeyB)
	if err := dst.svc.SetCloudCredSets([]CloudCredSet{
		{ID: "set-b2", Name: "B2 key", CloudCreds: CloudCreds{S3KeyID: "K005", S3Secret: "keepme"}},
	}); err != nil {
		t.Fatal(err)
	}
	_, exp := doExport(t, dst, "")
	exp.Credentials = &exportCredentials{CredSets: []CloudCredSet{
		{ID: "set-b2", Name: "B2 key", CloudCreds: CloudCreds{S3KeyID: "K006"}},
	}}
	body, err := json.Marshal(exp)
	if err != nil {
		t.Fatal(err)
	}

	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}
	if set, ok := storedCredSet(t, dst, "set-b2"); !ok || set.S3KeyID != "K006" || set.S3Secret != "keepme" {
		t.Fatalf("set-b2 = %+v (found %v), want the file's key id and the stored secret", set, ok)
	}
}

func TestImportRefusesCredentialSetsTheSaveWouldRefuse(t *testing.T) {
	for _, tc := range []struct {
		name string
		sets []CloudCredSet
		want string
	}{
		{"a set without a name", []CloudCredSet{{ID: "set-x"}}, "needs a name"},
		{"a set without an id", []CloudCredSet{{Name: "B2 key"}}, "needs an id"},
		{"two sets with one id", []CloudCredSet{{ID: "set-x", Name: "One"}, {ID: "set-x", Name: "Two"}}, "its id is used twice"},
		{"an archival storage class", []CloudCredSet{{ID: "set-x", Name: "B2 key", CloudCreds: CloudCreds{S3StorageClass: "deep_archive"}}}, "unsupported S3 storage class DEEP_ARCHIVE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dst, _ := newPortableHandler(t, appKeyB)
			_, exp := doExport(t, dst, "")
			exp.Credentials = &exportCredentials{CredSets: tc.sets}
			body, err := json.Marshal(exp)
			if err != nil {
				t.Fatal(err)
			}
			env := doImport(t, dst, body, "?apply=true")
			if msg, _ := env["error"].(string); env["ok"] != false || !strings.Contains(msg, tc.want) {
				t.Fatalf("import = %v, want a refusal containing %q", env, tc.want)
			}
			if _, found := storedCredSet(t, dst, "set-x"); found {
				t.Fatal("a refused file stored its credential set")
			}
		})
	}
}

// validPlacesFile is a small file whose places pass every check.
func validPlacesFile() settingsExport {
	return settingsExport{
		SchemaVersion: settingsExportSchema,
		Settings:      settingsView{ContainersPath: "backups/containers", ContainersOffsite: b2Base + "/container"},
		Places: []placeExport{
			{
				ID: "place-unraid", Name: "Unraid", Provider: "unraid-folder", Kind: "local", Base: "backups",
				Folders: map[string]string{"containers": "containers"}, Enabled: true,
			},
			{
				ID: "place-b2", Name: "B2", Provider: "b2", Kind: "s3", Base: b2Base,
				Folders: map[string]string{"containers": "container"}, CredsRef: "set-b2", Enabled: true,
			},
		},
		StorageDomainPlaces: map[string]string{"containers": "place-unraid"},
		OffsiteTargets: []offsiteTargetView{{
			ID: "tgt-b2", Domain: "containers", Name: "B2", Repo: b2Base + "/container", Enabled: true,
			PlaceID: "place-b2", PlaceDomain: "containers",
		}},
	}
}

func TestAConsistentPlacesFilePassesTheChecks(t *testing.T) {
	if msg := validateExport(validPlacesFile(), "/host/user"); msg != "" {
		t.Fatalf("validateExport = %q, want no refusal", msg)
	}
}

// An instance has to take back its own file. A row of any kind could name a
// credential set before storage places, and the migration hands the set on to
// the row's place, so a local or sftp place can carry one.
func TestTheExportOfAMigratedSetupPassesThePlaceChecks(t *testing.T) {
	f := newPlacementFixture(t)
	s := seedV813(t, f)
	for _, row := range []store.OffsiteTarget{s.pi, s.nas} {
		row.CredsRef = "garage"
		if _, err := f.st.UpsertOffsiteTarget(row); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.svc.MigrateToPlaces(); err != nil {
		t.Fatalf("MigrateToPlaces: %v", err)
	}

	_, exp := doExport(t, f.h, "")
	if len(exp.Places) == 0 {
		t.Fatal("the migration left no places to export")
	}
	if msg := validateExport(exp, f.h.cfg.HostMountRoot); msg != "" {
		t.Fatalf("validateExport = %q, want no refusal", msg)
	}
}

func TestImportRefusesPlacesTheFormsWouldRefuse(t *testing.T) {
	for _, tc := range []struct {
		name string
		want string
		edit func(*settingsExport)
	}{
		{"a place without an id", "storage place #1: needs an id", func(e *settingsExport) { e.Places[0].ID = " " }},
		{"a place without a name", "storage place #1: needs a name", func(e *settingsExport) { e.Places[0].Name = "" }},
		{"two places with one id", "its id is used twice", func(e *settingsExport) { e.Places[1].ID = "place-unraid" }},
		{"two places with one name", `the name "Unraid" is used twice`, func(e *settingsExport) { e.Places[1].Name = "Unraid" }},
		{"an unknown provider", `unknown provider "dropbox"`, func(e *settingsExport) { e.Places[1].Provider = "dropbox" }},
		{"a kind the provider does not have", `does not match provider "b2"`, func(e *settingsExport) { e.Places[1].Kind = "rest" }},
		{"a local base outside the mount root", "relative subpaths under the mount root", func(e *settingsExport) { e.Places[0].Base = "/mnt/user/backups" }},
		{"a local base that looks like a remote", "its base looks like a remote without a restic prefix", func(e *settingsExport) { e.Places[0].Base = "BackBlaze:bucket" }},
		{"a folder that climbs out", "the folder for containers leaves the base", func(e *settingsExport) { e.Places[0].Folders["containers"] = "../../etc" }},
		{"a folder for an unknown domain", `has a folder for an unknown domain "photos"`, func(e *settingsExport) { e.Places[0].Folders["photos"] = "photos" }},
		{"a remote kind on a local path", "its base is not a remote location", func(e *settingsExport) { e.Places[1].Base = "backups/b2" }},
		{"an archival storage class", "unsupported storage class GLACIER", func(e *settingsExport) { e.Places[1].StorageClass = "glacier" }},
		{"a home of an unknown domain", `storageDomainPlaces names an unknown domain "photos"`, func(e *settingsExport) { e.StorageDomainPlaces["photos"] = "place-unraid" }},
		{"a home the file does not carry", "the containers domain is stored at a place the file does not carry", func(e *settingsExport) { e.StorageDomainPlaces["containers"] = "place-gone" }},
		{"a home without a folder", "the vms domain is stored at Unraid, which has no folder for it", func(e *settingsExport) { e.StorageDomainPlaces["vms"] = "place-unraid" }},
		{"a home that is switched off", "which is switched off", func(e *settingsExport) { e.Places[0].Enabled = false }},
		{"a row on a place the file does not carry", "off-site target #1: it sits on a storage place the file does not carry", func(e *settingsExport) { e.OffsiteTargets[0].PlaceID = "place-gone" }},
		{"a row on a folder the place lacks", "has no folder for vms", func(e *settingsExport) { e.OffsiteTargets[0].PlaceDomain = "vms" }},
		{"a target in the folder of another domain", "not for its own domain containers", func(e *settingsExport) {
			e.Places[1].Folders["vms"] = "vms"
			e.OffsiteTargets[0].PlaceDomain = "vms"
		}},
		{"a suffix holding a path", "its place suffix must not hold a path", func(e *settingsExport) { e.OffsiteTargets[0].PlaceSuffix = "-copies/x" }},
		{"a placed row without an id", "a row on a storage place needs its id", func(e *settingsExport) { e.OffsiteTargets[0].ID = "" }},
		{"a placed row in a file without places", "it sits on a storage place the file does not carry", func(e *settingsExport) {
			e.Places = nil
			e.StorageDomainPlaces = nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exp := validPlacesFile()
			tc.edit(&exp)
			if msg := validateExport(exp, "/host/user"); !strings.Contains(msg, tc.want) {
				t.Fatalf("validateExport = %q, want a refusal containing %q", msg, tc.want)
			}
		})
	}
}

func TestPlacesSurviveARoundTrip(t *testing.T) {
	src := newPlacementFixture(t)
	seedPlaces(src)
	exp := src.do(http.MethodGet, "/api/settings/export", nil)

	dst := newPlacementFixture(t)
	preview := dst.do(http.MethodPost, "/api/settings/import", exp)
	if summary, _ := preview["summary"].(map[string]any); summary["places"] != float64(2) {
		t.Fatalf("preview = %v, want two places counted", preview)
	}
	if res := dst.do(http.MethodPost, "/api/settings/import?apply=true", exp); res["ok"] != true {
		t.Fatalf("import = %v", res)
	}

	back := dst.do(http.MethodGet, "/api/settings/export", nil)
	for _, block := range []string{"places", "storageDomainPlaces", "offsiteTargets", "namedRepos"} {
		if !reflect.DeepEqual(back[block], exp[block]) {
			t.Errorf("%s:\n got %v\nwant %v", block, back[block], exp[block])
		}
	}
	got, want := back["settings"].(map[string]any), exp["settings"].(map[string]any)
	for _, key := range []string{"containersPath", "vmsPath", "filesPath", "containersOffsite", "containersOffsiteImmutable"} {
		if got[key] != want[key] {
			t.Errorf("settings.%s = %v, want %v", key, got[key], want[key])
		}
	}
}

func TestAnImportLeavesPlacedRowsToTheirPlace(t *testing.T) {
	src := newPlacementFixture(t)
	s, err := src.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.OffsiteRetentionKeepDaily = 7
	if err := src.st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	seed := seedPlaces(src)
	exp := src.do(http.MethodGet, "/api/settings/export", nil)

	dst := newPlacementFixture(t)
	if res := dst.do(http.MethodPost, "/api/settings/import?apply=true", exp); res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	field, ok, err := dst.st.GetOffsiteTarget(seed.field.ID)
	if err != nil || !ok || field.PlaceID != seed.b2.ID || field.RetentionKeepDaily != 30 {
		t.Fatalf("field target = %+v (ok %v, err %v), want it on B2 with the place's 30 daily, not the settings' 7", field, ok, err)
	}
}

func TestAnEmptyPlacesBlockLeavesNoPlaces(t *testing.T) {
	f := newPlacementFixture(t)
	seedPlaces(f)
	res := importEdited(t, f, func(exp map[string]any) {
		exp["places"] = []any{}
		exp["storageDomainPlaces"] = map[string]any{}
		for _, block := range []string{"offsiteTargets", "namedRepos"} {
			rows, _ := exp[block].([]any)
			for _, row := range rows {
				m := row.(map[string]any)
				delete(m, "placeId")
				delete(m, "placeDomain")
				delete(m, "placeSuffix")
			}
		}
	})
	if res["ok"] != true {
		t.Fatalf("import = %v", res)
	}

	if got, err := f.st.ListPlaces(); err != nil || len(got) != 0 {
		t.Errorf("places = %+v (err %v), want none", got, err)
	}
	if homes, err := f.st.DomainPlaces(); err != nil || len(homes) != 0 {
		t.Errorf("home places = %v (err %v), want none", homes, err)
	}
	targets, err := f.st.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	repos, err := f.st.ListNamedRepos()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range slices.Concat(targets, repos) {
		if row.PlaceID != "" {
			t.Errorf("row %s is still on place %s", row.ID, row.PlaceID)
		}
	}
	if s, err := f.st.GetSettings(); err != nil || s.PlacesMigrated == 0 {
		t.Errorf("places_migrated = %d (err %v), want it set so the places migration does not build places the file said are not there", s.PlacesMigrated, err)
	}
}

func TestAnImportDoesNotMoveARepositoryInUseOntoItsPlace(t *testing.T) {
	src := newPlacementFixture(t)
	seed := seedPlaces(src)
	exp := src.do(http.MethodGet, "/api/settings/export", nil)

	dst := newPlacementFixture(t)
	const kept = "s3:https://s3.example.com/other/vms"
	repo, err := dst.st.UpsertOffsiteTarget(store.OffsiteTarget{
		ID: seed.repo.ID, Role: store.RoleRepo, Name: "B2 VMs", Repo: kept, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	dst.vm("win11", repo.ID)
	buf := watchLog(t)

	if res := dst.do(http.MethodPost, "/api/settings/import?apply=true", exp); res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	back, err := dst.st.GetNamedRepo(repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Repo != kept || back.PlaceID != "" {
		t.Fatalf("repository = %+v, want it at %s and on no place", back, kept)
	}
	if !strings.Contains(buf.String(), `repository "B2 VMs" keeps the location it has here`) {
		t.Errorf("no log line for the repository left off its place, got:\n%s", buf.String())
	}
}

func TestARedactedPlaceDoesNotTakeOverAWorkingDomainPath(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	seedCredentialedPlace(t, srcStore)
	body, _ := doExport(t, src, "")

	dst, dstStore := newPortableHandler(t, appKeyB)
	const working = "rest:https://backupuser:dst-pass@storage.example.com:8000/vms" //nolint:gosec // G101: fake credential
	s, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.VMsPath = working
	if err := dstStore.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	buf := watchLog(t)

	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}
	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.VMsPath != working {
		t.Fatalf("vmsPath = %q, want the working path kept", got.VMsPath)
	}
	homes, err := dstStore.DomainPlaces()
	if err != nil {
		t.Fatal(err)
	}
	if id, placed := homes["vms"]; placed {
		t.Fatalf("vms is stored at %s; a place with a redacted base would write that base over the working path", id)
	}
	placesNow, err := dstStore.ListPlaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(placesNow) != 1 || !strings.Contains(placesNow[0].Base, redactedLocationMarker) {
		t.Fatalf("places = %+v, want the file's place with its redacted base", placesNow)
	}
	if !strings.Contains(buf.String(), "the vms path keeps the location it has here") {
		t.Errorf("no log line for the domain left off its place, got:\n%s", buf.String())
	}
}

// Place and row ids differ between two instances, so the file's field row
// arrives under an id this instance has no location for.
func TestARedactedPlaceDoesNotTakeOverAWorkingOffsiteField(t *testing.T) {
	src := newPlacementFixture(t)
	rest := src.storePlace(store.Place{
		ID: "place-rest", Name: "Rest server", Provider: "rest-server", Kind: "rest", Base: restPlaceBase,
		Folders: map[string]string{"containers": "containers"}, OffPremises: true, Enabled: true,
	})
	field := src.fieldTarget("containers", locWithCreds)
	src.linkRow(field.ID, rest, "containers", "")
	exp := src.do(http.MethodGet, "/api/settings/export", nil)

	dst := newPlacementFixture(t)
	const working = "rest:https://backupuser:dst-pass@storage.example.com:8000/containers" //nolint:gosec // G101: fake credential
	dst.fieldTarget("containers", working)
	buf := watchLog(t)

	if res := dst.do(http.MethodPost, "/api/settings/import?apply=true", exp); res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	s, err := dst.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.ContainersOffsite != working {
		t.Fatalf("containersOffsite = %q, want the working location kept", s.ContainersOffsite)
	}
	row, ok, err := dst.st.FieldOffsiteTarget("containers")
	if err != nil || !ok || row.Repo != working || row.PlaceID != "" {
		t.Fatalf("field row = %+v (ok %v, err %v), want it at the working location and on no place", row, ok, err)
	}
	if !strings.Contains(buf.String(), "keeps the location it has here, so it is not put on") {
		t.Errorf("no log line for the field row left off its place, got:\n%s", buf.String())
	}
}

func TestASwitchedOffFieldRowStaysOnItsPlaceThroughAnImport(t *testing.T) {
	src := newPlacementFixture(t)
	b2 := src.storePlace(bucketAt("B2", "s3:https://s3.example.com/bucket"))
	row := src.placedFieldRow("containers", b2)
	row.Enabled = false
	if _, err := src.st.UpsertOffsiteTarget(row); err != nil {
		t.Fatal(err)
	}
	src.storePlace(b2)
	exp := src.do(http.MethodGet, "/api/settings/export", nil)

	dst := newPlacementFixture(t)
	if res := dst.do(http.MethodPost, "/api/settings/import?apply=true", exp); res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	got, ok, err := dst.st.GetOffsiteTarget(row.ID)
	if err != nil || !ok || got.Enabled || got.PlaceID != b2.ID {
		t.Fatalf("field row = %+v (ok %v, err %v), want it off and on %s", got, ok, err, b2.ID)
	}
}

func TestAPlaceMovesTheFieldWithItsRowWhenTheRepoHasSpacesAround(t *testing.T) {
	exp := validPlacesFile()
	exp.Places[1].Folders["containers"] = "bv"
	exp.OffsiteTargets[0].Repo += " "

	got := placeFileLocations(exp)
	want := b2Base + "/bv"
	if got.Settings.ContainersOffsite != want || got.OffsiteTargets[0].Repo != want {
		t.Fatalf("field %q, row %q, want both at %q", got.Settings.ContainersOffsite, got.OffsiteTargets[0].Repo, want)
	}
}

func TestAFilesPlacesDecideTheLocationsItsRowsDisagreeOn(t *testing.T) {
	f := newPlacementFixture(t)
	seed := seedPlaces(f)
	res := importEdited(t, f, func(exp map[string]any) {
		exp["settings"].(map[string]any)["containersPath"] = "elsewhere/containers"
		exportedRow(t, exp, "offsiteTargets", seed.copies.ID)["repo"] = b2Base + "/somewhere-else"
	})
	if res["ok"] != true {
		t.Fatalf("import = %v", res)
	}

	s, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.ContainersPath != "backups/containers" {
		t.Errorf("containersPath = %q, want the home place's backups/containers", s.ContainersPath)
	}
	homes, err := f.st.DomainPlaces()
	if err != nil {
		t.Fatal(err)
	}
	if homes["containers"] != seed.unraid.ID {
		t.Errorf("containers is stored at %q, want %s", homes["containers"], seed.unraid.ID)
	}
	copies, ok, err := f.st.GetOffsiteTarget(seed.copies.ID)
	if err != nil || !ok || copies.Repo != b2Base+"/vms-copies" || copies.PlaceID != seed.b2.ID {
		t.Errorf("copy target = %+v (ok %v, err %v), want it at the place's vms-copies and on B2", copies, ok, err)
	}
}

func TestAnOlderFileRebuildsThePlacesFromItsRows(t *testing.T) {
	src := newPlacementFixture(t)
	src.target("containers", "B2", b2Base+"/container")
	exp := src.do(http.MethodGet, "/api/settings/export", nil)
	delete(exp, "places")
	delete(exp, "storageDomainPlaces")

	dst := newPlacementFixture(t)
	if err := dst.svc.MigrateToPlaces(); err != nil {
		t.Fatal(err)
	}
	dst.storePlace(store.Place{
		ID: "place-old-nas", Name: "Old NAS", Provider: "share", Kind: "local", Base: "remotes/nas",
		Folders: map[string]string{"containers": "bv"}, Enabled: true,
	})

	preview := dst.do(http.MethodPost, "/api/settings/import", exp)
	if summary, _ := preview["summary"].(map[string]any); summary["places"] != nil {
		t.Errorf("preview = %v, want places null for a file without the block", preview)
	}
	if res := dst.do(http.MethodPost, "/api/settings/import?apply=true", exp); res["ok"] != true {
		t.Fatalf("import = %v", res)
	}

	s, err := dst.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.PlacesMigrated == 0 {
		t.Error("places_migrated is clear; the places were not rebuilt from the imported rows")
	}
	placesNow, err := dst.st.ListPlaces()
	if err != nil {
		t.Fatal(err)
	}
	homes, err := dst.st.DomainPlaces()
	if err != nil {
		t.Fatal(err)
	}
	targets, err := dst.st.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	repos, err := dst.st.ListNamedRepos()
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{}
	for _, id := range homes {
		used[id] = true
	}
	for _, row := range slices.Concat(targets, repos) {
		if row.PlaceID != "" {
			used[row.PlaceID] = true
		}
	}
	known := map[string]bool{}
	onBackups := 0
	for _, p := range placesNow {
		known[p.ID] = true
		if p.ID == "place-old-nas" {
			t.Error("a place the imported file does not describe survived the import")
		}
		if !used[p.ID] {
			t.Errorf("place %q is used by nothing", p.Name)
		}
		if p.Base == "backups" {
			onBackups++
		}
	}
	for id := range used {
		if !known[id] {
			t.Errorf("a row or domain names place %s, which does not exist", id)
		}
	}
	if onBackups != 1 {
		t.Errorf("%d places on backups, want one", onBackups)
	}
	if len(targets) != 1 || targets[0].PlaceID == "" {
		t.Errorf("targets = %+v, want the imported B2 target on a place", targets)
	}
}

// Moving an instance onto places is the start's job, so an older file leaves
// an instance that has none as it found it.
func TestAnOlderFileLeavesAnInstanceWithoutPlacesWithoutThem(t *testing.T) {
	src := newPlacementFixture(t)
	src.target("containers", "B2", b2Base+"/container")
	exp := src.do(http.MethodGet, "/api/settings/export", nil)
	delete(exp, "places")
	delete(exp, "storageDomainPlaces")

	dst := newPlacementFixture(t)
	if res := dst.do(http.MethodPost, "/api/settings/import?apply=true", exp); res["ok"] != true {
		t.Fatalf("import = %v", res)
	}

	if got, err := dst.st.ListPlaces(); err != nil || len(got) != 0 {
		t.Errorf("places = %+v (err %v), want none", got, err)
	}
	if s, err := dst.st.GetSettings(); err != nil || s.PlacesMigrated != 0 {
		t.Errorf("places_migrated = %d (err %v), want it clear", s.PlacesMigrated, err)
	}
	targets, err := dst.st.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].PlaceID != "" {
		t.Errorf("targets = %+v, want the imported B2 target on no place", targets)
	}
}

func TestAnImportThatCannotRebuildThePlacesLeavesTheInstanceWithoutThem(t *testing.T) {
	src := newPlacementFixture(t)
	src.target("containers", "B2", b2Base+"/container")
	exp := src.do(http.MethodGet, "/api/settings/export", nil)
	delete(exp, "places")
	delete(exp, "storageDomainPlaces")

	dst := newPlacementFixture(t)
	if err := dst.svc.MigrateToPlaces(); err != nil {
		t.Fatal(err)
	}
	// Stands in for a write that fails while the migration attaches a row.
	if _, err := dst.db.Exec(`CREATE TRIGGER fail_attach BEFORE UPDATE ON offsite_targets
		WHEN NEW.place_id <> ''
		BEGIN SELECT RAISE(ABORT, 'disk I/O error'); END`); err != nil {
		t.Fatal(err)
	}

	res := dst.do(http.MethodPost, "/api/settings/import?apply=true", exp)
	if msg, _ := res["error"].(string); res["ok"] != false || !strings.Contains(msg, "so this instance runs without places: ") {
		t.Fatalf("import = %v, want a failure that says the instance runs without places", res)
	}
	all, err := dst.st.ListPlaces()
	if err != nil || len(all) != 0 {
		t.Fatalf("places = %+v, %v, want none", all, err)
	}

	// The migration mark stays clear, so a later move still builds them.
	if _, err := dst.db.Exec(`DROP TRIGGER fail_attach`); err != nil {
		t.Fatal(err)
	}
	if err := dst.svc.MigrateToPlaces(); err != nil {
		t.Fatalf("a later move: %v", err)
	}
	targets, err := dst.st.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].PlaceID == "" {
		t.Errorf("targets = %+v, want the imported B2 target on a place after a later move", targets)
	}
}

// olderFileFor exports src without its places, the file an instance on places
// rebuilds them from, as the apply reads it.
func olderFileFor(t *testing.T, src *placementFixture) settingsExport {
	t.Helper()
	raw := src.do(http.MethodGet, "/api/settings/export", nil)
	delete(raw, "places")
	delete(raw, "storageDomainPlaces")
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var exp settingsExport
	if err := json.Unmarshal(b, &exp); err != nil {
		t.Fatal(err)
	}
	return exp
}

func TestAnImportWhoseScheduleCannotBeLoadedStillRebuildsThePlaces(t *testing.T) {
	src := newPlacementFixture(t)
	src.target("containers", "B2", b2Base+"/container")
	exp := olderFileFor(t, src)
	// Past the validation the handler runs, so only the reload refuses it.
	exp.Settings.ContainersSchedule = "not a cadence"

	dst := newPlacementFixture(t)
	if err := dst.svc.MigrateToPlaces(); err != nil {
		t.Fatal(err)
	}
	if err := dst.h.applyImport(context.Background(), exp); err == nil || !strings.Contains(err.Error(), "containers") {
		t.Fatalf("apply = %v, want the reload's refusal of the containers schedule", err)
	}

	if s, err := dst.st.GetSettings(); err != nil || s.PlacesMigrated == 0 {
		t.Errorf("places_migrated = %d (err %v), want the places rebuilt", s.PlacesMigrated, err)
	}
	targets, err := dst.st.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].PlaceID == "" {
		t.Errorf("targets = %+v, want the imported B2 target on a place", targets)
	}
}

func TestAnImportWaitsForAPlaceEditInProgress(t *testing.T) {
	src := newPlacementFixture(t)
	src.target("containers", "B2", b2Base+"/container")
	exp := olderFileFor(t, src)

	dst := newPlacementFixture(t)
	if err := dst.svc.MigrateToPlaces(); err != nil {
		t.Fatal(err)
	}
	before, err := dst.st.ListPlaces()
	if err != nil || len(before) == 0 {
		t.Fatalf("places = %+v, %v, want the migrated ones", before, err)
	}
	dst.svc.placeEditMu.Lock()
	done := make(chan error, 1)
	go func() { done <- dst.h.applyImport(context.Background(), exp) }()
	select {
	case err := <-done:
		t.Fatalf("an import went ahead during a place edit: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if now, err := dst.st.ListPlaces(); err != nil || len(now) != len(before) {
		t.Errorf("places during the edit = %+v, %v, want them untouched", now, err)
	}
	dst.svc.placeEditMu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the import never finished")
	}
}

func TestAnImportIsRefusedWhileABackupRuns(t *testing.T) {
	src := newPlacementFixture(t)
	src.target("containers", "B2", b2Base+"/container")
	exp := olderFileFor(t, src)

	dst := newPlacementFixture(t)
	if err := dst.svc.MigrateToPlaces(); err != nil {
		t.Fatal(err)
	}
	before, err := dst.st.ListPlaces()
	if err != nil {
		t.Fatal(err)
	}
	dst.holdDomain("vms", "backup")

	if err := dst.h.applyImport(context.Background(), exp); !errors.Is(err, errPlacementBusy) {
		t.Fatalf("apply = %v, want the busy refusal", err)
	}
	if now, err := dst.st.ListPlaces(); err != nil || !reflect.DeepEqual(now, before) {
		t.Errorf("places = %+v, %v, want them untouched", now, err)
	}
	if targets, err := dst.st.ListOffsiteTargets(); err != nil || len(targets) != 0 {
		t.Errorf("targets = %+v, %v, want nothing imported", targets, err)
	}
}

func TestAFailedImportPutsThePlacesBack(t *testing.T) {
	src := newPlacementFixture(t)
	src.target("files", "Hetzner", "sftp:u1@hz.example:/files")
	exp := olderFileFor(t, src)
	exp.NamedRepos = nil

	dst := newPlacementFixture(t)
	seed := seedPlaces(dst)
	seed.unraid.RetentionKeepLast = 30
	if _, err := dst.st.WritePlace(store.PlaceWrite{Place: seed.unraid}); err != nil {
		t.Fatal(err)
	}
	homes, err := dst.st.DomainPlaces()
	if err != nil {
		t.Fatal(err)
	}
	// Stands in for a write that fails while the places are gone.
	if _, err := dst.db.Exec(`CREATE TRIGGER fail_insert BEFORE INSERT ON offsite_targets
		BEGIN SELECT RAISE(ABORT, 'disk I/O error'); END`); err != nil {
		t.Fatal(err)
	}

	if err := dst.h.applyImport(context.Background(), exp); err == nil || !strings.Contains(err.Error(), "disk I/O error") {
		t.Fatalf("apply = %v, want the failed write", err)
	}

	unraid, err := dst.st.GetPlace(seed.unraid.ID)
	if err != nil || unraid.RetentionKeepLast != 30 {
		t.Fatalf("Unraid = %+v, %v, want it back with its own retention", unraid, err)
	}
	if now, err := dst.st.DomainPlaces(); err != nil || !maps.Equal(now, homes) {
		t.Errorf("homes = %v, %v, want %v", now, err, homes)
	}
	if repo, err := dst.st.GetNamedRepo(seed.repo.ID); err != nil || repo.PlaceID != seed.b2.ID || repo.PlaceDomain != "vms" {
		t.Errorf("B2 VMs = %+v, %v, want it back on B2", repo, err)
	}
	if s, err := dst.st.GetSettings(); err != nil || s.PlacesMigrated == 0 {
		t.Errorf("places_migrated = %d (err %v), want the places marked as there", s.PlacesMigrated, err)
	}
}

func TestTwoImportsAtOnceBothRebuildThePlaces(t *testing.T) {
	src := newPlacementFixture(t)
	src.target("containers", "B2", b2Base+"/container")
	exp := olderFileFor(t, src)

	dst := newPlacementFixture(t)
	if err := dst.svc.MigrateToPlaces(); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Go(func() { errs[i] = dst.h.applyImport(context.Background(), exp) })
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("import %d: %v", i+1, err)
		}
	}
	targets, err := dst.st.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].PlaceID == "" {
		t.Errorf("targets = %+v, want the imported B2 target on a place", targets)
	}
}
