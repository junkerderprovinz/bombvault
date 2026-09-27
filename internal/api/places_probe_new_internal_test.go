package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
)

func probeOf(t *testing.T, f *placementFixture, provider string, fields map[string]string) places.ProbeResult {
	t.Helper()
	res, err := f.svc.ProbePlace(context.Background(), ProbeRequest{Provider: provider, Fields: fields})
	if err != nil {
		t.Fatalf("probe %s: %v", provider, err)
	}
	return res
}

func TestAFolderOnThisServerIsProbedWhereTheFormPointsIt(t *testing.T) {
	f := newPlacementFixture(t)
	newEnvEngine(f)
	base := filepath.FromSlash(f.root + "/user/bombvault")
	if err := os.MkdirAll(filepath.Join(base, "vms"), 0o750); err != nil {
		t.Fatal(err)
	}
	res := probeOf(t, f, "unraid-folder", map[string]string{"path": "user/bombvault"})
	if !res.OK || res.Base != "user/bombvault" || res.Folders["containers"] != places.FolderAbsent || res.Folders["vms"] != places.FolderEmpty {
		t.Fatalf("probe = %+v", res)
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "vms" {
		t.Fatalf("the probe left %v behind", entries)
	}
}

func TestAFolderThatDoesNotExistYetIsProbedWhereItWillBeCreated(t *testing.T) {
	f := newPlacementFixture(t)
	newEnvEngine(f)
	user := filepath.FromSlash(f.root + "/user")
	if err := os.MkdirAll(user, 0o750); err != nil {
		t.Fatal(err)
	}
	res := probeOf(t, f, "unraid-folder", map[string]string{"path": "user/new/bombvault"})
	if !res.OK || res.Base != "user/new/bombvault" || res.Folders["containers"] != places.FolderAbsent {
		t.Fatalf("probe = %+v", res)
	}
	entries, err := os.ReadDir(user)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("the probe left %v behind", entries)
	}
}

