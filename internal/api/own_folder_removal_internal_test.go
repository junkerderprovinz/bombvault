package api

import (
	"errors"
	"testing"
)

func TestALocationHoldingAnOwnFolderIsNotRemoved(t *testing.T) {
	f := newPlacementFixture(t)
	f.rcloneConf("[vault]\ntype = sftp\nhost = vault.example.com\n")
	vault := f.destination("Vault", vaultRepo)
	ownSteps(t, f.do("POST", "/api/storage/own-folders", map[string]any{
		"domains": []string{"containers"}, "location": "destination:" + vault.ID,
	}))

	preview := f.removalPreview(vault)
	if !preview.holdsOwnFolder() {
		t.Fatalf("preview = %+v, want the own folder of containers listed", preview.Sections)
	}
	if res := f.remove(vault, preview); res["ok"] != false || res["code"] != "own-folder-there" {
		t.Fatalf("removal = %v, want the own folder named as the reason", res)
	}
	if err := f.svc.DeleteDestination(vault.ID); !errors.Is(err, errOwnFolderThere) {
		t.Fatalf("delete = %v, want errOwnFolderThere", err)
	}
	if _, ok, _ := f.st.GetDestination(vault.ID); !ok {
		t.Fatal("the location is gone")
	}
	if got := parseRcloneRemotes(f.storedRcloneConf()); len(got) != 1 || got[0] != "vault" {
		t.Fatalf("remotes = %v, want vault kept", got)
	}
}

func TestARemoteASectionPathUsesIsKept(t *testing.T) {
	f := newPlacementFixture(t)
	f.rcloneConf("[vault]\ntype = sftp\nhost = vault.example.com\n")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = vaultRepo + "/containers"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	f.svc.removeRcloneRemote("vault")

	if got := parseRcloneRemotes(f.storedRcloneConf()); len(got) != 1 {
		t.Fatalf("remotes = %v, want vault kept for the containers folder", got)
	}
}
