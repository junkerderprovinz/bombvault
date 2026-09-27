package api

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// bucketAt is an S3 bucket off the premises offering every domain in its
// default folder, with a keep-policy of its own.
func bucketAt(name, base string) store.Place {
	return store.Place{Name: name, Provider: "s3-other", Kind: string(places.KindS3), Base: base,
		Folders: places.DefaultFolders(), RetentionKeepDaily: 30, OffPremises: true, Enabled: true}
}

// storePlace writes p through the store and makes it the home place of homes.
func (f *placementFixture) storePlace(p store.Place, homes ...string) store.Place {
	f.t.Helper()
	w := store.PlaceWrite{Place: p, HomeDomains: map[string]string{}}
	for _, d := range homes {
		w.HomeDomains[d] = ""
	}
	stored, err := f.st.WritePlace(w)
	if err != nil {
		f.t.Fatalf("place %s: %v", p.Name, err)
	}
	return stored
}

// linkRow puts a row at p and writes p again, which mirrors it onto the row.
func (f *placementFixture) linkRow(rowID string, p store.Place, domain, suffix string) {
	f.t.Helper()
	tx, err := f.db.Begin()
	if err != nil {
		f.t.Fatal(err)
	}
	if err := store.AttachRowTx(tx, rowID, p.ID, domain, suffix); err != nil {
		_ = tx.Rollback()
		f.t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		f.t.Fatal(err)
	}
	f.storePlace(p)
}

// unlinkRow takes a row off its place, as a changed address does.
func (f *placementFixture) unlinkRow(rowID string) {
	f.t.Helper()
	tx, err := f.db.Begin()
	if err != nil {
		f.t.Fatal(err)
	}
	if err := store.DetachRowTx(tx, rowID); err != nil {
		_ = tx.Rollback()
		f.t.Fatalf("detach %s: %v", rowID, err)
	}
	if err := tx.Commit(); err != nil {
		f.t.Fatal(err)
	}
}

// storedTarget reads an off-site target back from the store.
func (f *placementFixture) storedTarget(id string) store.OffsiteTarget {
	f.t.Helper()
	t, found, err := f.st.GetOffsiteTarget(id)
	if err != nil || !found {
		f.t.Fatalf("target %s: found %v, %v", id, found, err)
	}
	return t
}

func TestMovingAPlacedTargetThroughTheOffsiteRouteTakesItOffItsPlace(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(bucketAt("B2", "s3:https://s3.example.com/bucket"))
	target := f.target("vms", "B2", "s3:https://s3.example.com/bucket/vms")
	f.linkRow(target.ID, b2, "vms", "")

	body := map[string]any{"domain": "vms", "name": "B2 renamed", "repo": target.Repo, "retentionKeepDaily": 30, "enabled": true}
	if res := f.do(http.MethodPut, "/api/offsite/targets/"+target.ID, body); res["ok"] != true {
		t.Fatalf("PUT = %v", res)
	}
	if got := f.storedTarget(target.ID); got.PlaceID != b2.ID || got.Name != "B2 renamed" {
		t.Fatalf("an edit that kept the address = %+v, want the target still at %s", got, b2.Name)
	}

	body["repo"] = "s3:https://s3.example.com/other/vms"
	if res := f.do(http.MethodPut, "/api/offsite/targets/"+target.ID, body); res["ok"] != true {
		t.Fatalf("PUT = %v", res)
	}
	if got := f.storedTarget(target.ID); got.PlaceID != "" || got.PlaceDomain != "" || got.Repo != "s3:https://s3.example.com/other/vms" {
		t.Fatalf("a moved target = %+v, want it at no place on its new address", got)
	}
}

