package store_test

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func depositRepo(t *testing.T) *store.Repo {
	t.Helper()
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return store.New(db)
}

func TestAKeptDepositStaysDeclinedUntilAPersonAsksAgain(t *testing.T) {
	r := depositRepo(t)
	d := store.KeptDeposit{OwnerID: "tower", OwnerName: "tower", Deposit: []byte("sealed"), Salt: []byte("salt"), Version: 1, OwnerKey: []byte("key"), Counter: 1}

	state, first, err := r.KeepDeposit(d, false, 8)
	if err != nil || state != store.DepositHeld || !first {
		t.Fatalf("the first deposit: %q, first %v, %v", state, first, err)
	}
	d.Deposit, d.Counter = []byte("sealed again"), 2
	if state, first, err := r.KeepDeposit(d, false, 8); err != nil || state != store.DepositHeld || first {
		t.Fatalf("a renewal: %q, first %v, %v", state, first, err)
	}
	if got, found, _ := r.KeptDeposit("tower"); !found || string(got.Deposit) != "sealed again" {
		t.Fatalf("after the renewal the deposit reads %q, found %v", got.Deposit, found)
	}

	if had, err := r.DeclineDeposit("tower"); err != nil || !had {
		t.Fatalf("DeclineDeposit: %v, %v", had, err)
	}
	if had, err := r.DeclineDeposit("tower"); err != nil || had {
		t.Fatalf("declining twice: %v, %v", had, err)
	}
	d.Counter = 3
	if state, _, err := r.KeepDeposit(d, false, 8); err != nil || state != store.DepositDeclined {
		t.Fatalf("a renewal after the delete: %q, %v", state, err)
	}
	if got, _, _ := r.KeptDeposit("tower"); got.State != store.DepositDeclined || len(got.Deposit) != 0 {
		t.Fatalf("a declined row holds %+v", got)
	}
	if state, _, err := r.KeepDeposit(d, true, 8); err != nil || state != store.DepositHeld {
		t.Fatalf("a deposit a person asked for again: %q, %v", state, err)
	}

	if had, err := r.DropDeposit("tower"); err != nil || !had {
		t.Fatalf("DropDeposit: %v, %v", had, err)
	}
	if _, found, _ := r.KeptDeposit("tower"); found {
		t.Fatal("a dropped deposit is still there")
	}
}

