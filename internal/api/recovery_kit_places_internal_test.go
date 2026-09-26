package api

import (
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestTheRecoveryKitNamesEveryPlaceAndTheVariablesItRunsWith(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{
		{ID: "dav", Name: "Cloud", Kind: "webdav", WebDAVURL: "https://cloud.example.com/remote.php/dav/files/anna/",
			WebDAVVendor: "nextcloud", WebDAVUser: "anna", WebDAVPass: "app-pass"},
		{ID: "blob", Name: "Blob", Kind: "azure", AzureAccount: "acct", AzureKey: "key"},
	}); err != nil {
		t.Fatal(err)
	}
	cloud := f.storePlace(store.Place{ID: "0a1b", Name: "Cloud", Provider: "nextcloud", Kind: "webdav",
		Base: "rclone:" + places.RemoteName("0a1b") + ":bombvault", Folders: places.DefaultFolders(), CredsRef: "dav", Enabled: true})
	f.storePlace(store.Place{Name: "Blob", Provider: "azure", Kind: "azure", Base: "azure:backups:",
		Folders: places.DefaultFolders(), CredsRef: "blob", Enabled: true})
	f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	f.storePlace(localPlace("Unraid", "backups"))
	unraid, err := f.svc.resolveRepo("backups")
	if err != nil {
		t.Fatal(err)
	}

	kit, err := f.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}

	remote := "RCLONE_CONFIG_" + strings.ToUpper(places.RemoteName(cloud.ID)) + "_"
	for _, want := range []string{
		"## Storage places",
		"- Cloud: rclone:" + places.RemoteName(cloud.ID) + ":bombvault\n",
		remote + "URL=https://cloud.example.com/remote.php/dav/files/anna/",
		remote + "USER=anna",
		remote + "PASS=" + places.Obscure("app-pass"),
		"- Blob: azure:backups:\n",
		"AZURE_ACCOUNT_NAME=acct",
		"AZURE_ACCOUNT_KEY=key",
		"- B2: s3:https://s3.example.com/bucket\n  uses the shared credentials above",
		"- Unraid: " + unraid + "\n",
	} {
		if !strings.Contains(kit, want) {
			t.Errorf("the kit lacks %q", want)
		}
	}
}
