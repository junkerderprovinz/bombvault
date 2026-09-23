package api

import (
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A remote primary path can name its own credential set. TestPrimaryRepo
// resolves the row's CredsRef too, so a connection test could pass while real
// backups ran with the shared credentials; these tests cover the backup path.

// primaryCredsService builds a service whose containers domain backs up to a
// remote repo, with one shared credential set and one named set to choose from.
func primaryCredsService(t *testing.T) *Service {
	t.Helper()
	s := unraidNotifyService(t, nil)
	if err := s.SetCloudCreds(CloudCreds{S3KeyID: "SHARED-KEY", S3Secret: "SHARED-SEC"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCloudCredSets([]CloudCredSet{{
		ID:         "garage",
		Name:       "Local Garage",
		CloudCreds: CloudCreds{S3KeyID: "GARAGE-KEY", S3Secret: "GARAGE-SEC", S3StorageClass: "STANDARD"},
	}}); err != nil {
		t.Fatal(err)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "s3:https://garage.example/bucket"
	if err := s.store.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	return s
}

func settingsOf(t *testing.T, s *Service) store.Settings {
	t.Helper()
	settings, err := s.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	return settings
}

func TestPrimaryModeForUsesItsOwnCredentialSet(t *testing.T) {
	s := primaryCredsService(t)
	if _, err := s.SetPrimaryRemoteConfig("containers", store.OffsiteTarget{CredsRef: "garage"}); err != nil {
		t.Fatal(err)
	}

	settings := settingsOf(t, s)
	mode := s.primaryModeFor(settings, "containers", settings.ContainersPath)
	env := strings.Join(mode.Env, "\n")
	if !strings.Contains(env, "AWS_ACCESS_KEY_ID=GARAGE-KEY") {
		t.Fatalf("a primary path naming a credential set must back up with THAT set's keys, got %v", mode.Env)
	}
	if strings.Contains(env, "SHARED-KEY") {
		t.Fatalf("the shared key must not ride along once a set is chosen, got %v", mode.Env)
	}
	if mode.StorageClass != "STANDARD" {
		t.Fatalf("the set's storage class must apply when the row has none of its own, got %q", mode.StorageClass)
	}
}

func TestPrimaryModeForWithoutASetKeepsSharedCredentials(t *testing.T) {
	s := primaryCredsService(t)
	if _, err := s.SetPrimaryRemoteConfig("containers", store.OffsiteTarget{LimitUpload: 500}); err != nil {
		t.Fatal(err)
	}

	settings := settingsOf(t, s)
	mode := s.primaryModeFor(settings, "containers", settings.ContainersPath)
	if !strings.Contains(strings.Join(mode.Env, "\n"), "AWS_ACCESS_KEY_ID=SHARED-KEY") {
		t.Fatalf("a primary row naming no set must keep using the shared credentials, got %v", mode.Env)
	}
}

// Losing the caps would go unnoticed: the backup would simply saturate the
// uplink.
func TestPrimaryModeForKeepsBandwidthCaps(t *testing.T) {
	s := primaryCredsService(t)
	if _, err := s.SetPrimaryRemoteConfig("containers", store.OffsiteTarget{
		LimitUpload: 500, LimitDownload: 250, CredsRef: "garage",
	}); err != nil {
		t.Fatal(err)
	}

	settings := settingsOf(t, s)
	mode := s.primaryModeFor(settings, "containers", settings.ContainersPath)
	if mode.Limits.UploadKBps != 500 || mode.Limits.DownloadKBps != 250 {
		t.Fatalf("the saved bandwidth caps must survive alongside the credential set, got %+v", mode.Limits)
	}
}

// A local primary gets no limits and no credential override, whatever a
// leftover row says.
func TestPrimaryModeForLocalPathIgnoresStoredRow(t *testing.T) {
	s := primaryCredsService(t)
	if _, err := s.SetPrimaryRemoteConfig("containers", store.OffsiteTarget{
		LimitUpload: 500, CredsRef: "garage",
	}); err != nil {
		t.Fatal(err)
	}

	settings := settingsOf(t, s)
	mode := s.primaryModeFor(settings, "containers", "/mnt/user/backups/containers")
	if strings.Contains(strings.Join(mode.Env, "\n"), "GARAGE-KEY") {
		t.Fatalf("a LOCAL primary path must not pick up a credential set, got %v", mode.Env)
	}
	if mode.Limits.UploadKBps != 0 {
		t.Fatalf("a LOCAL primary path must carry no bandwidth caps, got %+v", mode.Limits)
	}
}