func TestReleaseRequestsFollowTheirLimits(t *testing.T) {
	r := depositRepo(t)
	limits := store.ReleaseLimits{PerWindow: 2, Total: 10, Window: time.Hour, Answer: 24 * time.Hour, Collect: 30 * time.Minute, Keep: 30 * 24 * time.Hour}
	at := time.Unix(1_700_000_000, 0)

	if got, created, err := r.AskKeyRelease("tower", "fresh", "fresh", false, at, limits); err != nil || created || got.State != "" {
		t.Fatalf("looking before any request: %+v, %v, %v", got, created, err)
	}
	first, created, err := r.AskKeyRelease("tower", "fresh", "fresh", true, at, limits)
	if err != nil || !created || first.State != store.ReleaseAsked {
		t.Fatalf("the first request: %+v, %v, %v", first, created, err)
	}
	if again, created, _ := r.AskKeyRelease("tower", "fresh", "fresh", true, at.Add(time.Minute), limits); created || again.ID != first.ID {
		t.Fatalf("asking while a request waits made another: %+v", again)
	}
	if done, _ := r.CollectKeyRelease("tower", "fresh", at, limits); done {
		t.Fatal("a request nobody allowed was collected")
	}

	if _, err := r.DecideKeyRelease("no-such-id", store.ReleaseAllowed, at, limits); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("an answer to no request: %v", err)
	}
	allowed, err := r.DecideKeyRelease(first.ID, store.ReleaseAllowed, at.Add(2*time.Minute), limits)
	if err != nil || allowed.State != store.ReleaseAllowed {
		t.Fatalf("allow: %+v, %v", allowed, err)
	}
	if _, err := r.DecideKeyRelease(first.ID, store.ReleaseDeclined, at.Add(3*time.Minute), limits); !errors.Is(err, store.ErrReleaseDecided) {
		t.Fatalf("a second answer: %v", err)
	}
	if done, _ := r.CollectKeyRelease("tower", "other", at.Add(3*time.Minute), limits); done {
		t.Fatal("another instance collected the request")
	}
	if done, err := r.CollectKeyRelease("tower", "fresh", at.Add(3*time.Minute), limits); err != nil || !done {
		t.Fatalf("collect: %v, %v", done, err)
	}
	if done, _ := r.CollectKeyRelease("tower", "fresh", at.Add(4*time.Minute), limits); done {
		t.Fatal("a request was collected twice")
	}
	if got, _, _ := r.AskKeyRelease("tower", "fresh", "fresh", false, at.Add(5*time.Minute), limits); got.State != store.ReleaseDone {
		t.Fatalf("looking after the deposit went out: %q", got.State)
	}

	second, created, err := r.AskKeyRelease("tower", "fresh", "fresh", true, at.Add(6*time.Minute), limits)
	if err != nil || !created || second.ID == first.ID {
		t.Fatalf("the second request: %+v, %v, %v", second, created, err)
	}
	if _, err := r.DecideKeyRelease(second.ID, store.ReleaseAllowed, at.Add(7*time.Minute), limits); err != nil {
		t.Fatal(err)
	}
	// Allowed and not collected within the time for it.
	if done, _ := r.CollectKeyRelease("tower", "fresh", at.Add(40*time.Minute), limits); done {
		t.Fatal("an allowed request was collected after its time")
	}
	if got, _, _ := r.AskKeyRelease("tower", "fresh", "fresh", false, at.Add(41*time.Minute), limits); got.State != store.ReleaseExpired {
		t.Fatalf("an allowed request past its time reads %q", got.State)
	}
	if _, _, err := r.AskKeyRelease("tower", "fresh", "fresh", true, at.Add(42*time.Minute), limits); !errors.Is(err, store.ErrReleaseLimit) {
		t.Fatalf("a third request within the hour: %v, want %v", err, store.ErrReleaseLimit)
	}
	if _, created, err := r.AskKeyRelease("tower", "other", "other", true, at.Add(42*time.Minute), limits); err != nil || !created {
		t.Fatalf("another instance is held to the first one's limit: %v", err)
	}
	third, created, err := r.AskKeyRelease("tower", "fresh", "fresh", true, at.Add(70*time.Minute), limits)
	if err != nil || !created {
		t.Fatalf("a request after the hour: %v, %v", created, err)
	}
	// Asked and never answered.
	if got, _, _ := r.AskKeyRelease("tower", "fresh", "fresh", false, at.Add(70*time.Minute+25*time.Hour), limits); got.ID != third.ID || got.State != store.ReleaseExpired {
		t.Fatalf("a request nobody answered for a day reads %+v", got)
	}
	all, err := r.KeyReleases(10)
	if err != nil || len(all) != 4 || all[0].ID != third.ID {
		t.Fatalf("the record holds %d requests, newest %+v, %v", len(all), all, err)
	}
}

