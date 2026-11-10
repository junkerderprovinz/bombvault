package api

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// stuckKeyringEngine cannot add the member's own key, so a join the owner
// allowed is never finished.
type stuckKeyringEngine struct{ *keyringEngine }

func (stuckKeyringEngine) AddKey(context.Context, string, restic.Mode, string, string, string) error {
	return errors.New("Fatal: unable to open the repository")
}

// The keyring's repository holds no snapshots, so it measures empty.
func (e *keyringEngine) StatsOfEveryHost(context.Context, string, string, restic.Mode) (restic.StatsResult, error) {
	return restic.StatsResult{}, nil
}

func (e *keyringEngine) Stats(context.Context, string, string, restic.Mode) (restic.StatsResult, error) {
	return restic.StatsResult{}, nil
}

// allowedNeverJoined has nas allow tower, which never finishes joining.
func allowedNeverJoined(t *testing.T) (nas, tower *instance, ring *keyring) {
	t.Helper()
	nas, tower, ring = sharedPair(t)
	setUpShared(t, nas)
	tower.svc.engine = stuckKeyringEngine{tower.svc.engine.(*keyringEngine)}
	if _, err := tower.svc.JoinSharedRepo(context.Background(), nas.id(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := nas.svc.AllowSharedJoin(context.Background(), tower.id(t)); err != nil {
		t.Fatal(err)
	}
	if got := ring.hosts(); len(got) != 2 {
		t.Fatalf("keys %v, want the owner's and the one-time key", got)
	}
	return nas, tower, ring
}

func TestAnUnusedJoinLosesItsLoginAfterADay(t *testing.T) {
	nas, tower, _ := allowedNeverJoined(t)
	in := joinOf(t, nas, store.SharedJoinIn, tower)
	allowed := time.Unix(in.DecidedAt, 0)

	if err := nas.svc.expireSharedJoins(context.Background(), allowed.Add(sharedOnceFor-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if joinOf(t, nas, store.SharedJoinIn, tower).State != store.SharedJoinAllowed || !slices.Contains(htpasswdUsers(t, nas), in.User) {
		t.Fatal("the join expired before its day was over")
	}

	if err := nas.svc.expireSharedJoins(context.Background(), allowed.Add(sharedOnceFor)); err != nil {
		t.Fatal(err)
	}
	if got := htpasswdUsers(t, nas); slices.Contains(got, in.User) {
		t.Fatalf("logins %v still hold %s", got, in.User)
	}
	after := joinOf(t, nas, store.SharedJoinIn, tower)
	if after.State != store.SharedJoinExpired || len(after.OnceEnc) != 0 || len(after.LoginEnc) != 0 || after.User != "" {
		t.Fatalf("the owner holds %+v, want an expired join without a login", after)
	}
	waitForJoin(t, tower, store.SharedJoinOut, nas, store.SharedJoinExpired)
}

func TestAnUnusedJoinLosesItsLoginEvenWhileTheUpkeepPrunes(t *testing.T) {
	nas, tower, ring := allowedNeverJoined(t)
	in := joinOf(t, nas, store.SharedJoinIn, tower)
	late := time.Unix(in.DecidedAt, 0).Add(sharedOnceFor)

	if !nas.svc.sharedUpkeep.take() {
		t.Fatal("the upkeep is busy")
	}
	if err := nas.svc.runSharedUpkeep(context.Background(), &fakeMembers{}, late); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(htpasswdUsers(t, nas), in.User) || joinOf(t, nas, store.SharedJoinIn, tower).State != store.SharedJoinExpired {
		t.Fatal("the login outlived its day while the upkeep was busy")
	}
	if got := ring.hosts(); len(got) != 2 {
		t.Fatalf("keys %v, want the one-time key left for the upkeep's turn", got)
	}
	nas.svc.sharedUpkeep.done()

	if err := nas.svc.runSharedUpkeep(context.Background(), &fakeMembers{}, late.Add(time.Minute)); err != nil {
		t.Logf("upkeep: %v", err)
	}
	if got := ring.hosts(); len(got) != 1 || joinOf(t, nas, store.SharedJoinIn, tower).OnceKey != "" {
		t.Fatalf("keys %v once the upkeep had its turn, want only the owner's", got)
	}
}

func TestAMemberWhoseKeyExpiredCanAskAgain(t *testing.T) {
	nas, tower, ring := allowedNeverJoined(t)
	in := joinOf(t, nas, store.SharedJoinIn, tower)
	if err := nas.svc.expireSharedJoins(context.Background(), time.Unix(in.DecidedAt, 0).Add(sharedOnceFor)); err != nil {
		t.Fatal(err)
	}
	waitForJoin(t, tower, store.SharedJoinOut, nas, store.SharedJoinExpired)

	if state, err := tower.svc.JoinSharedRepo(context.Background(), nas.id(t)); err != nil || state != store.SharedJoinAsked {
		t.Fatalf("asking again: %q, %v", state, err)
	}
	if joinOf(t, nas, store.SharedJoinIn, tower).State != store.SharedJoinAsked {
		t.Fatal("the owner did not take the new ask")
	}
	if _, err := nas.svc.AllowSharedJoin(context.Background(), tower.id(t)); err != nil {
		t.Fatal(err)
	}
	if got := ring.hosts(); len(got) != 2 {
		t.Fatalf("keys %v, want the owner's and only the new one-time key", got)
	}
}

func TestTheUpkeepTakesBackAOneTimeKeyNobodyUsed(t *testing.T) {
	nas, tower, ring := allowedNeverJoined(t)
	in := joinOf(t, nas, store.SharedJoinIn, tower)
	if err := nas.svc.runSharedUpkeep(context.Background(), &fakeMembers{}, time.Unix(in.DecidedAt, 0).Add(sharedOnceFor)); err != nil {
		t.Logf("upkeep: %v", err)
	}
	if got := ring.hosts(); len(got) != 1 || joinOf(t, nas, store.SharedJoinIn, tower).State != store.SharedJoinExpired {
		t.Fatalf("keys %v after the upkeep, want the one-time key gone", got)
	}
}

func TestTheOwnersCardSaysWhenAnUnusedOneTimeKeyExpires(t *testing.T) {
	nas, tower, _ := allowedNeverJoined(t)
	in := joinOf(t, nas, store.SharedJoinIn, tower)
	v, err := nas.svc.sharedServerView(context.Background())
	if err != nil || len(v.Joins) != 1 {
		t.Fatalf("joins %+v, %v", v.Joins, err)
	}
	if want := in.DecidedAt + int64(sharedOnceFor.Seconds()); v.Joins[0].ExpiresAt != want {
		t.Fatalf("expires at %d, want %d", v.Joins[0].ExpiresAt, want)
	}
}
