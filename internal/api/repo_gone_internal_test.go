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
	if res := f.do("POST", "/api/storage/locations/target:"+target.ID+"/start-over", nil); res["ok"] != true {
		t.Fatalf("start over = %v", res)
	}
	if err := f.svc.EnsureRepo(context.Background(), loc, restic.Mode{Encrypted: true, Password: "pw"}); err != nil {
		t.Fatalf("EnsureRepo after starting over = %v", err)
	}
	if !slices.Contains(f.eng.ensured, loc) {
		t.Fatal("starting over did not let the repository be created")
	}
}

func (f *placementFixture) goneLocation(id string) bool {
	f.t.Helper()
	loc, err := f.svc.StorageLocation(id)
	if err != nil {
		f.t.Fatal(err)
	}
	return loc.Gone
}

// A destination keeps one repository per section, and a disk emptied for good
// takes all of them, so one start over has to free every section.
func TestStartingOverAtADestinationFreesEverySectionFoundGone(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("NAS", "rest:http://192.168.98.2:8000/nas")
	mode := restic.Mode{Encrypted: true, Password: "pw"}
	var repos []string
	for _, domain := range []string{"containers", "vms"} {
		tgt := f.tickDestination(box, domain)
		repo, err := f.svc.resolveRepo(tgt.Repo)
		if err != nil {
			t.Fatal(err)
		}
		f.eng.opens[repo] = false
		if err := f.st.MarkRepoEstablished(repo); err != nil {
			t.Fatal(err)
		}
		repos = append(repos, repo)
	}
	id := "destination:" + box.ID
	if f.goneLocation(id) {
		t.Fatal("the location reads as gone before any run found it so")
	}
	for _, repo := range repos {
		if err := f.svc.EnsureRepo(context.Background(), repo, mode); !errors.Is(err, errRemoteRepoGone) {
			t.Fatalf("EnsureRepo(%s) = %v, want the gone refusal", repo, err)
		}
	}
	if !f.goneLocation(id) {
		t.Fatal("the location does not say a repository there is gone")
	}

	if res := f.do("POST", "/api/storage/locations/"+id+"/start-over", nil); res["ok"] != true {
		t.Fatalf("start over = %v", res)
	}
	for _, repo := range repos {
		if err := f.svc.EnsureRepo(context.Background(), repo, mode); err != nil {
			t.Fatalf("EnsureRepo(%s) after starting over = %v", repo, err)
		}
	}
	if f.goneLocation(id) {
		t.Fatal("the location still reads as gone after its repositories were made again")
	}
}

func TestANamedRepositoryFoundGoneCanStartOver(t *testing.T) {
	f := newPlacementFixture(t)
	repo := "s3:https://s3.example.com/bucket/containers"
	named := f.namedRepo("Bucket", repo)
	f.eng.opens[repo] = false
	if err := f.st.MarkRepoEstablished(repo); err != nil {
		t.Fatal(err)
	}
	mode := restic.Mode{Encrypted: true, Password: "pw"}
	if err := f.svc.EnsureRepo(context.Background(), repo, mode); !errors.Is(err, errRemoteRepoGone) {
		t.Fatalf("EnsureRepo = %v, want the gone refusal", err)
	}
	id := "repo:" + named.ID
	if !f.goneLocation(id) {
		t.Fatal("the named repository does not say it is gone")
	}
	if res := f.do("POST", "/api/storage/locations/"+id+"/start-over", nil); res["ok"] != true {
		t.Fatalf("start over = %v", res)
	}
	if err := f.svc.EnsureRepo(context.Background(), repo, mode); err != nil {
		t.Fatalf("EnsureRepo after starting over = %v", err)
	}
}
