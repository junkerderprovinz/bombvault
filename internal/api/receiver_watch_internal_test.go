package api

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestReceiverDeadManDecision pins the once-per-stale-episode dedupe as a pure
// function (no store/clock/transport), mirroring the watchdog's decision test.
func TestReceiverDeadManDecision(t *testing.T) {
	const hour = int64(3600)
	now := int64(1_800_000_000)
	dead := 26 * hour

	cases := []struct {
		name      string
		newest    int64
		dead      int64
		state     store.ReceivedAlertState
		haveState bool
		wantAlert bool
		wantClear bool
	}{
		{"fresh source -> quiet", now - hour, dead, store.ReceivedAlertState{}, false, false, false},
		{"dead-man disabled -> quiet", now - 100*hour, 0, store.ReceivedAlertState{}, false, false, false},
		{"stale, no episode -> alert", now - 100*hour, dead, store.ReceivedAlertState{}, false, true, false},
		{"stale, same episode already alerted -> quiet", now - 100*hour, dead,
			store.ReceivedAlertState{BasedOn: now - 100*hour, NotifiedAt: now - hour}, true, false, false},
		{"stale again after a NEWER snapshot -> new episode, alert", now - 100*hour, dead,
			store.ReceivedAlertState{BasedOn: now - 500*hour, NotifiedAt: now - 400*hour}, true, true, false},
		{"recovered (fresh) with stale episode -> clear", now - hour, dead,
			store.ReceivedAlertState{BasedOn: now - 100*hour, NotifiedAt: now - hour}, true, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotAlert, gotClear := receiverDeadManDecision(now, c.newest, c.dead, c.state, c.haveState)
			if gotAlert != c.wantAlert || gotClear != c.wantClear {
				t.Fatalf("receiverDeadManDecision = (alert=%v, clear=%v), want (alert=%v, clear=%v)",
					gotAlert, gotClear, c.wantAlert, c.wantClear)
			}
		})
	}
}

// TestReceiverIntegrityShouldAlert pins the integrity transition debounce: alert
// only on the crossing into failure (never->fail, ok->fail), never while already
// failed, and never on success (which re-arms for the next failure).
func TestReceiverIntegrityShouldAlert(t *testing.T) {
	never := sql.NullBool{}
	ok := sql.NullBool{Bool: true, Valid: true}
	failed := sql.NullBool{Bool: false, Valid: true}

	cases := []struct {
		name  string
		prev  sql.NullBool
		newOK bool
		want  bool
	}{
		{"never -> fail: first breach alerts", never, false, true},
		{"ok -> fail: regression alerts", ok, false, true},
		{"fail -> fail: already alerted, quiet", failed, false, false},
		{"never -> ok: healthy, quiet", never, true, false},
		{"fail -> ok: recovery, quiet (and re-arms)", failed, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := receiverIntegrityShouldAlert(c.prev, c.newOK); got != c.want {
				t.Fatalf("receiverIntegrityShouldAlert(%+v, ok=%v) = %v, want %v", c.prev, c.newOK, got, c.want)
			}
		})
	}
}

// receiverWatchService builds a Service over a real (temp) store wired to the real
// restic engine, with a capturing webhook as the only notify channel (On=failure,
// so the OK:false alerts send). It returns the service, store, and a thread-safe
// accessor for the captured message bodies.
func receiverWatchService(t *testing.T, appKey string) (*Service, *store.Repo, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	svc := &Service{cfg: config.Config{AppKey: appKey}, store: st, engine: restic.Restic{Bin: "restic"}}
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", WebhookURL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	get := func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := make([]string, len(bodies))
		copy(out, bodies)
		return out
	}
	return svc, st, get
}

func countContaining(bodies []string, sub string) int {
	n := 0
	for _, b := range bodies {
		if strings.Contains(b, sub) {
			n++
		}
	}
	return n
}

