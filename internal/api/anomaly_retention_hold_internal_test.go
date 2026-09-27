package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Switching the hold off lets the next retention pass forget again, so nothing
// the reader sees may keep promising that old backups are being kept.
func TestHeldFlagFollowsTheRetentionHoldSwitch(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 11, 40*gib)
	f.run(t, id, "backup", f.now-3600, 20*mib)
	f.pass(t)
	if row := onlyRow(t, f.openRows(t)); row.Metric != metricSourceBytesShrink {
		t.Fatalf("want the collapse raised, got %s", row.Metric)
	}

	held := func(stage string, want bool) {
		t.Helper()
		page, err := f.svc.ListAnomalies(context.Background(), store.AnomalyFilter{})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Anomalies) != 1 || page.Anomalies[0].RetentionHeld != want {
			t.Fatalf("%s: the finding reports retentionHeld %v, want %v", stage, !want, want)
		}
		count := 0
		if want {
			count = 1
		}
		if got := f.e.summary().RetentionHeld; got != count {
			t.Fatalf("%s: the summary counts %d held item(s), want %d", stage, got, count)
		}
		items := f.e.itemViews()
		if len(items) != 1 || items[0].RetentionHeld != want {
			t.Fatalf("%s: the item badge reports %v, want %v", stage, !want, want)
		}
	}
	held("with both switches on", true)

	if _, err := f.st.MutateSettings(func(s *store.Settings) error {
		s.AnomalyRetentionHold = false
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f.e.MarkAllDirty()
	f.pass(t)
	held("with the hold switched off", false)
}

// A repository outage can open more criticals than one page of findings holds.
// The rows that pause deleting old backups are read without that limit, or a
// retention pass would delete the snapshots one of them was keeping.
func TestHoldSurvivesAFloodOfOpenCriticals(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")

	// A rewrite is an event: its last_seen_at never moves again, so it sorts
	// below every critical a later pass refreshes.
	rows := []store.Anomaly{openCritical("rewrite", metricNewDataRewrite, id, f.now-2*86400)}
	rows[0].TargetID, rows[0].Domain = id, anomalyDomainContainer
	for i := range anomalyOpenRowLimit {
		scope := "gone-" + strconv.Itoa(i)
		rows = append(rows, openCritical(scope, metricFailureStreak, scope, f.now))
	}
	if _, err := f.st.ApplyAnomalyChanges(store.AnomalyChanges{Insert: rows, Now: f.now}); err != nil {
		t.Fatal(err)
	}

	tags, err := f.e.HeldIdentityTags()
	if err != nil {
		t.Fatal(err)
	}
	if !tags.holds("container:nextcloud") {
		t.Fatalf("the hold was lost behind %d other criticals: %v", anomalyOpenRowLimit, tags.names())
	}
	f.e.refresh()
	if n := f.e.summary().RetentionHeld; n != 1 {
		t.Fatalf("the summary counts %d held item(s), want 1", n)
	}
	items := f.e.itemViews()
	if len(items) != 1 || !items[0].RetentionHeld {
		t.Fatalf("the item badge lost the hold: %+v", items)
	}
}

// A repository that cannot be listed is a different problem from a hold. The
// run record names both, or the operator acknowledges findings while the real
// reason nothing was deleted stays invisible.
func TestRepoWideRetentionNamesTheListingFailureAndTheHold(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 12, 40*gib)
	f.run(t, id, "backup", f.now-60, 20*mib)
	f.pass(t)
	if len(f.openRows(t)) != 1 {
		t.Fatalf("want the collapse raised, got %+v", f.openRows(t))
	}
	f.svc.engine = &previewEngine{snapsErr: errors.New("repository is unreachable")}

	paused, err := f.svc.applyRetentionPerIdentity(context.Background(), "/repo",
		restic.RetentionPolicy{KeepLast: 5}, restic.Mode{})

	if len(paused) == 0 {
		t.Fatal("the held item was not reported as paused")
	}
	if err == nil || !strings.Contains(err.Error(), "unreachable") || !strings.Contains(err.Error(), "paused") {
		t.Fatalf("error = %v, want the listing failure next to the hold", err)
	}
}

