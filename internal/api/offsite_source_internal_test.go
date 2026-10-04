package api

import (
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func directSource(t *testing.T, svc *Service, loc string, c CloudCreds) domainRepoRef {
	t.Helper()
	ref, err := svc.addCredSet("source", c)
	if err != nil {
		t.Fatal(err)
	}
	return namedRef(loc, store.OffsiteTarget{ID: "d", CompanionOf: "t", CredsRef: ref, Repo: loc})
}

func TestTwoS3LoginsMeetThroughAnRcloneRemote(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	src := directSource(t, svc, "s3:https://s3.wasabisys.com/bucket/bv/containers-direct", CloudCreds{S3KeyID: "SRC", S3Secret: "srcsecret", S3Region: "eu-central-2"})
	settings, _ := st.GetSettings()
	dest := restic.Mode{Env: []string{"AWS_ACCESS_KEY_ID=DEST", "AWS_SECRET_ACCESS_KEY=destsecret"}}

	loc, mode, err := svc.sourceAccess(settings, src, "s3:https://s3.example.com/other/containers", dest)
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
	src := directSource(t, svc, "s3:https://s3.example.com/a/x", CloudCreds{S3KeyID: "K", S3Secret: "S"})
	settings, _ := st.GetSettings()
	dest := restic.Mode{Env: []string{"AWS_ACCESS_KEY_ID=K", "AWS_SECRET_ACCESS_KEY=S"}}

	loc, mode, err := svc.sourceAccess(settings, src, "s3:https://s3.example.com/b/x", dest)
	if err != nil || loc != src.Loc || !slices.Equal(mode.Env, dest.Env) {
		t.Fatalf("got %q %v %v, want the source as it is", loc, mode.Env, err)
	}
}

func TestAnS3SourceToAnotherBackendBringsItsKeys(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	src := directSource(t, svc, "s3:https://s3.example.com/a/x", CloudCreds{S3KeyID: "K", S3Secret: "S"})
	settings, _ := st.GetSettings()

	loc, mode, err := svc.sourceAccess(settings, src, "rclone:nas:bv/containers", restic.Mode{})
	if err != nil || loc != src.Loc || !slices.Contains(mode.Env, "AWS_ACCESS_KEY_ID=K") || !slices.Contains(mode.Env, "AWS_SECRET_ACCESS_KEY=S") {
		t.Fatalf("got %q %v %v, want the source with its own keys", loc, mode.Env, err)
	}
}

func TestTwoRestLoginsPutTheSourcesInItsAddress(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	src := directSource(t, svc, "rest:https://box:8000/bv/containers-direct", CloudCreds{RESTUser: "bv", RESTPassword: "p@ss"})
	settings, _ := st.GetSettings()
	dest := restic.Mode{Env: []string{"RESTIC_REST_USERNAME=other", "RESTIC_REST_PASSWORD=x"}}

	loc, _, err := svc.sourceAccess(settings, src, "rest:https://elsewhere:8000/bv/containers", dest)
	if err != nil || loc != "rest:https://bv:p%40ss@box:8000/bv/containers-direct" {
		t.Fatalf("loc = %q, %v", loc, err)
	}
}

func TestTheDomainsOwnRepositoryIsReadAsItIs(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	settings, _ := st.GetSettings()
	dest := restic.Mode{Env: []string{"AWS_ACCESS_KEY_ID=DEST"}}
	loc, mode, err := svc.sourceAccess(settings, ownRef("s3:https://s3.example.com/own"), "s3:https://s3.example.com/x", dest)
	if err != nil || loc != "s3:https://s3.example.com/own" || !slices.Equal(mode.Env, dest.Env) {
		t.Fatalf("got %q %v %v", loc, mode.Env, err)
	}
}
