package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// stage4Store opens a migrated on-disk SQLite store for the per-target monitoring
// unit tests (mirrors the budget/tamper internal tests' setup).
func stage4Store(t *testing.T) *store.Repo {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return store.New(db)
}

// TestCollectStatsSource pins the stage-3 carry-over fix: a per-target
// "offsite:<id>" source must survive normalisation (it addresses the off-site
// repo), not be clobbered to "local" as the old `source != "offsite"` compare did.
func TestCollectStatsSource(t *testing.T) {
	cases := []struct{ in, want string }{
		{"local", "local"},
		{"", "local"},
		{"garbage", "local"},
		{"offsite", "offsite"},
		{"offsite:abcd1234", "offsite:abcd1234"},
	}
	for _, c := range cases {
		if got := collectStatsSource(c.in); got != c.want {
			t.Errorf("collectStatsSource(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestOffsiteStatSourceAndLatchKey pins the per-target source + latch-key helpers:
// an empty (settings-synthesised) target samples under bare "offsite" so N=1 stays
// byte-identical, and each destination gets a distinct latch key.
func TestOffsiteStatSourceAndLatchKey(t *testing.T) {
	if got := offsiteStatSource(""); got != "offsite" {
		t.Fatalf("offsiteStatSource(\"\") = %q, want offsite", got)
	}
	if got := offsiteStatSource("t1"); got != "offsite:t1" {
		t.Fatalf("offsiteStatSource(t1) = %q, want offsite:t1", got)
	}
	if offsiteBudgetLatchKey("containers", "t1") == offsiteBudgetLatchKey("containers", "t2") {
		t.Fatal("per-target latch keys must differ by target id")
	}
	if offsiteBudgetLatchKey("containers", "t1") == offsiteBudgetLatchKey("vms", "t1") {
		t.Fatal("per-target latch keys must differ by domain")
	}
}

// TestCheckOffsiteBudgetForTargetPerTargetLatch: each destination alarms on its
// OWN size vs its OWN GrowthBudgetGB, latched independently — a second check of the
// same destination while still over stays silent, while a different destination
// over its budget still alarms.
func TestCheckOffsiteBudgetForTargetPerTargetLatch(t *testing.T) {
	st := stage4Store(t)
	// Per-target size samples: both destinations are 2 GiB.
	for _, id := range []string{"t1", "t2"} {
		if err := st.AddRepoStat(store.RepoStat{
			Domain: "containers", Source: offsiteStatSource(id), At: 1700000000,
			RawSize: 2 * 1024 * 1024 * 1024,
		}); err != nil {
			t.Fatal(err)
		}
	}
	ssh := &fakeHostSSH{}
	svc := &Service{
		cfg:               config.Config{AppKey: strings.Repeat("a", 64)},
		store:             st,
		ssh:               ssh,
		offsiteOverBudget: map[string]bool{},
	}
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
		t.Fatal(err)
	}

	t1 := store.OffsiteTarget{ID: "t1", Domain: "containers", GrowthBudgetGB: 1} // 2 GiB > 1 GiB
	t2 := store.OffsiteTarget{ID: "t2", Domain: "containers", GrowthBudgetGB: 1}
	tHi := store.OffsiteTarget{ID: "t1", Domain: "containers", GrowthBudgetGB: 10}

	svc.checkOffsiteBudgetForTarget(context.Background(), "containers", t1)
	svc.checkOffsiteBudgetForTarget(context.Background(), "containers", t1) // still over → no second alarm
	if len(ssh.runs) != 1 {
		t.Fatalf("t1 must alarm exactly once per crossing, got %d", len(ssh.runs))
	}
	svc.checkOffsiteBudgetForTarget(context.Background(), "containers", t2) // different destination → its own alarm
	if len(ssh.runs) != 2 {
		t.Fatalf("t2 over its own budget must alarm independently, got %d", len(ssh.runs))
	}

	// A destination with a high budget (under it) never alarms, even sharing a domain.
	ssh2 := &fakeHostSSH{}
	svc2 := &Service{cfg: config.Config{AppKey: strings.Repeat("a", 64)}, store: st, ssh: ssh2, offsiteOverBudget: map[string]bool{}}
	if err := svc2.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
		t.Fatal(err)
	}
	svc2.checkOffsiteBudgetForTarget(context.Background(), "containers", tHi)
	if len(ssh2.runs) != 0 {
		t.Fatalf("a destination under its budget must not alarm, got %d", len(ssh2.runs))
	}
}

// enableTarget inserts an enabled off-site target with a known id.
func enableTarget(t *testing.T, st *store.Repo, domain, id string) {
	t.Helper()
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		ID: id, Domain: domain, Name: id, Repo: "rest:http://host/" + id, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
}

// TestAggregateTamperWorstOf: N=1 (no target rows) delegates to LatestTamperTest;
// with two destinations the domain is protected only when BOTH are, unproven when
// any destination lacks a verdict, and its currency is the OLDEST verdict.
func TestAggregateTamperWorstOf(t *testing.T) {
	st := stage4Store(t)
	svc := &Service{cfg: config.Config{AppKey: strings.Repeat("a", 64)}, store: st}

	// N=1: no target rows, one domain-wide verdict → delegates byte-identically.
	if err := st.RecordTamperTest("containers", true, ""); err != nil {
		t.Fatal(err)
	}
	had, protected, at := svc.aggregateTamper("containers")
	if !had || !protected || at == 0 {
		t.Fatalf("N=1 aggregate = (had=%v protected=%v at=%d), want protected", had, protected, at)
	}

	// Two enabled destinations, both protected → domain protected.
	enableTarget(t, st, "vms", "t1")
	enableTarget(t, st, "vms", "t2")
	if err := st.RecordTamperTestForTarget("vms", "t1", true, ""); err != nil {
		t.Fatal(err)
	}
	t1, _, _ := st.LatestTamperTestForTarget("vms", "t1")
	if err := st.RecordTamperTestForTarget("vms", "t2", true, ""); err != nil {
		t.Fatal(err)
	}
	had, protected, at = svc.aggregateTamper("vms")
	if !had || !protected {
		t.Fatalf("both protected → (had=%v protected=%v), want protected", had, protected)
	}
	if at != t1.At {
		t.Fatalf("currency must be the OLDEST verdict (%d), got %d", t1.At, at)
	}

	// Flip t2 to unprotected → domain no longer protected (worst-of).
	if err := st.RecordTamperTestForTarget("vms", "t2", false, "accepted"); err != nil {
		t.Fatal(err)
	}
	if _, protected, _ = svc.aggregateTamper("vms"); protected {
		t.Fatal("one unprotected destination must make the domain UNprotected")
	}

	// A destination with NO verdict → no protected claim at all.
	enableTarget(t, st, "flash", "f1")
	enableTarget(t, st, "flash", "f2")
	if err := st.RecordTamperTestForTarget("flash", "f1", true, ""); err != nil {
		t.Fatal(err)
	}
	had, protected, at = svc.aggregateTamper("flash")
	if had || protected || at != 0 {
		t.Fatalf("an untested destination → no claim, got (had=%v protected=%v at=%d)", had, protected, at)
	}
}

// TestRunTamperTestPerTargetWorstOf: with two off-site destinations — one that
// refuses deletes (protected) and one that accepts them (unprotected) — the
// aggregate verdict is worst-of (testable but NOT protected), and each
// destination's verdict is recorded independently under its offsite_target_id.
func TestRunTamperTestPerTargetWorstOf(t *testing.T) {
	refuse := httptest.NewServer(deleteRecorder(http.StatusForbidden, new([]string))) // protected
	defer refuse.Close()
	accept := httptest.NewServer(deleteRecorder(http.StatusOK, new([]string))) // NOT protected
	defer accept.Close()

	st := stage4Store(t)
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{ID: "good", Domain: "containers", Name: "good", Repo: "rest:" + refuse.URL, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{ID: "bad", Domain: "containers", Name: "bad", Repo: "rest:" + accept.URL, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	svc := &Service{cfg: config.Config{AppKey: strings.Repeat("a", 64)}, store: st, ssh: &fakeHostSSH{}}

	v, err := svc.RunTamperTest(context.Background(), "containers")
	if err != nil {
		t.Fatalf("RunTamperTest: %v", err)
	}
	if !v.Testable || v.Protected {
		t.Fatalf("worst-of verdict must be testable + NOT protected, got %+v", v)
	}

	good, found, err := st.LatestTamperTestForTarget("containers", "good")
	if err != nil || !found || !good.Protected {
		t.Fatalf("the refusing destination must record a protected verdict, found=%v protected=%v err=%v", found, good.Protected, err)
	}
	bad, found, err := st.LatestTamperTestForTarget("containers", "bad")
	if err != nil || !found || bad.Protected {
		t.Fatalf("the accepting destination must record an UNprotected verdict, found=%v protected=%v err=%v", found, bad.Protected, err)
	}
}

// TestAggregateReplicationCurrencyWorstOf: N=1 delegates; with two destinations the
// currency is the OLDEST successful copy, and a never-replicated destination makes
// the domain not-current.
func TestAggregateReplicationCurrencyWorstOf(t *testing.T) {
	st := stage4Store(t)
	svc := &Service{cfg: config.Config{AppKey: strings.Repeat("a", 64)}, store: st}

	// N=1: no target rows, one domain-wide success.
	id, err := st.RecordOffsiteRun("containers", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishOffsiteRun(id, true, ""); err != nil {
		t.Fatal(err)
	}
	if at, ok := svc.aggregateReplicationCurrency("containers"); !ok || at != 5000 {
		t.Fatalf("N=1 currency = (at=%d ok=%v), want 5000/true", at, ok)
	}

	// Two destinations, both successful at different times → oldest wins.
	enableTarget(t, st, "vms", "t1")
	enableTarget(t, st, "vms", "t2")
	id1, _ := st.RecordOffsiteRunForTarget("vms", "t1", 1000)
	_ = st.FinishOffsiteRun(id1, true, "")
	id2, _ := st.RecordOffsiteRunForTarget("vms", "t2", 2000)
	_ = st.FinishOffsiteRun(id2, true, "")
	if at, ok := svc.aggregateReplicationCurrency("vms"); !ok || at != 1000 {
		t.Fatalf("worst-of currency = (at=%d ok=%v), want 1000/true (oldest)", at, ok)
	}

	// A never-replicated second destination → domain not current.
	enableTarget(t, st, "flash", "f1")
	enableTarget(t, st, "flash", "f2")
	idf, _ := st.RecordOffsiteRunForTarget("flash", "f1", 3000)
	_ = st.FinishOffsiteRun(idf, true, "")
	if at, ok := svc.aggregateReplicationCurrency("flash"); ok || at != 0 {
		t.Fatalf("a never-replicated destination → not current, got (at=%d ok=%v)", at, ok)
	}
}
