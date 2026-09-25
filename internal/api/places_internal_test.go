package api

import (
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