func TestMovingAPlacedRepositoryThroughTheRepositoryRouteTakesItOffItsPlace(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.storePlace(store.Place{Name: "Storagebox", Provider: "storagebox", Kind: string(places.KindSFTP),
		Base: "sftp:u1@box.example.com:/bv", Folders: map[string]string{}, OffPremises: true, Enabled: true})
	repo := f.namedRepo("Storagebox", "sftp:u1@box.example.com:/bv")
	f.linkRow(repo.ID, box, "", "")

	if res := f.do(http.MethodPatch, "/api/repos/"+repo.ID, map[string]any{"name": "Box"}); res["ok"] != true {
		t.Fatalf("PATCH = %v", res)
	}
	if got, err := f.st.GetNamedRepo(repo.ID); err != nil || got.PlaceID != box.ID || got.Name != "Box" {
		t.Fatalf("a rename = %+v, %v, want the repository still at %s", got, err, box.Name)
	}

	if res := f.do(http.MethodPatch, "/api/repos/"+repo.ID, map[string]any{"repo": "sftp:u1@box.example.com:/bv2"}); res["ok"] != true {
		t.Fatalf("PATCH = %v", res)
	}
	if got, err := f.st.GetNamedRepo(repo.ID); err != nil || got.PlaceID != "" || got.Repo != "sftp:u1@box.example.com:/bv2" {
		t.Fatalf("a moved repository = %+v, %v, want it at no place on its new address", got, err)
	}
}

// placedFieldRow is the domain's off-site field row, at p.
func (f *placementFixture) placedFieldRow(domain string, p store.Place) store.OffsiteTarget {
	f.t.Helper()
	addr, ok := places.Address(p.Base, p.Folders, domain, "")
	if !ok {
		f.t.Fatalf("%s has no folder for %s", p.Name, domain)
	}
	row, err := f.st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: domain, Name: p.Name, Repo: addr, Enabled: true})
	if err != nil {
		f.t.Fatal(err)
	}
	f.linkRow(row.ID, p, domain, "")
	return f.storedTarget(row.ID)
}

func TestTheOffsiteFieldLeavesAPlacedTargetAlone(t *testing.T) {
	f := newPlacementFixture(t)
	row := f.placedFieldRow("containers", f.storePlace(bucketAt("B2", "s3:https://s3.example.com/bucket")))
	for _, field := range []string{"s3:https://s3.example.com/elsewhere/container", ""} {
		settings, err := f.st.GetSettings()
		if err != nil {
			t.Fatal(err)
		}
		settings.ContainersOffsite = field
		if err := f.svc.syncPrimaryOffsiteTarget("containers", settings); err != nil {
			t.Fatal(err)
		}
		if got := f.storedTarget(row.ID); got != row {
			t.Fatalf("with the field at %q the placed target became %+v, want %+v", field, got, row)
		}
	}
}

func TestSavingTheSharedCredentialsLeavesPlacedTargetsAlone(t *testing.T) {
	f := newPlacementFixture(t)
	row := f.placedFieldRow("containers", f.storePlace(bucketAt("B2", "s3:https://s3.example.com/bucket")))
	body := map[string]any{"s3KeyId": "key", "s3Secret": "secret", "s3StorageClass": "STANDARD_IA"}
	if res := f.do(http.MethodPost, "/api/cloud", body); res["ok"] != true {
		t.Fatalf("POST /api/cloud = %v", res)
	}
	if got := f.storedTarget(row.ID); got != row {
		t.Fatalf("placed target = %+v, want %+v", got, row)
	}
}

func TestAnImportLeavesPlacedTargetsAlone(t *testing.T) {
	f := newPlacementFixture(t)
	row := f.placedFieldRow("containers", f.storePlace(bucketAt("B2", "s3:https://s3.example.com/bucket")))
	file := f.do(http.MethodGet, "/api/settings/export", nil)
	if res := f.do(http.MethodPost, "/api/settings/import?apply=true", file); res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	if got := f.storedTarget(row.ID); got != row {
		t.Fatalf("placed target = %+v, want %+v", got, row)
	}
}

func TestAStaleSettingsSaveKeepsWhatAHomePlaceWrote(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(store.Place{Name: "Unraid", Provider: "unraid-folder", Kind: string(places.KindLocal), Base: "backups",
		Folders: map[string]string{"containers": "containers", "vms": "vms", "files": "files"}, Enabled: true}, "containers")
	b2 := f.storePlace(bucketAt("B2", "s3:https://s3.example.com/bucket"))
	f.placedFieldRow("containers", b2)
	stale, _ := f.do(http.MethodGet, "/api/settings", nil)["settings"].(map[string]any)

	unraid.Folders["containers"] = "containers-2"
	f.storePlace(unraid)
	b2.Base, b2.Immutable = "s3:https://s3.example.com/bucket-2", true
	f.storePlace(b2)
	stale["vmsPath"] = "backups/vms-2"
	if res := f.do(http.MethodPut, "/api/settings", stale); res["ok"] != true {
		t.Fatalf("PUT /api/settings = %v", res)
	}

	got, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.ContainersPath != "backups/containers-2" || got.ContainersOffsite != "s3:https://s3.example.com/bucket-2/container" || !got.ContainersOffsiteImmutable {
		t.Fatalf("containers = %q, %q, %v, want what the places wrote", got.ContainersPath, got.ContainersOffsite, got.ContainersOffsiteImmutable)
	}
	if got.VMsPath != "backups/vms-2" {
		t.Fatalf("vms path = %q, want the saved value for a domain without a home place", got.VMsPath)
	}
}

