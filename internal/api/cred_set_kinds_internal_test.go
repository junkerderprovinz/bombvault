package api

import (
	"slices"
	"testing"
)

// davSet is a Nextcloud place's credential set.
func davSet() CloudCredSet {
	return CloudCredSet{
		ID: "dav", Name: "Nextcloud", Kind: "webdav",
		WebDAVURL: "https://cloud.example.com/remote.php/dav/files/anna/", WebDAVVendor: "nextcloud",
		WebDAVUser: "anna", WebDAVPass: "app-pass",
	}
}

// blobSet is an Azure place's credential set.
func blobSet() CloudCredSet {
	return CloudCredSet{ID: "blob", Name: "Azure", Kind: "azure", AzureAccount: "acct", AzureKey: "key"}
}

func TestACredentialSetKeepsItsKindThroughTheBlob(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCredSets([]CloudCredSet{davSet(), blobSet()}); err != nil {
		t.Fatal(err)
	}
	got, err := s.decodeCloudCredSets(settingsOf(t, s))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []CloudCredSet{davSet(), blobSet()}) {
		t.Fatalf("sets = %+v", got)
	}
}

func TestTheSetListHidesWebDAVAndAzureSecrets(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCredSets([]CloudCredSet{davSet(), blobSet()}); err != nil {
		t.Fatal(err)
	}
	sets, err := s.CloudCredSets()
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 2 || sets[0].WebDAVPass != "" || sets[1].AzureKey != "" {
		t.Fatalf("the list carries a secret: %+v", sets)
	}
	if sets[0].WebDAVUser != "anna" || sets[0].Kind != "webdav" || sets[1].AzureAccount != "acct" {
		t.Fatalf("the list lost what it may show: %+v", sets)
	}
}

func TestACardThatKnowsNoKindKeepsTheSetsKind(t *testing.T) {
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCredSets([]CloudCredSet{davSet(), blobSet()}); err != nil {
		t.Fatal(err)
	}
	// What the credential sets card sends back: id, name and the fields it shows.
	if err := s.SetCloudCredSets([]CloudCredSet{{ID: "dav", Name: "Cloud"}, {ID: "blob", Name: "Azure", AzureKey: "rotated"}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.decodeCloudCredSets(settingsOf(t, s))
	if err != nil {
		t.Fatal(err)
	}
	dav, blob := davSet(), blobSet()
	dav.Name, blob.AzureKey = "Cloud", "rotated"
	if !slices.Equal(got, []CloudCredSet{dav, blob}) {
		t.Fatalf("sets = %+v\nwant %+v", got, []CloudCredSet{dav, blob})
	}
}
