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

// stage4Store opens a migrated SQLite store in a temp directory.
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

// A target without an id, built from the settings, samples under the bare
// "offsite" source. Each target gets its own latch key.
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

// Each target alarms on its own size against its own GrowthBudgetGB, and the
// alarm latches per target.
func TestCheckOffsiteBudgetForTargetPerTargetLatch(t *testing.T) {
	st := stage4Store(t)
	// Both targets hold 2 GiB.
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

	t1 := store.OffsiteTarget{ID: "t1", Domain: "containers", GrowthBudgetGB: 1}
	t2 := store.OffsiteTarget{ID: "t2", Domain: "containers", GrowthBudgetGB: 1}
	tHi := store.OffsiteTarget{ID: "t1", Domain: "containers", GrowthBudgetGB: 10}

	svc.checkOffsiteBudgetForTarget(context.Background(), "containers", t1)
	svc.checkOffsiteBudgetForTarget(context.Background(), "containers", t1)
	if len(ssh.runs) != 1 {
		t.Fatalf("t1 must alarm exactly once per crossing, got %d", len(ssh.runs))
	}
	svc.checkOffsiteBudgetForTarget(context.Background(), "containers", t2)
	if len(ssh.runs) != 2 {
		t.Fatalf("t2 over its own budget must alarm independently, got %d", len(ssh.runs))
	}

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

// Without target rows the domain-wide verdict applies. With several targets
// the domain is protected only when all are, has no verdict while any target
// lacks one, and is dated by the oldest verdict.
func TestAggregateTamperWorstOf(t *testing.T) {
	st := stage4Store(t)
	svc := &Service{cfg: config.Config{AppKey: strings.Repeat("a", 64)}, store: st}

	if err := st.RecordTamperTest("containers", true, ""); err != nil {
		t.Fatal(err)
	}
	had, protected, at := svc.aggregateTamper("containers")
	if !had || !protected || at == 0 {
		t.Fatalf("N=1 aggregate = (had=%v protected=%v at=%d), want protected", had, protected, at)
	}

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

	if err := st.RecordTamperTestForTarget("vms", "t2", false, "accepted"); err != nil {
		t.Fatal(err)
	}
	if _, protected, _ = svc.aggregateTamper("vms"); protected {
		t.Fatal("one unprotected destination must make the domain UNprotected")
	}

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

// One target refuses deletes and one accepts them. The domain verdict is
// testable but not protected, and each target's verdict is recorded under its
// own id.
func TestRunTamperTestPerTargetWorstOf(t *testing.T) {
	refuse := httptest.NewServer(deleteRecorder(http.StatusForbidden, new([]string)))
	defer refuse.Close()
	accept := httptest.NewServer(deleteRecorder(http.StatusOK, new([]string)))
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

// With several targets the domain is as current as its oldest successful copy,
// and a target that never replicated makes it not current.
func TestAggregateReplicationCurrencyWorstOf(t *testing.T) {
	st := stage4Store(t)
	svc := &Service{cfg: config.Config{AppKey: strings.Repeat("a", 64)}, store: st}

	// No target rows: the domain-wide run counts.
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

	enableTarget(t, st, "vms", "t1")
	enableTarget(t, st, "vms", "t2")
	id1, _ := st.RecordOffsiteRunForTarget("vms", "t1", 1000)
	_ = st.FinishOffsiteRun(id1, true, "")
	id2, _ := st.RecordOffsiteRunForTarget("vms", "t2", 2000)
	_ = st.FinishOffsiteRun(id2, true, "")
	if at, ok := svc.aggregateReplicationCurrency("vms"); !ok || at != 1000 {
		t.Fatalf("worst-of currency = (at=%d ok=%v), want 1000/true (oldest)", at, ok)
	}

	enableTarget(t, st, "flash", "f1")
	enableTarget(t, st, "flash", "f2")
	idf, _ := st.RecordOffsiteRunForTarget("flash", "f1", 3000)
	_ = st.FinishOffsiteRun(idf, true, "")
	if at, ok := svc.aggregateReplicationCurrency("flash"); ok || at != 0 {
		t.Fatalf("a never-replicated destination → not current, got (at=%d ok=%v)", at, ok)
	}
}