func TestAStaleSettingsSaveIsNotRefusedOverLocationsItKeeps(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(store.Place{Name: "Unraid", Provider: "unraid-folder", Kind: string(places.KindLocal), Base: "backups",
		Folders: map[string]string{"containers": "containers", "vms": "vms", "files": "files"}, Enabled: true}, "containers")
	b2 := f.storePlace(bucketAt("B2", "s3:https://s3.example.com/bucket"))
	f.placedFieldRow("containers", b2)
	stale, _ := f.do(http.MethodGet, "/api/settings", nil)["settings"].(map[string]any)

	unraid.Folders["containers"] = "containers-2"
	f.storePlace(unraid)
	b2.Base = "s3:https://s3.example.com/bucket-2"
	f.storePlace(b2)
	// The old path holds a named repository and the old off-site field
	// another target, so both stale values would clash if they were saved.
	f.namedRepo("Old containers", "backups/containers")
	f.target("vms", "Old bucket", "s3:https://s3.example.com/bucket/container/vms")
	stale["defaultLanguage"] = "de"
	if res := f.do(http.MethodPut, "/api/settings", stale); res["ok"] != true {
		t.Fatalf("PUT /api/settings = %v, want the stale locations ignored", res)
	}

	got, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.ContainersPath != "backups/containers-2" || got.ContainersOffsite != "s3:https://s3.example.com/bucket-2/container" || got.DefaultLanguage != "de" {
		t.Fatalf("containers = %q, %q, language %q, want the places' locations and the language saved", got.ContainersPath, got.ContainersOffsite, got.DefaultLanguage)
	}
}

func TestASettingsSaveNamesTheLocationsItKeptForTheirPlace(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(store.Place{Name: "Unraid", Provider: "unraid-folder", Kind: string(places.KindLocal), Base: "backups",
		Folders: map[string]string{"containers": "containers", "vms": "vms", "files": "files"}, Enabled: true}, "containers")
	nasBase := "rest:http://u:p@nas.lan:8000/bv" //nolint:gosec // G101: a test-fixture literal, not a real credential
	nas := f.storePlace(store.Place{Name: "NAS", Provider: "rest-server", Kind: string(places.KindREST),
		Base: nasBase, Folders: places.DefaultFolders(), Enabled: true})
	f.placedFieldRow("vms", nas)
	read := func() map[string]any {
		settings, _ := f.do(http.MethodGet, "/api/settings", nil)["settings"].(map[string]any)
		return settings
	}
	kept := func(res map[string]any) []any {
		t.Helper()
		if res["ok"] != true {
			t.Fatalf("PUT /api/settings = %v", res)
		}
		k, _ := res["kept"].([]any)
		return k
	}

	asRead := read()
	if asRead["vmsOffsite"] != "rest:http://[redacted]@nas.lan:8000/bv/vms" {
		t.Fatalf("vms off-site field as read = %v, want it redacted", asRead["vmsOffsite"])
	}
	if got := kept(f.do(http.MethodPut, "/api/settings", asRead)); len(got) != 0 {
		t.Fatalf("a save of the settings as read kept %v, want nothing", got)
	}
	typed := read()
	typed["containersPath"] = "backups/old"
	typed["vmsOffsite"] = "rest:http://u:p@old.lan:8000/bv/vms"
	typed["filesPath"] = "backups/files-2"
	if got := kept(f.do(http.MethodPut, "/api/settings", typed)); !reflect.DeepEqual(got, []any{"containersPath", "vmsOffsite"}) {
		t.Fatalf("kept = %v, want the path and the off-site field their places own", got)
	}
}

