package api

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

func TestEnsureRepoDoesNotRecreateARemoteRepositoryThatWasThere(t *testing.T) {
	f := newPlacementFixture(t)
	loc := "rest:http://192.168.98.2:8000/nas/containers/"
	f.eng.opens[loc] = false
	if err := f.st.MarkRepoEstablished(loc); err != nil {
		t.Fatal(err)
	}
	err := f.svc.EnsureRepo(context.Background(), loc, restic.Mode{Encrypted: true, Password: "pw"})
	if !errors.Is(err, errRemoteRepoGone) {
		t.Fatalf("EnsureRepo = %v, want the gone refusal", err)
	}
	if slices.Contains(f.eng.ensured, loc) {
		t.Fatal("an empty repository was created where the old one is not mounted")
	}
}

func TestEnsureRepoCreatesARemoteRepositoryWhereNoneWasBefore(t *testing.T) {
	f := newPlacementFixture(t)
	loc := "rest:http://192.168.98.2:8000/nas/containers/"
	f.eng.opens[loc] = false
	if err := f.svc.EnsureRepo(context.Background(), loc, restic.Mode{Encrypted: true, Password: "pw"}); err != nil {
		t.Fatalf("EnsureRepo = %v, want a new repository", err)
	}
	if !slices.Contains(f.eng.ensured, loc) {
		t.Fatal("a fresh remote location was not initialised")
	}
}

func TestStartingOverLetsTheNextRunCreateTheRepository(t *testing.T) {
	f := newPlacementFixture(t)
	loc := "rest:http://192.168.98.2:8000/nas/containers/"
	target := f.target("containers", "NAS", loc)
	f.eng.opens[loc] = false
	if err := f.st.MarkRepoEstablished(loc); err != nil {
		t.Fatal(err)
	}

	res := f.do("POST", "/api/offsite/targets/"+target.ID+"/test", nil)
	if res["ok"] != false || res["code"] != codeRepoGone {
		t.Fatalf("test = %v, want the gone code", res)
	}
	if res := f.do("POST", "/api/offsite/targets/"+target.ID+"/start-over", nil); res["ok"] != true {
		t.Fatalf("start over = %v", res)
	}
	if err := f.svc.EnsureRepo(context.Background(), loc, restic.Mode{Encrypted: true, Password: "pw"}); err != nil {
		t.Fatalf("EnsureRepo after starting over = %v", err)
	}
	if !slices.Contains(f.eng.ensured, loc) {
		t.Fatal("starting over did not let the repository be created")
	}
}