// openCritical is one open critical finding on its own scope, so a test can
// fill the findings table with rows that differ.
func openCritical(id, metric, scopeID string, lastSeenAt int64) store.Anomaly {
	return store.Anomaly{
		ID:          id,
		Fingerprint: store.AnomalyFingerprint(anomalyDetectors[metric], anomalyScopeItem, scopeID, metric),
		Detector:    anomalyDetectors[metric], Metric: metric,
		Severity: "critical", State: "open",
		ScopeKind: anomalyScopeItem, ScopeID: scopeID,
		FirstSeenAt: lastSeenAt, LastSeenAt: lastSeenAt, Occurrences: 1,
	}
}

// A guard that cannot look must not wave the forget through: a retention pass
// whose anomaly check fails deletes nothing and says so.
func TestHoldCheckErrorSkipsTheForgetAndAlerts(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "plex")
	f.steadySeries(t, id, "backup", 11, 40*gib)
	forget := &forgetTrackingEngine{}
	f.svc.engine = forget

	var sent atomic.Int64
	wh := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		sent.Add(1)
	}))
	defer wh.Close()
	// An unset policy, the state of an install where a channel was configured
	// and the policy never touched.
	if err := f.svc.SetNotifyConfig(notify.Config{WebhookEnabled: true, WebhookURL: wh.URL, WebhookFormat: "generic"}); err != nil {
		t.Fatal(err)
	}
	f.e.items = func(store.Settings) (map[string]anomalyItemRef, error) {
		return nil, errors.New("the item tables could not be read")
	}

	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.RetentionKeepLast = 5
	f.svc.applyRetention(context.Background(), "/repo", settings, restic.Mode{},
		tagIdentity("container:plex"), "containers", anomalyScope{Kind: anomalyScopeItem, ID: id})

	if forget.forgetCalls() != 0 {
		t.Fatalf("a failed hold check must not forget anything, got %d call(s)", forget.forgetCalls())
	}
	if sent.Load() != 1 {
		t.Fatalf("want one alert about the failed check, got %d", sent.Load())
	}

	f.e.items = func(s store.Settings) (map[string]anomalyItemRef, error) { return f.svc.readAnomalyItems(s) }
	f.e.refresh()
	if got := f.e.summary().EvalErrors; got != 1 {
		t.Fatalf("evalErrors = %d, want the failed check counted once", got)
	}
}

// A container's dumps are a series of their own with a tag of their own. A
// database that was emptied keeps its dumps, and the volume backup beside it
// prunes as usual; the manual prune spares the same tag the live pass does.
func TestDumpCollapseHoldsOnlyTheDumpSeries(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "pg")
	f.steadySeries(t, id, "backup", 11, 40*gib)
	f.steadySeries(t, id, "dbdump", 11, 300*mib)
	f.run(t, id, "dbdump", f.now-3600, 100<<10)
	forget := &forgetTrackingEngine{}
	f.svc.engine = forget
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.RetentionKeepLast = 5

	f.svc.forgetDBDumpSeries(context.Background(), "/repo", settings, restic.Mode{}, "pg", id)
	if forget.forgetCalls() != 0 {
		t.Fatalf("the emptied database's dumps were forgotten (%d call(s))", forget.forgetCalls())
	}
	f.svc.applyRetention(context.Background(), "/repo", settings, restic.Mode{},
		f.svc.containerIdentity("pg"), "containers", anomalyScope{Kind: anomalyScopeItem, ID: id})
	if forget.forgetCalls() != 1 {
		t.Fatalf("the volume backup made %d forget call(s), want its own one", forget.forgetCalls())
	}

	held, err := f.e.HeldIdentityTags()
	if err != nil {
		t.Fatal(err)
	}
	if !held.holds("dbdump:pg") || held.holds("container:pg") {
		t.Fatalf("held = %v, want the dump series and not the container", held.names())
	}
}