func TestAFlashChipSwitchedOffStaysOffAfterAnUnrelatedSettingsSave(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(bucketAt("B2", "s3:https://s3.example.com/bucket"))
	row := f.placedFieldRow("flash", b2)
	stale, _ := f.do(http.MethodGet, "/api/settings", nil)["settings"].(map[string]any)

	row.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(row); err != nil {
		t.Fatal(err)
	}
	f.storePlace(b2)
	stale["defaultLanguage"] = "de"
	if res := f.do(http.MethodPut, "/api/settings", stale); res["ok"] != true {
		t.Fatalf("PUT /api/settings = %v", res)
	}

	if got := f.storedTarget(row.ID); got.Enabled || got.PlaceID != b2.ID {
		t.Fatalf("flash target = %+v, want it off at %s", got, b2.Name)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.FlashOffsite != "" || settings.DefaultLanguage != "de" {
		t.Fatalf("flash field = %q, language %q, want the field empty and the language saved", settings.FlashOffsite, settings.DefaultLanguage)
	}
	if targets := f.svc.offsiteReplicationTargets("flash", settings); len(targets) != 0 {
		t.Fatalf("flash still copies to %+v", targets)
	}
}

func TestAConfigChipSwitchedOffStaysOffWhenItsPlaceIsSwitchedOffAndOn(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(bucketAt("B2", "s3:https://s3.example.com/bucket"))
	flash := f.placedFieldRow("flash", b2)
	config := f.placedFieldRow("config", b2)
	config.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(config); err != nil {
		t.Fatal(err)
	}
	f.storePlace(b2)

	for _, on := range []bool{false, true} {
		if res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"enabled": on}); res["ok"] != true {
			t.Fatalf("PATCH enabled %v = %v", on, res)
		}
	}

	if got := f.storedTarget(flash.ID); !got.Enabled {
		t.Errorf("flash target = %+v, want it back on with its place", got)
	}
	if got := f.storedTarget(config.ID); got.Enabled {
		t.Errorf("config target = %+v, want it off as it was before its place went off", got)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.FlashOffsite != flash.Repo || settings.ConfigOffsite != "" {
		t.Fatalf("flash field = %q, config field = %q, want flash filled and config empty", settings.FlashOffsite, settings.ConfigOffsite)
	}
}

// acceptedOffer leaves an accepted mesh offer of location, as an accept does.
func (f *placementFixture) acceptedOffer(location string) {
	f.t.Helper()
	offer, err := f.st.CreateMeshOffer(store.MeshOffer{From: "tower", Repo: location})
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.st.UpdateMeshOfferStatus(offer.ID, "accepted"); err != nil {
		f.t.Fatal(err)
	}
}

// towerPlace is another BombVault's rest-server, offering containers.
func (f *placementFixture) towerPlace() store.Place {
	f.t.Helper()
	return f.storePlace(store.Place{Name: "Tower", Provider: "bombvault", Kind: string(places.KindREST),
		Base: "rest:http://tower:8000/bv", Folders: map[string]string{"containers": "containers"}, OffPremises: true, Enabled: true})
}

func TestAPlacedMeshTargetOnSortOrderZeroStaysThere(t *testing.T) {
	f := newPlacementFixture(t)
	tower := f.towerPlace()
	row := f.placedFieldRow("containers", tower)
	row.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(row); err != nil {
		t.Fatal(err)
	}
	f.storePlace(tower)
	row = f.storedTarget(row.ID)
	f.acceptedOffer(row.Repo)

	if moved, err := f.svc.MoveTargetsOffPrimarySlot(); err != nil || moved != 0 {
		t.Fatalf("moved %d, %v, want nothing moved", moved, err)
	}
	if got := f.storedTarget(row.ID); got != row {
		t.Fatalf("mesh target = %+v, want it as its place left it: %+v", got, row)
	}
}

func TestTheOffsiteFieldOnAPlacedTargetsAddressAddsNoSecondRow(t *testing.T) {
	f := newPlacementFixture(t)
	row := f.placeTarget(f.storePlace(bucketAt("B2", "s3:https://s3.example.com/bucket")), "containers", "")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersOffsite = row.Repo
	if err := f.svc.syncPrimaryOffsiteTarget("containers", settings); err != nil {
		t.Fatal(err)
	}
	want := row
	want.SortOrder = 0
	if got := f.storedTarget(row.ID); got != want {
		t.Fatalf("placed target = %+v, want it as its place left it, as the field's row: %+v", got, want)
	}
	if targets, err := f.st.OffsiteTargetsForDomain("containers"); err != nil || len(targets) != 1 {
		t.Fatalf("containers targets = %+v, %v, want only the placed one", targets, err)
	}
}