func TestANewRecoveryCodeMakesEveryDepositStale(t *testing.T) {
	r := depositRepo(t)
	if _, found, err := r.DepositIdentity(); err != nil || found {
		t.Fatalf("an identity before any code: %v, %v", found, err)
	}
	if err := r.SetDepositIdentity([]byte("recipient-1"), []byte("salt-1")); err != nil {
		t.Fatal(err)
	}
	for _, d := range []store.SentDeposit{
		{MemberID: "attic", MemberName: "attic", State: store.DepositHeld, Digest: "abc", SentAt: 5},
		{MemberID: "barn", MemberName: "barn", State: store.DepositDeclined, Digest: "abc", SentAt: 6},
	} {
		if err := r.SaveSentDeposit(d); err != nil {
			t.Fatal(err)
		}
	}

	if err := r.SetDepositIdentity([]byte("recipient-2"), []byte("salt-2")); err != nil {
		t.Fatal(err)
	}
	identity, found, err := r.DepositIdentity()
	if err != nil || !found || string(identity.RecipientEnc) != "recipient-2" || len(identity.Recipient) != 0 || string(identity.Salt) != "salt-2" {
		t.Fatalf("the identity reads %+v, %v, %v", identity, found, err)
	}
	sent, err := r.SentDeposits()
	if err != nil || len(sent) != 2 {
		t.Fatal(err)
	}
	if sent[0].State != store.DepositStale || sent[0].Digest != "" || sent[1].State != store.DepositDeclined {
		t.Fatalf("after a new code the deposits read %+v", sent)
	}
	if err := r.DeleteSentDeposit("attic"); err != nil {
		t.Fatal(err)
	}
	if err := r.ClearDepositIdentity(); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := r.DepositIdentity(); found {
		t.Fatal("the identity outlived ClearDepositIdentity")
	}
}

