package api

import (
	"context"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// remotePathScene puts the containers domain on a remote path whose safety row
// names pathSet, copies it to one S3 target on the shared credentials, and
// leaves one snapshot to copy.
func remotePathScene(t *testing.T, path, pathSet string) (*placementFixture, *envEngine) {
	t.Helper()
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCreds(CloudCreds{S3KeyID: "SHARED-KEY", S3Secret: "s"}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetCloudCredSets([]CloudCredSet{davSet(), {ID: "acct-a", Name: "Account A", CloudCreds: CloudCreds{S3KeyID: "KEY-A", S3Secret: "a"}}}); err != nil {
		t.Fatal(err)
	}
	settings := settingsOf(t, f.svc)
	settings.ContainersPath = path
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.UpsertPrimaryRemoteTarget("containers", store.OffsiteTarget{Repo: path, CredsRef: pathSet, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	f.target("containers", "S3", "s3:https://s3.example.com/bv/containers")
	f.replicated("containers")
	f.hold(path, snap("a1", 100, "container:web"))
	return f, newEnvEngine(f)
}

func TestACopyFromAWebDAVPathCarriesBothEnvironments(t *testing.T) {
	path := "rclone:" + places.RemoteName(davPlace) + ":bombvault/container"
	f, eng := remotePathScene(t, path, "dav")
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	copies := eng.copyEnvs()
	if len(copies) != 1 {
		t.Fatalf("copies = %v, want one", copies)
	}
	for _, want := range append(davEnv(t, davPlace), "AWS_ACCESS_KEY_ID=SHARED-KEY") {
		if !slices.Contains(copies[0], want) {
			t.Errorf("the copy runs without %s: %v", want, copies[0])
		}
	}
	if listed := eng.env(path); !slices.Contains(listed, davEnv(t, davPlace)[0]) {
		t.Errorf("the source was listed with %v", listed)
	}
}

func TestACopyBetweenTwoS3AccountsKeepsTheDestinationsEnvironment(t *testing.T) {
	f, eng := remotePathScene(t, "s3:https://s3.other.example/bv/containers", "acct-a")
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	copies := eng.copyEnvs()
	if len(copies) != 1 || !slices.Contains(copies[0], "AWS_ACCESS_KEY_ID=SHARED-KEY") || slices.Contains(copies[0], "AWS_ACCESS_KEY_ID=KEY-A") {
		t.Fatalf("copies = %v, want the destination's key alone", copies)
	}
}

func TestACopyFromALocalPathRunsWithTheDestinationsEnvironmentAlone(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCreds(CloudCreds{S3KeyID: "SHARED-KEY", S3Secret: "s"}); err != nil {
		t.Fatal(err)
	}
	target := f.target("containers", "Rest", "rest:http://nas:8000/bv/containers")
	f.replicated("containers")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:web"))
	eng := newEnvEngine(f)
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	want := f.svc.offsiteModeForTarget(settingsOf(t, f.svc), target).Env
	if copies := eng.copyEnvs(); len(copies) != 1 || !slices.Equal(copies[0], want) {
		t.Fatalf("copies = %v, want %v", copies, want)
	}
}