func TestAFolderBombVaultCannotWriteToFailsTheProbe(t *testing.T) {
	f := newPlacementFixture(t)
	newEnvEngine(f)
	if err := os.MkdirAll(filepath.FromSlash(f.root+"/user"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.FromSlash(f.root+"/user/blocked"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	res := probeOf(t, f, "unraid-folder", map[string]string{"path": "user/blocked/bombvault"})
	if res.OK || res.Code != "place-probe-failed" || res.Folders != nil {
		t.Fatalf("probe = %+v, want the folder refused", res)
	}
}

func TestAnUnmountedShareFailsTheProbe(t *testing.T) {
	f := newPlacementFixture(t)
	newEnvEngine(f)
	if err := os.MkdirAll(filepath.FromSlash(f.root+"/remotes/nas/bombvault"), 0o750); err != nil {
		t.Fatal(err)
	}
	// The folder tile browses all of /mnt, so it can point into a share too.
	providers := []string{"synology", "unraid-folder"}
	writeMountFixture(t, "/")
	for _, provider := range providers {
		if res := probeOf(t, f, provider, map[string]string{"path": "remotes/nas/bombvault"}); res.OK || res.Code != "place-probe-failed" {
			t.Fatalf("%s: probe of an unmounted share = %+v", provider, res)
		}
	}
	// mountinfo escapes a space in a mount point as \040.
	writeMountFixture(t, "/", strings.ReplaceAll(f.root+"/remotes/nas", " ", `\040`))
	for _, provider := range providers {
		if res := probeOf(t, f, provider, map[string]string{"path": "remotes/nas/bombvault"}); !res.OK {
			t.Fatalf("%s: probe of a mounted share = %+v", provider, res)
		}
	}
}

func TestARestServerIsProbedUnderItsUserWithItsCredentials(t *testing.T) {
	f := newPlacementFixture(t)
	eng := newEnvEngine(f)
	base := "rest:https://nas:8000/tower"
	eng.ids[base+"/container"] = "id-container"
	for _, folder := range []string{"vms", "flash", "config", "files", "zfs"} {
		f.eng.opens[base+"/"+folder] = false
	}
	res := probeOf(t, f, "rest-server", map[string]string{"url": "https://nas:8000", "user": "tower", "password": "pw"})
	if !res.OK || res.Base != base || res.Folders["containers"] != places.FolderRepository || res.RepoIDs["containers"] != "id-container" {
		t.Fatalf("probe = %+v", res)
	}
	if env := eng.env(base + "/vms"); !slices.Equal(env, []string{"RESTIC_REST_USERNAME=tower", "RESTIC_REST_PASSWORD=pw"}) {
		t.Errorf("vms was opened with %v", env)
	}
	if _, leaked := res.Fields["password"]; leaked || res.Fields["user"] != "tower" {
		t.Errorf("fields = %v, want the form without its secret", res.Fields)
	}
}

func TestANextcloudIsProbedThroughARemoteFromTheEnvironment(t *testing.T) {
	f := newPlacementFixture(t)
	eng := newEnvEngine(f)
	res := probeOf(t, f, "nextcloud", map[string]string{"url": "https://cloud.example.com", "user": "anna", "password": "app-pass", "path": "bombvault"})
	base := "rclone:" + places.RemoteName("probe") + ":bombvault"
	// The remote a new place is reached through is named after the id it gets
	// when it is added, so the probe's own address is not shown.
	if res.Base != "" || res.Folders["containers"] == "" {
		t.Fatalf("probe = %+v, want the folders and no address", res)
	}
	want := []string{
		"RCLONE_CONFIG_BVPPROBE_TYPE=webdav",
		"RCLONE_CONFIG_BVPPROBE_URL=https://cloud.example.com/remote.php/dav/files/anna/",
		"RCLONE_CONFIG_BVPPROBE_VENDOR=nextcloud",
		"RCLONE_CONFIG_BVPPROBE_USER=anna",
		"RCLONE_CONFIG_BVPPROBE_PASS=" + places.Obscure("app-pass"),
	}
	if env := eng.env(base + "/container"); !slices.Equal(env, want) {
		t.Fatalf("env = %v\nwant %v", env, want)
	}
}

func TestANextcloudFolderThatIsARepositoryIsOfferedWithoutAnAddress(t *testing.T) {
	f := newPlacementFixture(t)
	eng := newEnvEngine(f)
	eng.ids["rclone:"+places.RemoteName("probe")+":bombvault"] = "id-dav"
	res := probeOf(t, f, "nextcloud", map[string]string{"url": "https://cloud.example.com", "user": "anna", "password": "app-pass", "path": "bombvault"})
	if !res.OK || res.Base != "" || res.RepoIDs[""] != "id-dav" {
		t.Fatalf("probe = %+v, want the repository and no address", res)
	}
}

func TestAnAzureContainerIsProbedWithTheAccountAndKey(t *testing.T) {
	f := newPlacementFixture(t)
	eng := newEnvEngine(f)
	res := probeOf(t, f, "azure", map[string]string{"account": "acct", "secret": "key", "container": "backups"})
	if res.Base != "azure:backups:" {
		t.Fatalf("base = %q", res.Base)
	}
	if env := eng.env("azure:backups:container"); !slices.Equal(env, []string{"AZURE_ACCOUNT_NAME=acct", "AZURE_ACCOUNT_KEY=key"}) {
		t.Fatalf("env = %v", env)
	}
}

func TestAStorageBoxIsProbedOverSFTPWithoutTheSharedCredentials(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCreds(CloudCreds{S3KeyID: "SHARED-KEY", S3Secret: "s"}); err != nil {
		t.Fatal(err)
	}
	eng := newEnvEngine(f)
	base := "sftp://u123456@u123456.your-storagebox.de:23/bombvault"
	f.eng.opens[base+"/container"] = false
	res := probeOf(t, f, "storagebox", map[string]string{"user": "u123456", "path": "bombvault"})
	if res.Base != base || res.Folders["containers"] != places.FolderEmpty {
		t.Fatalf("probe = %+v", res)
	}
	// sftp signs in with the container's SSH key, so no variable goes along.
	if env := eng.env(base + "/container"); len(env) != 0 {
		t.Fatalf("env = %v, want none", env)
	}
}

func TestAProbeWithoutAProviderOrARequiredFieldIsABadRequest(t *testing.T) {
	f := newPlacementFixture(t)
	newEnvEngine(f)
	if _, err := f.svc.ProbePlace(context.Background(), ProbeRequest{Provider: "dropbox"}); err == nil {
		t.Error("an unknown provider was probed")
	}
	_, err := f.svc.ProbePlace(context.Background(), ProbeRequest{Provider: "sftp", Fields: map[string]string{"user": "bv"}})
	var missing places.MissingField
	if !errors.As(err, &missing) || missing != "host" {
		t.Errorf("err = %v, want the host named", err)
	}
}
