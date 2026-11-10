package store_test

import (
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func ownedSharedRepo(t *testing.T, r *store.Repo) store.SharedRepo {
	t.Helper()
	sr, err := r.PutSharedRepo(store.SharedRepo{Repo: "/mnt/user/shared", Owner: "nas-id"})
	if err != nil {
		t.Fatal(err)
	}
	return sr
}

func TestANewSharedRepositoryIsKeptByTheDefaultUpkeep(t *testing.T) {
	r := sharedRepoStore(t)
	sr := ownedSharedRepo(t, r)
	u, err := r.SharedUpkeepOf(sr.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := store.SharedUpkeep{RepoID: sr.ID, PruneCadence: "weekly sun 03:00", PruneHours: 6, MaxUnused: 5, CheckSubset: 5}
	if !reflect.DeepEqual(u, want) {
		t.Fatalf("upkeep = %+v, want %+v", u, want)
	}
}

func TestTheRulesOfASharedRepositoryLeaveWhatItsPassesFoundAlone(t *testing.T) {
	r := sharedRepoStore(t)
	sr := ownedSharedRepo(t, r)
	forget := store.SharedForgetReport{At: 100, Forgotten: 3, Skipped: []string{"Cube"}, Unknown: []string{"bombvault"}}
	prune := store.SharedPruneReport{At: 200, Seconds: 42, Checked: true}
	space := store.SharedSpace{At: 300, Total: 1000, Hosts: []store.SharedHostSpace{{Host: "bv-0123456789ab", Name: "Tower", Bytes: 900}}}
	if err := r.NoteSharedForget(sr.ID, forget); err != nil {
		t.Fatal(err)
	}
	if err := r.NoteSharedPrune(sr.ID, 150, prune); err != nil {
		t.Fatal(err)
	}
	if err := r.NoteSharedSpace(sr.ID, space); err != nil {
		t.Fatal(err)
	}
	rules := store.SharedUpkeep{
		RepoID: sr.ID, Keep: store.RetentionKeep{KeepDaily: 7, KeepWeekly: 4}, KeepSet: true,
		PruneCadence: "weekly sat 01:30", PruneHours: 4, MaxUnused: 10, CheckSubset: 2,
	}
	if err := r.PutSharedUpkeepRules(rules); err != nil {
		t.Fatal(err)
	}

	got, err := r.SharedUpkeepOf(sr.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := rules
	want.PrunedWindow = 150
	want.Forget, want.Prune, want.Space = &forget, &prune, &space
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("upkeep = %+v, want %+v", got, want)
	}
}

func TestWhatAnOwnerMeasuredReachesItsMembers(t *testing.T) {
	r := sharedRepoStore(t)
	if _, ok, err := r.SharedSpaceFrom("nas-id"); err != nil || ok {
		t.Fatalf("before any word: ok %v, err %v", ok, err)
	}
	space := store.SharedSpace{At: 300, Total: 1000, Hosts: []store.SharedHostSpace{{Host: "bv-0123456789ab", Name: "Tower", Bytes: 900}}}
	if err := r.SetSharedSpaceFrom("nas-id", space); err != nil {
		t.Fatal(err)
	}
	got, ok, err := r.SharedSpaceFrom("nas-id")
	if err != nil || !ok || !reflect.DeepEqual(got, space) {
		t.Fatalf("space = %+v, %v, %v; want %+v", got, ok, err, space)
	}
}

func TestForgettingASharedRepositoryForgetsItsUpkeep(t *testing.T) {
	r := sharedRepoStore(t)
	sr := ownedSharedRepo(t, r)
	if err := r.PutSharedUpkeepRules(store.SharedUpkeep{RepoID: sr.ID, PruneCadence: "daily 02:00", PruneHours: 2, KeepSet: true, Keep: store.RetentionKeep{KeepLast: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteSharedRepo(sr.ID); err != nil {
		t.Fatal(err)
	}
	u, err := r.SharedUpkeepOf(sr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if u.KeepSet || u.PruneCadence != "weekly sun 03:00" {
		t.Fatalf("upkeep after the repository went = %+v, want the default", u)
	}
}
