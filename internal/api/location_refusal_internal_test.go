package api

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const (
	refusalBase = "s3:http://nas:9000/bucket/primary"
	refusalRepo = refusalBase + "/files"
)

// copyingLocation is a destination Folders copies to, with a repository of the
// section's to copy from.
func (f *placementFixture) copyingLocation() store.OffsiteTarget {
	f.t.Helper()
	d := f.destination("NAS", refusalBase)
	f.tickDestination(d, "files")
	preview := f.do("POST", "/api/placement/default/files/preview", map[string]any{"skip": []string{}})
	if res := f.do("PUT", "/api/placement/default/files", map[string]any{"skip": []string{}, "expect": preview["impact"]}); res["ok"] != true {
		f.t.Fatalf("placement = %v", res)
	}
	f.makeRepo(f.root + "/backups/files")
	return d
}

// refusing makes every restic call on the repository fail as the server does.
func (f *placementFixture) refusing(repo string, cause error) {
	f.eng.opens[repo] = false
	f.eng.openErr[repo] = cause
	f.eng.listErr[repo] = cause
}

func missingBucket() error {
	return fmt.Errorf("restic cat failed: %w", restic.ErrBucketMissing)
}

var wrongPassword = errors.New("restic snapshots failed: Fatal: wrong password or no key found")

func (f *placementFixture) refusedCode(d store.OffsiteTarget) string {
	f.t.Helper()
	loc, err := f.svc.StorageLocation("destination:" + d.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	return loc.Refused
}

func (f *placementFixture) lastCopyError() string {
	f.t.Helper()
	runs, err := f.st.ListRuns(5)
	if err != nil {
		f.t.Fatal(err)
	}
	i := slices.IndexFunc(runs, func(r store.Run) bool { return r.Kind == "offsite" })
	if i < 0 {
		f.t.Fatal("no copy has a run")
	}
	return runs[i].Error
}

func TestAMissingBucketRefusesARepositoryWithItsReason(t *testing.T) {
	f := newPlacementFixture(t)
	f.refusing(refusalRepo, missingBucket())

	if err := f.svc.EnsureRepo(context.Background(), refusalRepo, aMode); !errors.Is(err, errBucketMissing) {
		t.Fatalf("EnsureRepo = %v, want the bucket named", err)
	}
	if slices.Contains(f.eng.ensured, refusalRepo) {
		t.Fatal("a repository was created where the bucket is missing")
	}
}

func TestAMissingBucketIsNotARepositoryFoundGone(t *testing.T) {
	f := newPlacementFixture(t)
	d := f.destination("NAS", refusalBase)
	f.tickDestination(d, "files")
	if err := f.st.MarkRepoEstablished(refusalRepo); err != nil {
		t.Fatal(err)
	}
	f.refusing(refusalRepo, missingBucket())

	if err := f.svc.EnsureRepo(context.Background(), refusalRepo, aMode); !errors.Is(err, errBucketMissing) {
		t.Fatalf("EnsureRepo = %v, want the bucket named", err)
	}
	if f.goneLocation("destination:" + d.ID) {
		t.Error("a new repository would not bring a missing bucket back, so the page must not offer one")
	}
}

func TestTheLocationTestNamesWhatRefusedIt(t *testing.T) {
	for name, c := range map[string]struct {
		cause error
		code  string
	}{
		"a missing bucket": {missingBucket(), "bucket-missing"},
		"another key":      {wrongPassword, "repo-wrong-key"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newPlacementFixture(t)
			tgt := f.target("files", "NAS", refusalRepo)
			f.refusing(refusalRepo, c.cause)

			res := f.do("POST", "/api/offsite/targets/"+tgt.ID+"/test", nil)
			if res["ok"] != false || res["code"] != c.code {
				t.Fatalf("test = %v, want the code %s", res, c.code)
			}
		})
	}
}

func TestACopyThatFindsItsPlaceRefusedSaysWhyOnTheRunAndThePage(t *testing.T) {
	for name, c := range map[string]struct {
		cause      error
		says, code string
	}{
		"a missing bucket": {missingBucket(), "bucket does not exist", "bucket-missing"},
		"another key":      {wrongPassword, "key does not open it", "repo-wrong-key"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newPlacementFixture(t)
			d := f.copyingLocation()
			f.refusing(refusalRepo, c.cause)

			err := f.svc.ReplicateOffsite(context.Background(), "files")
			if err == nil || !strings.Contains(err.Error(), c.says) {
				t.Fatalf("copy = %v, want %q", err, c.says)
			}
			if got := f.lastCopyError(); !strings.Contains(got, c.says) {
				t.Errorf("the run says %q, want %q", got, c.says)
			}
			if got := f.refusedCode(d); got != c.code {
				t.Errorf("the page has the refusal %q, want %q", got, c.code)
			}
		})
	}
}

func TestARefusalLeavesThePageOnceACopyWorks(t *testing.T) {
	f := newPlacementFixture(t)
	d := f.copyingLocation()
	f.refusing(refusalRepo, wrongPassword)
	if err := f.svc.ReplicateOffsite(context.Background(), "files"); err == nil {
		t.Fatal("the copy went through")
	}

	delete(f.eng.opens, refusalRepo)
	delete(f.eng.openErr, refusalRepo)
	delete(f.eng.listErr, refusalRepo)
	if err := f.svc.ReplicateOffsite(context.Background(), "files"); err != nil {
		t.Fatalf("copy = %v", err)
	}
	if got := f.refusedCode(d); got != "" {
		t.Errorf("the page still has the refusal %q", got)
	}
}