func TestAKeptDepositIsReplacedOnlyByANewerOneOfTheSameKey(t *testing.T) {
	r := depositRepo(t)
	d := store.KeptDeposit{OwnerID: "tower", OwnerName: "tower", Deposit: []byte("sealed"), Salt: []byte("salt"), Version: 1, OwnerKey: []byte("towers key"), Counter: 10}
	if _, _, err := r.KeepDeposit(d, true, 8); err != nil {
		t.Fatal(err)
	}

	other := d
	other.Deposit, other.OwnerKey, other.Counter = []byte("planted"), []byte("another key"), 11
	if _, _, err := r.KeepDeposit(other, true, 8); !errors.Is(err, store.ErrDepositNotOwner) {
		t.Fatalf("a deposit under another key: %v", err)
	}
	again := d
	again.Deposit = []byte("an older one sent again")
	if _, _, err := r.KeepDeposit(again, false, 8); !errors.Is(err, store.ErrDepositReplayed) {
		t.Fatalf("a deposit that is not newer: %v", err)
	}
	if got, _, _ := r.KeptDeposit("tower"); string(got.Deposit) != "sealed" || got.Counter != 10 {
		t.Fatalf("the keeper holds %q at %d", got.Deposit, got.Counter)
	}

	// Deleted by the keeper's person, the id takes the key of whoever a
	// person asks for next, and still nothing older than what it had.
	if _, err := r.DeclineDeposit("tower"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.KeepDeposit(again, true, 8); !errors.Is(err, store.ErrDepositReplayed) {
		t.Fatalf("an old deposit after the delete: %v", err)
	}
	if state, _, err := r.KeepDeposit(other, true, 8); err != nil || state != store.DepositHeld {
		t.Fatalf("a new key after the delete: %q, %v", state, err)
	}
}

func TestAKeeperTakesAFirstDepositFromOnlySoManyInstalls(t *testing.T) {
	r := depositRepo(t)
	for i, owner := range []string{"tower", "barn"} {
		d := store.KeptDeposit{OwnerID: owner, Deposit: []byte("sealed"), Salt: []byte("salt"), Version: 1, OwnerKey: []byte(owner), Counter: int64(i + 1)}
		if _, _, err := r.KeepDeposit(d, true, 2); err != nil {
			t.Fatal(err)
		}
	}
	third := store.KeptDeposit{OwnerID: "shed", Deposit: []byte("sealed"), Salt: []byte("salt"), Version: 1, OwnerKey: []byte("shed"), Counter: 1}
	if _, _, err := r.KeepDeposit(third, true, 2); !errors.Is(err, store.ErrDepositsFull) {
		t.Fatalf("a third install: %v", err)
	}
	renewal := store.KeptDeposit{OwnerID: "tower", Deposit: []byte("newer"), Salt: []byte("salt"), Version: 1, OwnerKey: []byte("tower"), Counter: 5}
	if state, _, err := r.KeepDeposit(renewal, false, 2); err != nil || state != store.DepositHeld {
		t.Fatalf("a renewal while the keeper is full: %q, %v", state, err)
	}
}

func TestReleaseRequestsOfAllInstancesTogetherAreLimited(t *testing.T) {
	r := depositRepo(t)
	limits := store.ReleaseLimits{PerWindow: 3, Total: 4, Window: time.Hour, Answer: 24 * time.Hour, Collect: 30 * time.Minute, Keep: 30 * 24 * time.Hour}
	at := time.Unix(1_700_000_000, 0)
	for i := range 4 {
		id := fmt.Sprintf("instance-%d", i)
		if _, created, err := r.AskKeyRelease("tower", id, id, true, at.Add(time.Duration(i)*time.Minute), limits); err != nil || !created {
			t.Fatalf("request %d: %v, %v", i+1, created, err)
		}
	}
	if _, _, err := r.AskKeyRelease("tower", "instance-4", "instance-4", true, at.Add(5*time.Minute), limits); !errors.Is(err, store.ErrReleaseLimit) {
		t.Fatalf("a fifth request under a fifth id: %v", err)
	}
	if _, created, err := r.AskKeyRelease("tower", "instance-4", "instance-4", true, at.Add(62*time.Minute), limits); err != nil || !created {
		t.Fatalf("a request once the hour moved on: %v, %v", created, err)
	}
}

func TestRequestsThatAreLongOverAreForgotten(t *testing.T) {
	r := depositRepo(t)
	limits := store.ReleaseLimits{PerWindow: 3, Total: 6, Window: time.Hour, Answer: 24 * time.Hour, Collect: 30 * time.Minute, Keep: 30 * 24 * time.Hour}
	at := time.Unix(1_700_000_000, 0)
	old, _, err := r.AskKeyRelease("tower", "fresh", "fresh", true, at, limits)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.DecideKeyRelease(old.ID, store.ReleaseDeclined, at.Add(time.Minute), limits); err != nil {
		t.Fatal(err)
	}
	recent, _, err := r.AskKeyRelease("tower", "fresh", "fresh", true, at.Add(29*24*time.Hour), limits)
	if err != nil {
		t.Fatal(err)
	}
	if all, _ := r.KeyReleases(10); len(all) != 2 {
		t.Fatalf("within the time requests are kept: %d", len(all))
	}
	if _, _, err := r.AskKeyRelease("tower", "fresh", "fresh", false, at.Add(31*24*time.Hour), limits); err != nil {
		t.Fatal(err)
	}
	all, err := r.KeyReleases(10)
	if err != nil || len(all) != 1 || all[0].ID != recent.ID {
		t.Fatalf("after thirty days the record holds %+v, %v", all, err)
	}
}

func TestTheDepositCounterOnlyGrowsAndFollowsTheClock(t *testing.T) {
	r := depositRepo(t)
	if err := r.SetDepositSigner([]byte("sealed seed")); err != nil {
		t.Fatal(err)
	}
	first, err := r.NextDepositCounter(1000)
	if err != nil || first != 1000 {
		t.Fatalf("the first counter: %d, %v", first, err)
	}
	if next, _ := r.NextDepositCounter(900); next != 1001 {
		t.Fatalf("with the clock set back the counter reads %d", next)
	}
	if next, _ := r.NextDepositCounter(5000); next != 5000 {
		t.Fatalf("with the clock ahead the counter reads %d", next)
	}
}

func TestADatabaseWithOneOfTheTwoDepositTablesGetsTheOther(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`DROP TABLE key_deposit_identity`,
		`DELETE FROM schema_migrations WHERE name = 'key_deposits_sent'`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = 'key_deposit_identity'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("the identity table after migrating again: %d, %v", n, err)
	}
}
