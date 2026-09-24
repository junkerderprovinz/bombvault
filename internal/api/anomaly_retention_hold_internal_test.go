package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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
