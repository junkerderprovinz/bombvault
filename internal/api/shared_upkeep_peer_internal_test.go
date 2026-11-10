package api

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// sharedPair is an owner and a member of one shared repository, in one group.
func sharedPair(t *testing.T) (owner, member *instance, sr store.SharedRepo) {
	t.Helper()
	owner = newInstance(t, "nas", strings.Repeat("a1", 32))
	member = newInstance(t, "tower", strings.Repeat("b2", 32))
	pairThroughRelay(t, owner, member)
	members := []store.SharedMember{{Host: towerHost, Instance: member.id(t), Name: "tower"}}
	sr, err := owner.st.PutSharedRepo(store.SharedRepo{Repo: "backups/shared", Owner: owner.id(t), Members: members})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := member.st.PutSharedRepo(store.SharedRepo{Repo: "rest:http://nas:8001/shared", Owner: owner.id(t), Members: members}); err != nil {
		t.Fatal(err)
	}
	return owner, member, sr
}

// holdEntry makes the newest backup of the container name on in one that held
// less than it should, which keeps its older backups.
func holdEntry(t *testing.T, in *instance, name string) {
	t.Helper()
	if _, err := in.st.WritePlacement(store.ItemRef{Domain: "containers", Key: name}, &store.HomeWrite{Choice: store.RepoChosen}, nil, nil); err != nil {
		t.Fatal(err)
	}
	tg, err := in.st.GetTargetByContainer(name)
	if err != nil {
		t.Fatal(err)
	}
	run, err := in.st.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := in.st.FinishRun(run, "success", "aa11", 1, store.NoteOlderBackupsKept); err != nil {
		t.Fatal(err)
	}
}

func TestTheOwnerHearsThroughTheGroupWhichEntriesAMemberHolds(t *testing.T) {
	owner, member, sr := sharedPair(t)
	holdEntry(t, member, "plex")

	held, err := owner.svc.groupMembers().heldBy(context.Background(), sr.Members[0])
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(held, "container:plex") {
		t.Fatalf("held %v, want container:plex", held)
	}
}

func TestAMemberThatAgreedCopiesNothingUntilTheOwnerIsDone(t *testing.T) {
	owner, member, sr := sharedPair(t)
	ctx := context.Background()
	members := owner.svc.groupMembers()

	busy, err := members.quiet(ctx, sr.Members[0], time.Now().Add(time.Hour))
	if err != nil || busy {
		t.Fatalf("busy %v, err %v; want an agreement", busy, err)
	}
	if _, ok := member.svc.beginSharedCopy("rest:http://nas:8001/shared"); ok {
		t.Fatal("the member started a copy while the owner prunes")
	}
	members.release(ctx, sr.Members[0])
	release, ok := member.svc.beginSharedCopy("rest:http://nas:8001/shared")
	if !ok {
		t.Fatal("the member started no copy once the owner was done")
	}

	if busy, err := members.quiet(ctx, sr.Members[0], time.Now().Add(time.Hour)); err != nil || !busy {
		t.Fatalf("busy %v, err %v; want the running copy named", busy, err)
	}
	release()
}

func TestAMemberKeepsWhatItsOwnerMeasured(t *testing.T) {
	owner, member, sr := sharedPair(t)
	space := store.SharedSpace{At: 300, Total: 1000, Hosts: []store.SharedHostSpace{{Host: towerHost, Name: "tower", Bytes: 900}}}

	owner.svc.groupMembers().tellSpace(context.Background(), sr.Members[0], space)
	got, ok, err := member.st.SharedSpaceFrom(owner.id(t))
	if err != nil || !ok || !sameSpace(got, space) {
		t.Fatalf("kept %+v, %v, %v; want %+v", got, ok, err, space)
	}
}

// Another member of the group could otherwise hold up every copy to a
// repository it does not hold.
func TestOnlyTheOwnerOfASharedRepositorySpeaksForIt(t *testing.T) {
	owner, member, sr := sharedPair(t)
	if _, err := owner.st.PutSharedRepo(store.SharedRepo{ID: sr.ID, Repo: sr.Repo, Owner: member.id(t), Members: sr.Members}); err != nil {
		t.Fatal(err)
	}
	if err := member.st.DeleteSharedRepo(mustSharedRepoAt(t, member, "rest:http://nas:8001/shared").ID); err != nil {
		t.Fatal(err)
	}

	if _, err := owner.svc.groupMembers().quiet(context.Background(), sr.Members[0], time.Now().Add(time.Hour)); err == nil {
		t.Fatal("a server that holds no shared repository of the member was heard")
	}
}

func mustSharedRepoAt(t *testing.T, in *instance, repo string) store.SharedRepo {
	t.Helper()
	sr, ok, err := in.st.SharedRepoAt(repo)
	if err != nil || !ok {
		t.Fatalf("no shared repository at %s: %v", repo, err)
	}
	return sr
}

// The owner backs up into its own repository as well: it answers for its own
// host without the group.
func TestTheOwnerAnswersForItsOwnHostByItself(t *testing.T) {
	owner, _, _ := sharedPair(t)
	self := store.SharedMember{Host: cubeHost, Instance: owner.id(t), Name: "nas"}
	holdEntry(t, owner, "plex")
	members := owner.svc.groupMembers()
	ctx := context.Background()

	held, err := members.heldBy(ctx, self)
	if err != nil || !slices.Contains(held, "container:plex") {
		t.Fatalf("held %v, %v; want container:plex", held, err)
	}
	if busy, err := members.quiet(ctx, self, time.Now().Add(time.Hour)); err != nil || busy {
		t.Fatalf("busy %v, err %v", busy, err)
	}
	if _, ok := owner.svc.beginSharedCopy("backups/shared"); ok {
		t.Fatal("the owner copied into its own repository while it prunes")
	}
	members.release(ctx, self)
}
