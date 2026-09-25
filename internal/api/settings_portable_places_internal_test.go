package api

import (
	"bytes"
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"

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
	if creds, _ := summary["credentials"].(map[string]any); creds["credSets"] != true {
		t.Fatalf("preview = %v, want the credential sets reported", preview)
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
