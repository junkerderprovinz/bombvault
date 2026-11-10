package api

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const sharedAt = "rest:http://nas:8001/shared"

// sharedScene is a containers domain copied to a shared repository another
// server holds, with this server's own snapshot, one from before it had a host
// of its own, and one another server left in its folder.
func sharedScene(t *testing.T) (*placementFixture, store.OffsiteTarget) {
	t.Helper()
	f := newPlacementFixture(t)
	shared := f.target("containers", "Shared", sharedAt)
	if _, err := f.st.PutSharedRepo(store.SharedRepo{Repo: sharedAt, Owner: "nas-id"}); err != nil {
		t.Fatal(err)
	}
	f.replicated("containers")
	own, legacy, other := snap("own1", 300, "container:plex"), snap("old1", 100, "container:plex"), snap("oth1", 200, "container:plex")
	own.Hostname, legacy.Hostname, other.Hostname = f.svc.resticHost(), restic.LegacyHost, "bv-ba9876543210"
	f.hold(f.domainPath("containers"), legacy, other, own)
	return f, shared
}

func copiesTo(f *placementFixture, dest string) []copyCall {
	var out []copyCall
	for _, c := range f.eng.copies {
		if c.Dest == dest {
			out = append(out, c)
		}
	}
	return out
}

func TestACopyIntoASharedRepositoryTakesOnlySnapshotsUnderThisServersHost(t *testing.T) {
	f, _ := sharedScene(t)
	if f.svc.resticHost() == "" {
		t.Fatal("the fixture has no host of its own")
	}
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	got := copiesTo(f, sharedAt)
	if len(got) != 1 || !slices.Equal(got[0].IDs, []string{"own1"}) {
		t.Fatalf("copies to the shared repository = %+v, want own1 alone", got)
	}
}

func TestASourceThatCannotBeListedCopiesNothingIntoASharedRepository(t *testing.T) {
	f, _ := sharedScene(t)
	f.eng.listErr[f.domainPath("containers")] = errors.New("connection reset")
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err == nil {
		t.Fatal("the pass reported no trouble with a source it could not read")
	}
	if got := copiesTo(f, sharedAt); len(got) != 0 {
		t.Fatalf("copies to the shared repository = %+v, want none", got)
	}
}

func TestAMemberNeverAgesASharedRepository(t *testing.T) {
	f, shared := sharedScene(t)
	keepLast(t, f, shared, 1)
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if got := forgetLines(f, sharedAt); len(got) != 0 {
		t.Fatalf("forgets at the shared repository = %v, want none", got)
	}
	if slices.Contains(f.eng.prunes, sharedAt) {
		t.Fatalf("prunes = %v, want none at the shared repository", f.eng.prunes)
	}
}
