package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestWatchdogDecisionUsesDashboardPredicate pins the once-per-episode dedupe
// AND the predicate reuse: the decision is a pure function of the SAME
// rpoStatus inputs the dashboard's protection status renders, so the push and
// the chip can never disagree on "overdue".
func TestWatchdogDecisionUsesDashboardPredicate(t *testing.T) {
	const day = int64(86400)
	now := int64(1_800_000_000)

	cases := []struct {
		name       string
		lastSucc   int64
		period     int64
		enabled    bool
		state      store.WatchdogState
		haveState  bool
		wantNotify bool
		wantClear  bool
	}{
		// rpoStatus "ok"/"warn"/"never"/"off" are all non-events for the watchdog.
		{"fresh backup → quiet", now - day/2, day, true, store.WatchdogState{}, false, false, false},
		{"warn (1..2 periods) → quiet, not an episode yet", now - day - 1, day, true, store.WatchdogState{}, false, false, false},
		{"never ran → quiet (alerts regressions, not new setups)", 0, day, true, store.WatchdogState{}, false, false, false},
		{"disabled domain → quiet", now - 30*day, day, false, store.WatchdogState{}, false, false, false},
		{"no period (schedule off) → quiet", now - 30*day, 0, true, store.WatchdogState{}, false, false, false},

		// Overdue (> 2× period — rpoStatus's own threshold).
		{"overdue, no episode yet → notify", now - 3*day, day, true, store.WatchdogState{}, false, true, false},
		{"overdue, episode already notified → quiet", now - 3*day, day, true,
			store.WatchdogState{Domain: "containers", NotifiedAt: now - day, LastSuccessAt: now - 3*day}, true, false, false},
		{"overdue again after a NEWER success → new episode, notify", now - 3*day, day, true,
			store.WatchdogState{Domain: "containers", NotifiedAt: now - 10*day, LastSuccessAt: now - 12*day}, true, true, false},

		// Recovery: a recorded episode is cleared as soon as the domain is current.
		{"recovered with stale episode → clear state", now - day/2, day, true,
			store.WatchdogState{Domain: "containers", NotifiedAt: now - day, LastSuccessAt: now - 3*day}, true, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotNotify, gotClear := watchdogDecision(now, c.lastSucc, c.period, c.enabled, c.state, c.haveState)
			if gotNotify != c.wantNotify || gotClear != c.wantClear {
				t.Fatalf("watchdogDecision = (notify=%v, clear=%v), want (notify=%v, clear=%v)",
					gotNotify, gotClear, c.wantNotify, c.wantClear)
			}
			// Invariant: notify is only ever needed when rpoStatus itself says
			// overdue — the literal predicate-reuse contract.
			if gotNotify && rpoStatus(now, c.lastSucc, c.period, c.enabled && c.period > 0) != "overdue" {
				t.Fatal("watchdog must never notify when the dashboard predicate is not overdue")
			}
		})
	}
}

// watchdogTestService builds a Service over a real (temp) store with a
// counting webhook as the only notify channel, one seeded container target and
// one successful backup run, and the containers domain on a daily schedule.
// It returns the service, the store, the hit counter, and the (real) unix time
// around which the seeded success landed.
func watchdogTestService(t *testing.T) (*Service, *store.Repo, *int32, int64) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
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
	svc := &Service{cfg: config.Config{AppKey: strings.Repeat("a", 64)}, store: st}
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", WebhookEnabled: true, WebhookURL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersSchedule = "daily 03:00" // period = 1 day
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/host/user/appdata/plex"}})
	if err != nil {
		t.Fatal(err)
	}
	runID, err := st.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(runID, "success", "deadbeef", 1024, ""); err != nil {
		t.Fatal(err)
	}
	return svc, st, &hits, time.Now().Unix()
}

