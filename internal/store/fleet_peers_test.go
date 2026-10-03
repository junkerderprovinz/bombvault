package store_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestFleetPeerCRUD(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	got, err := r.CreateFleetPeer(store.FleetPeer{MemberID: "member-a", Name: "tower", Enabled: true})
	if err != nil {
		t.Fatalf("CreateFleetPeer: %v", err)
	}
	if got.ID == "" || got.CreatedAt == 0 {
		t.Fatalf("CreateFleetPeer did not assign an id and a creation time: %+v", got)
	}

	back, ok, err := r.GetFleetPeer(got.ID)
	if err != nil || !ok {
		t.Fatalf("GetFleetPeer: ok=%v err=%v", ok, err)
	}
	if back.Name != "tower" || back.MemberID != "member-a" || !back.Enabled || back.NeedsPairing() {
		t.Fatalf("GetFleetPeer round-trip mismatch: %+v", back)
	}
	if back.LastPollOK.Valid {
		t.Fatalf("a fresh peer must have LastPollOK unset, got %+v", back.LastPollOK)
	}

	back.Name = "tower (renamed)"
	back.Enabled = false
	if err := r.UpdateFleetPeer(back); err != nil {
		t.Fatalf("UpdateFleetPeer: %v", err)
	}
	if err := r.UpdateFleetPeerPollResult(got.ID, 1_700_000_000, sql.NullBool{Valid: true, Bool: true}, "", "tower-instance", "1.2.3", `[{"domain":"containers"}]`); err != nil {
		t.Fatalf("UpdateFleetPeerPollResult: %v", err)
	}
	polled, ok, err := r.GetFleetPeer(got.ID)
	if err != nil || !ok {
		t.Fatalf("GetFleetPeer after poll result: ok=%v err=%v", ok, err)
	}
	if polled.LastPollAt != 1_700_000_000 || !polled.LastPollOK.Bool ||
		polled.LastPollInstanceName != "tower-instance" || polled.LastPollVersion != "1.2.3" ||
		polled.LastPollDomainsJSON != `[{"domain":"containers"}]` {
		t.Fatalf("UpdateFleetPeerPollResult round-trip mismatch: %+v", polled)
	}
	if polled.Name != "tower (renamed)" || polled.Enabled {
		t.Fatalf("a poll result must not touch the configuration: %+v", polled)
	}

	if err := r.DeleteFleetPeer(got.ID); err != nil {
		t.Fatalf("DeleteFleetPeer: %v", err)
	}
	if _, ok, err := r.GetFleetPeer(got.ID); err != nil || ok {
		t.Fatalf("GetFleetPeer after delete: ok=%v err=%v", ok, err)
	}
}

func TestGroupStateStartsWithAnInstanceIDAndNoGroup(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	g, err := r.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	if len(g.InstanceID) != 32 || len(g.SecretEnc) != 0 || g.RelayMode != "project" || g.RelayServe {
		t.Fatalf("fresh group state = %+v, want a 32-hex id, no secret, the project relay", g)
	}
	if err := r.SetGroupSecret([]byte("sealed"), time.Unix(1_700_000_000, 0)); err != nil {
		t.Fatal(err)
	}
	if err := r.SetGroupRelay("own", "https://relay.example.org", true); err != nil {
		t.Fatal(err)
	}
	after, err := r.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	if after.InstanceID != g.InstanceID || string(after.SecretEnc) != "sealed" ||
		after.RelayMode != "own" || after.RelayURL != "https://relay.example.org" || !after.RelayServe {
		t.Fatalf("group state did not round-trip: %+v", after)
	}
}

func TestGroupStateRemembersWhenTheGroupWasEnteredAndFirstShared(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	if err := r.MarkGroupMemberSeen(time.Unix(1_700_000_010, 0)); err != nil {
		t.Fatal(err)
	}
	if g, _ := r.GetGroupState(); !g.JoinedAt.IsZero() || !g.MemberSeenAt.IsZero() {
		t.Fatalf("outside a group: joined %v, seen %v; want both zero", g.JoinedAt, g.MemberSeenAt)
	}

	joined := time.Unix(1_700_000_000, 0)
	if err := r.SetGroupSecret([]byte("sealed"), joined); err != nil {
		t.Fatal(err)
	}
	for _, at := range []int64{1_700_000_030, 1_700_000_090} {
		if err := r.MarkGroupMemberSeen(time.Unix(at, 0)); err != nil {
			t.Fatal(err)
		}
	}
	g, err := r.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	if !g.JoinedAt.Equal(joined) || g.MemberSeenAt.Unix() != 1_700_000_030 {
		t.Fatalf("joined %v, seen %v; want the join time and the first sighting", g.JoinedAt, g.MemberSeenAt)
	}

	if err := r.SetGroupSecret([]byte("other"), joined.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if g, _ := r.GetGroupState(); !g.JoinedAt.Equal(joined.Add(time.Hour)) || !g.MemberSeenAt.IsZero() {
		t.Fatalf("after joining another group: joined %v, seen %v; want a fresh start", g.JoinedAt, g.MemberSeenAt)
	}
	if err := r.SetGroupSecret(nil, joined.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if g, _ := r.GetGroupState(); !g.JoinedAt.IsZero() || !g.MemberSeenAt.IsZero() {
		t.Fatalf("after leaving: joined %v, seen %v; want both zero", g.JoinedAt, g.MemberSeenAt)
	}
}

func TestAFleetPeerKeepsTheKindOfMemberItIs(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	p, err := r.CreateFleetPeer(store.FleetPeer{MemberID: "phone", Name: "Pixel", Kind: "android", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	p.Name = "Pixel 8"
	if err := r.UpdateFleetPeer(p); err != nil {
		t.Fatal(err)
	}
	back, _, err := r.GetFleetPeer(p.ID)
	if err != nil || back.Kind != "android" {
		t.Fatalf("kind after a rename = %q (err %v), want android", back.Kind, err)
	}
}
