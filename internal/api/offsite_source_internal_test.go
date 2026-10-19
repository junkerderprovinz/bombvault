package api

import (
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func namedSource(t *testing.T, svc *Service, st *store.Repo, loc string, c CloudCreds) domainRepoRef {
	t.Helper()
	ref, err := svc.addCredSet("source", c)
	if err != nil {
		t.Fatal(err)
	}
	row, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: "Source", Repo: loc, CredsRef: ref, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	return namedRef(loc, row)
}

func TestTwoS3LoginsMeetThroughAnRcloneRemote(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	src := namedSource(t, svc, st, "s3:https://s3.wasabisys.com/bucket/bv/containers-direct", CloudCreds{S3KeyID: "SRC", S3Secret: "srcsecret", S3Region: "eu-central-2"})
	settings, _ := st.GetSettings()
	dest := restic.Mode{Env: []string{"AWS_ACCESS_KEY_ID=DEST", "AWS_SECRET_ACCESS_KEY=destsecret"}}

	loc, mode, err := svc.sourceAccess(settings, "containers", src, "s3:https://s3.example.com/other/containers", dest)
	if err != nil {
		t.Fatal(err)
	}
	if loc != "rclone:BVSRC:bucket/bv/containers-direct" {
		t.Fatalf("loc = %q", loc)
	}
	for _, want := range []string{
		"AWS_ACCESS_KEY_ID=DEST",
		"RCLONE_CONFIG_BVSRC_TYPE=s3",
		"RCLONE_CONFIG_BVSRC_ACCESS_KEY_ID=SRC",
		"RCLONE_CONFIG_BVSRC_SECRET_ACCESS_KEY=srcsecret",
		"RCLONE_CONFIG_BVSRC_ENDPOINT=https://s3.wasabisys.com",
		"RCLONE_CONFIG_BVSRC_REGION=eu-central-2",
	} {
		if !slices.Contains(mode.Env, want) {
			t.Errorf("env lacks %s: %v", want, mode.Env)
		}
	}
	if len(dest.Env) != 2 {
		t.Fatalf("the destination's mode was changed: %v", dest.Env)
	}
}

func TestOneS3LoginForBothEndsChangesNothing(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	src := namedSource(t, svc, st, "s3:https://s3.example.com/a/x", CloudCreds{S3KeyID: "K", S3Secret: "S"})
	settings, _ := st.GetSettings()
	dest := restic.Mode{Env: []string{"AWS_ACCESS_KEY_ID=K", "AWS_SECRET_ACCESS_KEY=S"}}

	loc, mode, err := svc.sourceAccess(settings, "containers", src, "s3:https://s3.example.com/b/x", dest)
	if err != nil || loc != src.Loc || !slices.Equal(mode.Env, dest.Env) {
		t.Fatalf("got %q %v %v, want the source as it is", loc, mode.Env, err)
	}
}

func TestAnS3SourceToAnotherBackendBringsItsKeys(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	src := namedSource(t, svc, st, "s3:https://s3.example.com/a/x", CloudCreds{S3KeyID: "K", S3Secret: "S"})
	settings, _ := st.GetSettings()

	loc, mode, err := svc.sourceAccess(settings, "containers", src, "rclone:nas:bv/containers", restic.Mode{})
	if err != nil || loc != src.Loc || !slices.Contains(mode.Env, "AWS_ACCESS_KEY_ID=K") || !slices.Contains(mode.Env, "AWS_SECRET_ACCESS_KEY=S") {
		t.Fatalf("got %q %v %v, want the source with its own keys", loc, mode.Env, err)
	}
}

func TestTwoRestLoginsPutTheSourcesInItsAddress(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	src := namedSource(t, svc, st, "rest:https://box:8000/bv/containers-direct", CloudCreds{RESTUser: "bv", RESTPassword: "p@ss"})
	settings, _ := st.GetSettings()
	dest := restic.Mode{Env: []string{"RESTIC_REST_USERNAME=other", "RESTIC_REST_PASSWORD=x"}}

	loc, _, err := svc.sourceAccess(settings, "containers", src, "rest:https://elsewhere:8000/bv/containers", dest)
	if err != nil || loc != "rest:https://bv:p%40ss@box:8000/bv/containers-direct" {
		t.Fatalf("loc = %q, %v", loc, err)
	}
}

func TestARemotePrimaryIsReadWithItsOwnCredentialSet(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	const primary = "s3:https://s3.wasabisys.com/main/containers"
	ref, err := svc.addCredSet("main", CloudCreds{S3KeyID: "MAIN", S3Secret: "mainsecret"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertPrimaryRemoteTarget("containers", store.OffsiteTarget{Repo: primary, CredsRef: ref, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	settings, _ := st.GetSettings()
	dest := restic.Mode{Env: []string{"AWS_ACCESS_KEY_ID=OFFSITE", "AWS_SECRET_ACCESS_KEY=offsitesecret"}}

	loc, mode, err := svc.sourceAccess(settings, "containers", ownRef(primary), "s3:https://s3.example.com/offsite/containers", dest)
	if err != nil || loc != "rclone:BVSRC:main/containers" || !slices.Contains(mode.Env, "RCLONE_CONFIG_BVSRC_ACCESS_KEY_ID=MAIN") {
		t.Fatalf("got %q %v %v, want the primary through its own keys", loc, mode.Env, err)
	}
}

func TestARemotePrimaryOnTheSharedCredentialsIsReadWithThem(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	if err := svc.SetCloudCreds(CloudCreds{S3KeyID: "SHARED", S3Secret: "sharedsecret"}); err != nil {
		t.Fatal(err)
	}
	settings, _ := st.GetSettings()
	dest := restic.Mode{Env: []string{"AWS_ACCESS_KEY_ID=OFFSITE", "AWS_SECRET_ACCESS_KEY=offsitesecret"}}

	loc, mode, err := svc.sourceAccess(settings, "containers", ownRef("s3:https://s3.wasabisys.com/main/containers"), "s3:https://s3.example.com/offsite/containers", dest)
	if err != nil || loc != "rclone:BVSRC:main/containers" || !slices.Contains(mode.Env, "RCLONE_CONFIG_BVSRC_ACCESS_KEY_ID=SHARED") {
		t.Fatalf("got %q %v %v, want the primary through the shared keys", loc, mode.Env, err)
	}
}

func TestALocalPrimaryIsReadAsItIs(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	settings, _ := st.GetSettings()
	dest := restic.Mode{Env: []string{"AWS_ACCESS_KEY_ID=DEST"}}
	loc, mode, err := svc.sourceAccess(settings, "containers", ownRef("/mnt/user/backups/containers"), "s3:https://s3.example.com/x", dest)
	if err != nil || loc != "/mnt/user/backups/containers" || !slices.Equal(mode.Env, dest.Env) {
		t.Fatalf("got %q %v %v", loc, mode.Env, err)
	}
}
