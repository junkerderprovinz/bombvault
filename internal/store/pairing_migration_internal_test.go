package store

import (
	"testing"
	"time"
)

// An instance updated from a build with fleet tokens and hand-entered APP_KEYs
// keeps every peer, receiver and pull source, each marked as needing to be
// paired again, and loses the tokens.
func TestPairingMigrationKeepsOldEntriesAndMarksThemForPairing(t *testing.T) {
	db := OpenMem(t)
	bootAs(t, db, pairingMigration-1)

	seed := []string{
		`UPDATE settings SET fleet_token = 'deadbeefcafef00d', instance_name = 'tower' WHERE id = 1`,
		`INSERT INTO fleet_peers (id, name, url, token_enc, enabled, created_at, last_poll_instance_name)
		 VALUES ('fp1', 'attic', 'https://192.168.1.9:3443', x'0102', 1, 1700000000, 'attic')`,
		`INSERT INTO received_repos (id, name, repo, app_key_enc, created_at)
		 VALUES ('rr1', 'from attic', '/host/user/offsite/attic', x'0304', 1700000000)`,
		`INSERT INTO pull_sources (id, name, repo, app_key_enc, domain, created_at)
		 VALUES ('ps1', 'attic', 'rest:http://192.168.1.9:8000/containers', x'0506', 'containers', 1700000000)`,
	}
	for _, q := range seed {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	r := New(db)

	peers, err := r.ListFleetPeers()
	if err != nil || len(peers) != 1 {
		t.Fatalf("fleet peers after migration = %+v, %v; want the one peer kept", peers, err)
	}
	if !peers[0].NeedsPairing() || peers[0].URL != "https://192.168.1.9:3443" || peers[0].LastPollInstanceName != "attic" {
		t.Fatalf("fleet peer after migration = %+v, want it kept and marked for pairing", peers[0])
	}
	var token []byte
	if err := db.QueryRow(`SELECT token_enc FROM fleet_peers WHERE id = 'fp1'`).Scan(&token); err != nil || len(token) != 0 {
		t.Fatalf("the peer's fleet token survived the migration: %x %v", token, err)
	}
	var own string
	if err := db.QueryRow(`SELECT fleet_token FROM settings WHERE id = 1`).Scan(&own); err != nil || own != "" {
		t.Fatalf("this instance's fleet token survived the migration: %q %v", own, err)
	}

	rr, ok, err := r.GetReceivedRepo("rr1")
	if err != nil || !ok {
		t.Fatalf("received repo lost: %v", err)
	}
	if !rr.NeedsPairing() || string(rr.LegacyAppKeyEnc) != "\x03\x04" || len(rr.ResticPasswordEnc) != 0 {
		t.Fatalf("received repo after migration = %+v, want it marked with its old key kept for conversion", rr)
	}
	ps, ok, err := r.GetPullSource("ps1")
	if err != nil || !ok {
		t.Fatalf("pull source lost: %v", err)
	}
	if !ps.NeedsPairing() || string(ps.LegacyAppKeyEnc) != "\x05\x06" || len(ps.ResticPasswordEnc) != 0 {
		t.Fatalf("pull source after migration = %+v, want it marked with its old key kept for conversion", ps)
	}

	g, err := r.GetGroupState()
	if err != nil || len(g.InstanceID) != 32 || len(g.SecretEnc) != 0 {
		t.Fatalf("group state after migration = %+v, %v", g, err)
	}
}

// A group that exists when the upgrade runs counts as entered then, so its
// page gets a minute of searching before it says anything.
func TestAnExistingGroupCountsAsEnteredAtTheUpgrade(t *testing.T) {
	db := OpenMem(t)
	bootAs(t, db, pairingMigration+4)
	if _, err := db.Exec(`UPDATE group_state SET secret_enc = x'0102' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	before := time.Now().Add(-time.Second)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	g, err := New(db).GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	if g.JoinedAt.Before(before) || g.JoinedAt.After(time.Now().Add(time.Second)) || !g.MemberSeenAt.IsZero() {
		t.Fatalf("joined %v, seen %v; want now and never", g.JoinedAt, g.MemberSeenAt)
	}
}

// An instance updated from before this instance's own direct address existed
// starts out with none set and no manual override, so it falls back to
// LocalIPv4 until a browser or a person supplies one.
func TestGroupStateDirectURLStartsEmpty(t *testing.T) {
	db := OpenMem(t)
	bootAs(t, db, pairingMigration+5)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	g, err := New(db).GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	if g.DirectURL != "" || g.DirectURLManual {
		t.Fatalf("group state after migration = %+v, want no direct address yet", g)
	}
}