// TestReceiverWatchDeadMansSwitch drives the dead-mans-switch end-to-end over the
// real store + notify fan-out against a real received repo: stale sources alert
// ONCE per episode, a fresh vantage stays quiet and clears the episodes, and going
// stale again re-alerts. Two sources (container:web, vm:db) exercise per-source
// grouping in the alert path.
func TestReceiverWatchDeadMansSwitch(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	const hour = int64(3600)
	appKey := strings.Repeat("ab", 32)
	sendingKey := strings.Repeat("cd", 32)
	repo := seedReceivedRepo(t, sendingKey) // snapshots for container:web x2, vm:db, all ~now

	svc, st, bodies := receiverWatchService(t, appKey)
	rr := makeReceivedRepo(t, appKey, sendingKey, repo, 0)
	rr.Name = "Off-site A"
	rr.DeadManHours = 26
	rr.CheckCadence = "off" // isolate the dead-mans-switch from integrity checks
	created, err := st.CreateReceivedRepo(rr)
	if err != nil {
		t.Fatal(err)
	}

	// Determine the actual newest snapshot time so "now" can be placed relative to
	// it deterministically.
	sources, err := svc.receiverDeadManSources(context.Background(), created)
	if err != nil {
		t.Fatalf("receiverDeadManSources: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("want 2 sources, got %d: %+v", len(sources), sources)
	}
	var newest int64
	for _, s := range sources {
		if s.newest > newest {
			newest = s.newest
		}
	}

	// Stale vantage: 100h after the newest snapshot, dead-man is 26h -> both sources
	// stale -> exactly two alerts.
	staleNow := newest + 100*hour
	if err := svc.runReceiverChecksAt(context.Background(), staleNow); err != nil {
		t.Fatalf("runReceiverChecksAt: %v", err)
	}
	if n := countContaining(bodies(), "No backup received from"); n != 2 {
		t.Fatalf("first stale sweep must alert both sources once, got %d", n)
	}

	// Same vantage again: episodes already recorded -> no new alerts.
	if err := svc.runReceiverChecksAt(context.Background(), staleNow); err != nil {
		t.Fatal(err)
	}
	if n := countContaining(bodies(), "No backup received from"); n != 2 {
		t.Fatalf("an already-alerted episode must stay quiet, got %d total", n)
	}

	// Fresh vantage (within the window): not stale -> no alert, and the episodes are
	// cleared (recovery), re-arming the switch.
	freshNow := newest + hour
	if err := svc.runReceiverChecksAt(context.Background(), freshNow); err != nil {
		t.Fatal(err)
	}
	if n := countContaining(bodies(), "No backup received from"); n != 2 {
		t.Fatalf("a fresh vantage must not alert, got %d total", n)
	}
	for _, s := range sources {
		if _, ok, _ := st.GetReceivedAlertState(created.ID, s.key); ok {
			t.Fatalf("recovery must clear the episode for %s", s.source)
		}
	}

	// Stale again after the recovery -> a brand-new episode -> re-alerts (two more).
	if err := svc.runReceiverChecksAt(context.Background(), staleNow); err != nil {
		t.Fatal(err)
	}
	if n := countContaining(bodies(), "No backup received from"); n != 4 {
		t.Fatalf("a new stale episode must re-alert both sources, got %d total", n)
	}
}

// TestReceiverWatchIntegrityAlert proves a failed independent check raises the
// integrity alert once (never->fail), and a still-failing check on the next due
// run does not re-fire (fail->fail). The failure is induced with a wrong stored
// sending key so the repo cannot be opened; dead-man is disabled so only the
// integrity path can speak.
func TestReceiverWatchIntegrityAlert(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	const day = int64(86400)
	appKey := strings.Repeat("ab", 32)
	sendingKey := strings.Repeat("cd", 32)
	repo := seedReceivedRepo(t, sendingKey)

	svc, st, bodies := receiverWatchService(t, appKey)
	// Wrong stored key -> receiverOpen fails -> the scheduled check is not-ok.
	rr := makeReceivedRepo(t, appKey, strings.Repeat("ef", 32), repo, 0)
	rr.Name = "Broken repo"
	rr.DeadManHours = 0 // disable the dead-mans-switch: only integrity may alert
	rr.CheckCadence = "daily 04:00"
	created, err := st.CreateReceivedRepo(rr)
	if err != nil {
		t.Fatal(err)
	}

	// First due run (never -> fail): exactly one integrity alert, and the verdict is
	// persisted as a failure.
	if err := svc.runReceiverChecksAt(context.Background(), 1_000_000); err != nil {
		t.Fatal(err)
	}
	if n := countContaining(bodies(), "Integrity check FAILED on Broken repo"); n != 1 {
		t.Fatalf("first failing check must alert once, got %d", n)
	}
	got, _, _ := st.GetReceivedRepo(created.ID)
	if !got.LastCheckOK.Valid || got.LastCheckOK.Bool {
		t.Fatalf("the failed verdict must persist: %+v", got.LastCheckOK)
	}

	// Next due run two days later (fail -> fail): the check runs again and still
	// fails, but the transition debounce keeps it quiet.
	if err := svc.runReceiverChecksAt(context.Background(), 1_000_000+2*day); err != nil {
		t.Fatal(err)
	}
	if n := countContaining(bodies(), "Integrity check FAILED"); n != 1 {
		t.Fatalf("an already-failing repo must not re-alert, got %d total", n)
	}
}

// TestReceiverWatchMutedPolicyStillPersists pins that with notifications off the
// watch sends nothing but STILL persists the check verdict (dashboard state), and
// still records dead-man episodes so enabling alerts later does not replay old
// staleness as new.
func TestReceiverWatchMutedPolicyStillPersists(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	appKey := strings.Repeat("ab", 32)
	sendingKey := strings.Repeat("cd", 32)
	repo := seedReceivedRepo(t, sendingKey)

	svc, st, bodies := receiverWatchService(t, appKey)
	if err := svc.SetNotifyConfig(notify.Config{On: "never"}); err != nil {
		t.Fatal(err)
	}
	rr := makeReceivedRepo(t, appKey, sendingKey, repo, 0)
	rr.CheckCadence = "daily 04:00"
	rr.DeadManHours = 26
	created, err := st.CreateReceivedRepo(rr)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.runReceiverChecksAt(context.Background(), 2_000_000_000); err != nil {
		t.Fatal(err)
	}
	if len(bodies()) != 0 {
		t.Fatalf("a muted policy must send nothing, got %d", len(bodies()))
	}
	// The integrity check still ran and persisted a verdict (the good repo passes).
	got, _, _ := st.GetReceivedRepo(created.ID)
	if !got.LastCheckOK.Valid {
		t.Fatal("the check must persist a verdict even under a muted policy")
	}
}