// TestWatchdogNotifiesOncePerEpisode drives the dedupe end-to-end over the real
// store + notify fan-out: overdue → ONE notification; still overdue the next
// day → no second one; a recovery clears the episode; going overdue again →
// notifies again.
func TestWatchdogNotifiesOncePerEpisode(t *testing.T) {
	svc, st, hits, seeded := watchdogTestService(t)
	ctx := context.Background()
	const day = int64(86400)

	// Day 3: the seeded success is 3 periods old → overdue → one notification.
	if err := svc.runWatchdogAt(ctx, seeded+3*day); err != nil {
		t.Fatalf("runWatchdogAt: %v", err)
	}
	if n := atomic.LoadInt32(hits); n != 1 {
		t.Fatalf("first overdue check must notify exactly once, got %d", n)
	}
	if _, found, err := st.GetWatchdogState("containers"); err != nil || !found {
		t.Fatalf("the notified episode must be recorded, found=%v err=%v", found, err)
	}

	// Day 4: still the same episode (same last success) → NO second notification.
	if err := svc.runWatchdogAt(ctx, seeded+4*day); err != nil {
		t.Fatalf("runWatchdogAt: %v", err)
	}
	if n := atomic.LoadInt32(hits); n != 1 {
		t.Fatalf("an already-notified episode must stay quiet, got %d notifications", n)
	}

	// Recovery: from a vantage where the backup is current again, the episode is
	// cleared (the success "arrived" relative to this check).
	if err := svc.runWatchdogAt(ctx, seeded+day/2); err != nil {
		t.Fatalf("runWatchdogAt: %v", err)
	}
	if _, found, err := st.GetWatchdogState("containers"); err != nil || found {
		t.Fatalf("a current domain must clear its episode, found=%v err=%v", found, err)
	}

	// A NEW overdue episode after the recovery notifies again.
	if err := svc.runWatchdogAt(ctx, seeded+3*day); err != nil {
		t.Fatalf("runWatchdogAt: %v", err)
	}
	if n := atomic.LoadInt32(hits); n != 2 {
		t.Fatalf("a new overdue episode must notify again, got %d total", n)
	}
}

// TestWatchdogMutedPolicyIsSilent pins the policy gate: with notifications off
// ("never"/unset) the watchdog neither sends nor records episodes — enabling
// notifications later must still surface a long-running overdue state.
func TestWatchdogMutedPolicyIsSilent(t *testing.T) {
	svc, st, hits, seeded := watchdogTestService(t)
	if err := svc.SetNotifyConfig(notify.Config{On: "never"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.runWatchdogAt(context.Background(), seeded+3*86400); err != nil {
		t.Fatalf("runWatchdogAt under a muted policy must be a silent no-op, got %v", err)
	}
	if n := atomic.LoadInt32(hits); n != 0 {
		t.Fatalf("a muted policy must send nothing, got %d", n)
	}
	if _, found, err := st.GetWatchdogState("containers"); err != nil || found {
		t.Fatalf("a muted policy must not record an episode, found=%v err=%v", found, err)
	}
}

// TestWatchdogCoversEverythingOnlyDomains: a domain backed up ONLY by the
// "Backup Everything" pass must still get an overdue alert.
//
// Issue #177's reporter runs exactly this configuration: every per-domain
// schedule off, one whole-server pass. The watchdog read the per-domain cadence
// alone, which is "off", which is period 0, which the decision treats as "no
// expectation" — so the dead-man's switch was disarmed for every domain on that
// server, and the only visible sign was the protection card saying "Not
// scheduled", which reads like a statement about scheduling rather than about
// alerting.
func TestWatchdogCoversEverythingOnlyDomains(t *testing.T) {
	svc, _, hits, seeded := watchdogTestService(t)

	settings, err := svc.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersSchedule = "off"         // no cadence of its own …
	settings.EverythingSchedule = "daily 03:00" // … but the pass backs it up nightly
	if err := svc.store.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	// Three days past a daily expectation is overdue by the same predicate the
	// dashboard chip uses.
	if err := svc.runWatchdogAt(context.Background(), seeded+3*86400); err != nil {
		t.Fatalf("runWatchdogAt: %v", err)
	}
	if n := atomic.LoadInt32(hits); n != 1 {
		t.Fatalf("a domain covered by the Everything pass must alert when it goes overdue, got %d notifications", n)
	}
}

// TestWatchdogStaysQuietWithNothingScheduled is the other half: no per-domain
// cadence and no pass either means there genuinely is no expectation, and the
// fix above must not invent one.
func TestWatchdogStaysQuietWithNothingScheduled(t *testing.T) {
	svc, _, hits, seeded := watchdogTestService(t)

	settings, err := svc.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersSchedule = "off"
	settings.EverythingSchedule = "off"
	if err := svc.store.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	if err := svc.runWatchdogAt(context.Background(), seeded+30*86400); err != nil {
		t.Fatalf("runWatchdogAt: %v", err)
	}
	if n := atomic.LoadInt32(hits); n != 0 {
		t.Fatalf("nothing is scheduled, so nothing is overdue, got %d notifications", n)
	}
}
