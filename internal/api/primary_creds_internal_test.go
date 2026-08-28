package api

import (
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A domain whose PRIMARY backup path is a remote repository can now name its own
// credential set (#182, manilx: "I can't set s3 credentials when i use s3 as
// main path and offsite"). These tests exist mainly because of the trap that
// shape invites: TestPrimaryRepo already resolved the row's CredsRef, so the
// connection test would go green while every real backup still ran on the
// SHARED credentials. A selector that only works in the test is worse than no
// selector, so what is pinned here is the BACKUP path, not the probe.

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

// TestPrimaryModeForUsesItsOwnCredentialSet is #182 itself: the mode a real
// backup runs with must carry the NAMED set's keys once the primary row selects
// one.
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

// TestPrimaryModeForWithoutASetKeepsSharedCredentials is the no-regression half:
// every install that never picks a set must behave exactly as before.
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

// TestPrimaryModeForKeepsBandwidthCaps guards the refactor rather than the
// feature: the caps from issue #152 used to be assigned at each of the five
// backup call sites and now come from primaryModeFor, so losing them would be
// silent (a backup that simply saturates the uplink).
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

// TestPrimaryModeForLocalPathIgnoresStoredRow pins the guard that keeps a local
// primary byte-identical to what it was before any of this existed: no limits,
// no credential override, whatever a leftover row happens to say.
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
