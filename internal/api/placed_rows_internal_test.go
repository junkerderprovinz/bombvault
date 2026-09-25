package api

import (
	"net/http"
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
