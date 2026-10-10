package store_test

import (
	"database/sql"
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func aReceiveAsk(members ...string) store.ZFSReceiveSlot {
	return store.ZFSReceiveSlot{
		PeerID: "peer-tower", PeerName: "tower", ItemID: "item-1", Dataset: "cache/appdata",
		SourceServer: "tower", Members: members, ProposedKeep: store.ZFSReplicaKeep{Preset: "short"},
	}
}

func allow(t *testing.T, r *store.Repo, id string) store.ZFSReceiveSlot {
	t.Helper()
	s, err := r.DecideZFSReceive(id, store.ZFSReceiveDecision{
		State: store.ZFSReceiveAllowed, Pool: "tank", Root: "tank/bombvault-replica",
		Keep: store.DefaultZFSReplicaKeep, TokenEnc: []byte("sealed"),
	})
	if err != nil {
		t.Fatalf("allow: %v", err)
	}
	return s
}

func TestAFirstReceiveRequestWaitsWithTheProposedKeep(t *testing.T) {
	_, r := zfsStore(t)
	s, err := r.AskZFSReceive(aReceiveAsk("cache/appdata", "cache/appdata/db"), false)
	if err != nil {
		t.Fatal(err)
	}
	if s.ID == "" || s.State != store.ZFSReceiveAsked || s.AskedAt == 0 || s.TokenEnc != nil ||
		s.Keep.Preset != "short" || s.SourceServer != "tower" {
		t.Fatalf("first request = %+v", s)
	}
	got, ok, err := r.GetZFSReceiveSlot(s.ID)
	if err != nil || !ok || !slices.Equal(got.Members, s.Members) || got.ProposedKeep.Preset != "short" {
		t.Fatalf("stored slot = %+v, %v, %v", got, ok, err)
	}
}

func TestAllowingASlotStoresWhereItLandsAndItsToken(t *testing.T) {
	_, r := zfsStore(t)
	s, err := r.AskZFSReceive(aReceiveAsk("cache/appdata"), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.DecideZFSReceive(s.ID, store.ZFSReceiveDecision{State: store.ZFSReceiveRevoked}); !errors.Is(err, store.ErrZFSReceiveMove) {
		t.Fatalf("revoking an open request = %v, want ErrZFSReceiveMove", err)
	}
	allowed := allow(t, r, s.ID)
	got, _, _ := r.GetZFSReceiveSlot(s.ID)
	if got.State != store.ZFSReceiveAllowed || got.DecidedAt == 0 || string(got.TokenEnc) != "sealed" ||
		got.Pool != "tank" || got.Base() != "tank/bombvault-replica/tower" || got.Keep != allowed.Keep {
		t.Fatalf("allowed slot = %+v", got)
	}

	revoked, err := r.DecideZFSReceive(s.ID, store.ZFSReceiveDecision{State: store.ZFSReceiveRevoked})
	if err != nil || revoked.State != store.ZFSReceiveRevoked || revoked.TokenEnc != nil {
		t.Fatalf("revoke = %+v, %v", revoked, err)
	}
	if got, _, _ := r.GetZFSReceiveSlot(s.ID); got.TokenEnc != nil || got.Root != "tank/bombvault-replica" {
		t.Fatalf("a revoked slot = %+v, want no token and its place kept", got)
	}
	if _, err := r.DecideZFSReceive("nobody", store.ZFSReceiveDecision{State: store.ZFSReceiveAllowed}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deciding a missing slot = %v, want sql.ErrNoRows", err)
	}
}

func TestRepeatedReceiveRequestsKeepTheirAnswer(t *testing.T) {
	_, r := zfsStore(t)
	ask := aReceiveAsk("cache/appdata", "cache/appdata/db")
	s, err := r.AskZFSReceive(ask, false)
	if err != nil {
		t.Fatal(err)
	}
	allow(t, r, s.ID)

	again, err := r.AskZFSReceive(ask, false)
	if err != nil || again.ID != s.ID || again.State != store.ZFSReceiveAllowed || string(again.TokenEnc) != "sealed" {
		t.Fatalf("the same request again = %+v, %v; want the allowed slot", again, err)
	}
	narrow, err := r.AskZFSReceive(aReceiveAsk("cache/appdata"), false)
	if err != nil || narrow.State != store.ZFSReceiveAllowed || !slices.Equal(narrow.Members, []string{"cache/appdata"}) {
		t.Fatalf("fewer members = %+v, %v; want the slot allowed for the remaining one", narrow, err)
	}

	wider, err := r.AskZFSReceive(aReceiveAsk("cache/appdata", "cache/appdata/new"), false)
	if err != nil || wider.State != store.ZFSReceiveAsked || wider.TokenEnc != nil || wider.DecidedAt != 0 {
		t.Fatalf("more members = %+v, %v; want a new request without a token", wider, err)
	}
	if wider.Root != "tank/bombvault-replica" {
		t.Fatalf("the new request lost the place of the old answer: %+v", wider)
	}

	if _, err := r.DecideZFSReceive(s.ID, store.ZFSReceiveDecision{State: store.ZFSReceiveRefused}); err != nil {
		t.Fatal(err)
	}
	if again, _ := r.AskZFSReceive(aReceiveAsk("cache/appdata", "cache/appdata/new"), true); again.State != store.ZFSReceiveRefused {
		t.Fatalf("asking again after a refusal gave %s, want the refusal to stand", again.State)
	}
	if again, _ := r.AskZFSReceive(aReceiveAsk("cache/appdata", "cache/appdata/new", "cache/appdata/more"), false); again.State != store.ZFSReceiveAsked {
		t.Fatalf("more members after a refusal gave %s, want a new request", again.State)
	}
}

func TestARevokedSlotWaitsForARequestFromAPerson(t *testing.T) {
	_, r := zfsStore(t)
	ask := aReceiveAsk("cache/appdata")
	s, err := r.AskZFSReceive(ask, false)
	if err != nil {
		t.Fatal(err)
	}
	allow(t, r, s.ID)
	if _, err := r.DecideZFSReceive(s.ID, store.ZFSReceiveDecision{State: store.ZFSReceiveRevoked}); err != nil {
		t.Fatal(err)
	}
	if again, _ := r.AskZFSReceive(ask, false); again.State != store.ZFSReceiveRevoked {
		t.Fatalf("a run asking after the revoke gave %s, want revoked", again.State)
	}
	renewed, err := r.AskZFSReceive(ask, true)
	if err != nil || renewed.State != store.ZFSReceiveAsked || renewed.ID != s.ID {
		t.Fatalf("a renewed request = %+v, %v; want the slot asked again", renewed, err)
	}
}

func TestTheSourceFolderStaysWhenTheSourceIsRenamed(t *testing.T) {
	_, r := zfsStore(t)
	s, err := r.AskZFSReceive(aReceiveAsk("cache/appdata"), false)
	if err != nil {
		t.Fatal(err)
	}
	renamed := aReceiveAsk("cache/appdata")
	renamed.PeerName, renamed.SourceServer = "big tower", "big-tower"
	got, err := r.AskZFSReceive(renamed, false)
	if err != nil || got.PeerName != "big tower" || got.SourceServer != "tower" || got.ID != s.ID {
		t.Fatalf("after a rename = %+v, %v; want the new name over the old folder", got, err)
	}
}

func TestReceiveSlotsAddUpTheLastRunAndKeepTheirOwnRule(t *testing.T) {
	_, r := zfsStore(t)
	first, err := r.AskZFSReceive(aReceiveAsk("cache/appdata"), false)
	if err != nil {
		t.Fatal(err)
	}
	other := aReceiveAsk("cache/system")
	other.ItemID = "item-2"
	if _, err := r.AskZFSReceive(other, false); err != nil {
		t.Fatal(err)
	}
	const run1, run2 = "bombvault-replica-20261009030000", "bombvault-replica-20261010030000"
	for _, s := range []struct {
		snap string
		n    int64
	}{{run1, 100}, {run1, 7}, {run2, 40}, {run1, 3}, {run2, 7}} {
		if err := r.RecordZFSReceived(first.ID, s.snap, s.n); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.SetZFSReceiveKeep(first.ID, store.ZFSReplicaKeep{Preset: "long"}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetZFSReceiveKeep("nobody", store.DefaultZFSReplicaKeep); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("keep of a missing slot = %v, want sql.ErrNoRows", err)
	}
	all, err := r.ListZFSReceiveSlots()
	if err != nil || len(all) != 2 {
		t.Fatalf("ListZFSReceiveSlots = %+v, %v", all, err)
	}
	got, _, _ := r.GetZFSReceiveSlot(first.ID)
	if got.Bytes != 50 || got.LastSnapshot != run2 || got.LastReceived == 0 || got.Keep.Preset != "long" {
		t.Fatalf("slot after two runs = %+v, want the 50 bytes of the second", got)
	}
}

func TestAPeerAnswerStaysUntilTheTargetChanges(t *testing.T) {
	_, r := zfsStore(t)
	d := aZFSDataset(t, r, "cache/appdata")
	if err := r.SetZFSReplicaTarget(d.ID, store.ZFSReplicaTargetPeer, "member-b"); err != nil {
		t.Fatal(err)
	}
	peer := store.ZFSReplicaPeer{State: store.ZFSReceiveAllowed, Slot: "slot-1", TokenEnc: []byte("sealed"),
		Base: "tank/bombvault-replica/tower", URL: "https://10.0.0.9:3443", Pin: "abcd"}
	if err := r.SetZFSReplicaPeer(d.ID, peer); err != nil {
		t.Fatal(err)
	}
	if err := r.SetZFSReplicaTarget(d.ID, store.ZFSReplicaTargetPeer, "member-b"); err != nil {
		t.Fatal(err)
	}
	got, _ := r.GetZFSDataset(d.ID)
	if got.Replica.Peer.Slot != "slot-1" || string(got.Replica.Peer.TokenEnc) != "sealed" || got.Replica.Peer.Pin != "abcd" {
		t.Fatalf("peer answer after the same target = %+v", got.Replica.Peer)
	}
	if err := r.SetZFSReplicaTarget(d.ID, store.ZFSReplicaTargetPeer, "member-c"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetZFSDataset(d.ID); got.Replica.Peer.State != "" || got.Replica.Peer.TokenEnc != nil || got.Replica.Peer.URL != "" {
		t.Fatalf("peer answer after another target = %+v, want it gone", got.Replica.Peer)
	}
	if err := r.SetZFSReplicaPeer("nobody", peer); err == nil {
		t.Fatal("SetZFSReplicaPeer on a missing item succeeded")
	}
}

func TestTheKeepFollowsTheProposalUntilAPersonAnswers(t *testing.T) {
	_, r := zfsStore(t)
	ask := aReceiveAsk("cache/appdata")
	s, err := r.AskZFSReceive(ask, false)
	if err != nil {
		t.Fatal(err)
	}
	ask.ProposedKeep = store.ZFSReplicaKeep{Preset: "long"}
	if again, _ := r.AskZFSReceive(ask, false); again.Keep.Preset != "long" || again.ProposedKeep.Preset != "long" {
		t.Fatalf("an asked slot after a new proposal = %+v, want its keep to follow", again)
	}
	allow(t, r, s.ID)
	ask.ProposedKeep = store.ZFSReplicaKeep{Preset: "short"}
	if again, _ := r.AskZFSReceive(ask, false); again.Keep != store.DefaultZFSReplicaKeep {
		t.Fatalf("an allowed slot after a new proposal keeps %+v, want this side's rule", again.Keep)
	}
}