func TestTheOffsiteFieldOnAPlacedTargetsAddressFollowsThePlace(t *testing.T) {
	for name, leave := range map[string]func(*placementFixture, store.Place){
		"switched off": func(f *placementFixture, p store.Place) {
			p.Enabled = false
			f.storePlace(p)
		},
		"removed": func(f *placementFixture, p store.Place) {
			if _, err := f.st.DeletePlaceIfUnused(p.ID); err != nil {
				f.t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newPlacementFixture(t)
			b2 := f.storePlace(bucketAt("B2", "s3:https://s3.example.com/bucket"))
			row := f.placeTarget(b2, "containers", "")
			typed, _ := f.do(http.MethodGet, "/api/settings", nil)["settings"].(map[string]any)
			typed["containersOffsite"] = row.Repo
			if res := f.do(http.MethodPut, "/api/settings", typed); res["ok"] != true {
				t.Fatalf("PUT /api/settings = %v", res)
			}

			leave(f, b2)
			next, _ := f.do(http.MethodGet, "/api/settings", nil)["settings"].(map[string]any)
			if res := f.do(http.MethodPut, "/api/settings", next); res["ok"] != true {
				t.Fatalf("PUT /api/settings = %v", res)
			}

			settings, err := f.st.GetSettings()
			if err != nil {
				t.Fatal(err)
			}
			if settings.ContainersOffsite != "" {
				t.Fatalf("containers field = %q, want it empty with its place %s", settings.ContainersOffsite, name)
			}
			if targets := f.svc.offsiteReplicationTargets("containers", settings); len(targets) != 0 {
				t.Fatalf("containers still copies to %+v", targets)
			}
		})
	}
}

func TestTheOldRoutesCannotChangeWhatAPlaceSetsOnItsRows(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(bucketAt("B2", "s3:https://s3.example.com/bucket"))
	target := f.placeTarget(b2, "vms", "")
	repo := f.namedRepo("B2 files", "s3:https://s3.example.com/bucket/files")
	f.linkRow(repo.ID, b2, "files", "")
	repo, _ = f.st.GetNamedRepo(repo.ID)

	body := map[string]any{"domain": "vms", "name": "B2", "repo": target.Repo, "immutable": true,
		"retentionKeepDaily": 30, "enabled": true}
	if res := f.do(http.MethodPut, "/api/offsite/targets/"+target.ID, body); res["ok"] != false || res["code"] != "place-owned-field" {
		t.Fatalf("PUT append-only on a placed target = %v, want place-owned-field", res)
	}
	if got := f.storedTarget(target.ID); got != target {
		t.Fatalf("target = %+v after the refusal, want %+v", got, target)
	}
	if res := f.do(http.MethodPatch, "/api/repos/"+repo.ID, map[string]any{"immutable": true}); res["ok"] != false || res["code"] != "place-owned-field" {
		t.Fatalf("PATCH append-only on a placed repository = %v, want place-owned-field", res)
	}
	if got, _ := f.st.GetNamedRepo(repo.ID); got != repo {
		t.Fatalf("repository = %+v after the refusal, want %+v", got, repo)
	}
}

func TestADomainPathAtAPlaceKeepsItsSafetySettingsFromTheOldRoutes(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(bucketAt("B2", "s3:https://s3.example.com/bucket"), "containers")
	before, found, err := f.st.PrimaryRemoteTarget("containers")
	if err != nil || !found {
		t.Fatalf("primary row = %v, %v, want one", found, err)
	}

	res := f.do(http.MethodPut, "/api/settings/primary-remote/containers", map[string]any{"immutable": true})
	if res["ok"] != false || res["code"] != "place-owned-field" {
		t.Fatalf("PUT = %v, want place-owned-field", res)
	}
	res = f.do(http.MethodDelete, "/api/settings/primary-remote/containers", nil)
	if res["ok"] != false || res["code"] != "place-owned-field" {
		t.Fatalf("DELETE = %v, want place-owned-field", res)
	}
	if got, found, err := f.st.PrimaryRemoteTarget("containers"); err != nil || !found || got != before {
		t.Fatalf("primary row = %+v, %v, %v, want it as the place wrote it", got, found, err)
	}
}
