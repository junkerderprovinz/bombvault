package api

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// davPlace is the id of the Nextcloud place davSet belongs to.
const davPlace = "0a1b"

// davEnv is what davSet renders into for the remote of placeID.
func davEnv(placeID string) []string {
	prefix := "RCLONE_CONFIG_" + strings.ToUpper(places.RemoteName(placeID)) + "_"
	return []string{
		prefix + "TYPE=webdav",
		prefix + "URL=https://cloud.example.com/remote.php/dav/files/anna/",
		prefix + "VENDOR=nextcloud",
		prefix + "USER=anna",
		prefix + "PASS=" + places.Obscure("app-pass"),
	}
}

// kindService holds shared S3 credentials and the WebDAV and Azure sets.
func kindService(t *testing.T) *Service {
	t.Helper()
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCreds(CloudCreds{S3KeyID: "SHARED-KEY", S3Secret: "SHARED-SEC"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCloudCredSets([]CloudCredSet{davSet(), blobSet()}); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAWebDAVRowRunsWithTheRemoteItsAddressNames(t *testing.T) {
	s := kindService(t)
	target := store.OffsiteTarget{Domain: "containers", Repo: "rclone:" + places.RemoteName(davPlace) + ":bombvault/containers", Enabled: true, CredsRef: "dav"}
	if got := s.offsiteModeForTarget(settingsOf(t, s), target).Env; !slices.Equal(got, davEnv(davPlace)) {
		t.Fatalf("env = %v\nwant %v", got, davEnv(davPlace))
	}
}

func TestAnAzureRowRunsWithTheAccountAndKey(t *testing.T) {
	s := kindService(t)
	target := store.OffsiteTarget{Domain: "vms", Repo: "azure:backups:/vms", Enabled: true, CredsRef: "blob"}
	want := []string{"AZURE_ACCOUNT_NAME=acct", "AZURE_ACCOUNT_KEY=key"}
	if got := s.offsiteModeForTarget(settingsOf(t, s), target).Env; !slices.Equal(got, want) {
		t.Fatalf("env = %v, want %v", got, want)
	}
}

func TestAWebDAVDomainPathBacksUpThroughTheRemoteOfItsPath(t *testing.T) {
	s := kindService(t)
	settings := settingsOf(t, s)
	settings.ContainersPath = "rclone:" + places.RemoteName(davPlace) + ":bombvault/container"
	if err := s.store.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	// The row's address is the path at its last save, which names another remote.
	if _, err := s.store.UpsertPrimaryRemoteTarget("containers", store.OffsiteTarget{Repo: "rclone:bvpstale:old", CredsRef: "dav", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	got := s.primaryModeFor(settingsOf(t, s), "containers", settings.ContainersPath).Env
	if !slices.Equal(got, davEnv(davPlace)) {
		t.Fatalf("env = %v\nwant %v", got, davEnv(davPlace))
	}
}

func TestAPlaceRunsWithTheEnvironmentOfItsRows(t *testing.T) {
	s := kindService(t)
	dav := store.Place{ID: davPlace, Kind: "webdav", CredsRef: "dav"}
	env, err := s.placeEnv(dav)
	if err != nil || !slices.Equal(env, davEnv(davPlace)) {
		t.Fatalf("placeEnv = %v, %v", env, err)
	}
	creds, err := s.placeCreds(dav)
	if err != nil || creds.WebDAVPass != "app-pass" || creds.WebDAVUser != "anna" {
		t.Fatalf("placeCreds = %+v, %v", creds, err)
	}
	shared, err := s.placeEnv(store.Place{ID: "c2", Kind: "s3"})
	if err != nil || !slices.Equal(shared, s.ModeFor(settingsOf(t, s)).Env) {
		t.Fatalf("a place on the shared credentials runs with %v, %v", shared, err)
	}
}

func TestAWebDAVPathIsMeasuredThroughTheRemoteOfItsPath(t *testing.T) {
	s := kindService(t)
	settings := settingsOf(t, s)
	settings.ContainersPath = "rclone:" + places.RemoteName(davPlace) + ":bombvault/container"
	if err := s.store.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.UpsertPrimaryRemoteTarget("containers", store.OffsiteTarget{Repo: settings.ContainersPath, CredsRef: "dav", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	s.anomalies = newAnomalyEngine(s, func() time.Time { return time.Unix(testNow, 0) })
	var asked []string
	s.rcloneAbout = func(_ context.Context, remote string, env []string) (aboutResult, error) {
		asked = env
		return aboutResult{Free: 1 << 40}, nil
	}
	s.sampleVolumesFor(context.Background(), "containers")
	for _, want := range davEnv(davPlace) {
		if !slices.Contains(asked, want) {
			t.Fatalf("rclone about ran with %v, want the place's remote %v", asked, davEnv(davPlace))
		}
	}
}
