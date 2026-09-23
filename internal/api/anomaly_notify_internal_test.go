package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAnomalyNotifyTextSingleAndGrouped(t *testing.T) {
	collapse := AnomalyView{
		Metric: metricSourceBytesShrink, Severity: "critical", ScopeKind: anomalyScopeItem,
		Domain: "container", Name: "nextcloud",
		Observed: 20 << 20, Expected: 40 << 30,
		Details:       map[string]any{"collapse": true, "lastGoodAt": float64(1_800_000_000)},
		RetentionHeld: true,
	}

	title, body := anomalyNotifyText([]AnomalyView{collapse})
	if title != "BombVault: anomaly in the backup of nextcloud" {
		t.Fatalf("title = %q", title)
	}
	for _, want := range []string{
		"nextcloud is almost empty",
		"Deleting old backups of nextcloud is paused until you acknowledge this or mark it as expected.",
		"Last good backup: ",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body %q is missing %q", body, want)
		}
	}

	views := make([]AnomalyView, 0, 12)
	for range 12 {
		views = append(views, collapse)
	}
	title, body = anomalyNotifyText(views)
	if title != "BombVault: 12 anomalies" {
		t.Fatalf("title = %q", title)
	}
	if lines := strings.Split(body, "\n"); len(lines) != anomalyNotifyLines+1 {
		t.Fatalf("want %d lines plus the count, got %d", anomalyNotifyLines, len(lines))
	}
	if !strings.HasSuffix(body, "and 2 more") {
		t.Fatalf("body has to count the rest, got %q", body)
	}
}

// The singleton domains have no item name of their own, and a message that
// called them differently from the backup notification would read as a
// different thing entirely.
func TestAnomalyNotifyNamesTheSingletonDomains(t *testing.T) {
	for domain, want := range map[string]string{
		"flash":  "Unraid flash",
		"config": "BombVault configuration",
	} {
		title, _ := anomalyNotifyText([]AnomalyView{{
			Metric: metricSourceBytesShrink, Severity: "critical",
			ScopeKind: anomalyScopeItem, Domain: domain,
		}})
		if !strings.Contains(title, want) {
			t.Fatalf("%s: title = %q, want it to name %q", domain, title, want)
		}
	}
	title, _ := anomalyNotifyText([]AnomalyView{{
		Metric: metricDumpBytesShrink, Severity: "critical",
		ScopeKind: anomalyScopeDump, Domain: "container", Name: "nextcloud",
	}})
	if !strings.Contains(title, "database dump of nextcloud") {
		t.Fatalf("title = %q", title)
	}
}

// webhookRecorder collects what the notify package delivered, so a test can
// read the messages instead of the calls that produced them.
type webhookRecorder struct {
	srv *httptest.Server
	mu  sync.Mutex
	got []string
}

func newWebhookRecorder(t *testing.T) *webhookRecorder {
	t.Helper()
	w := &webhookRecorder{}
	w.srv = httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload struct{ Text, Content, Message string }
		_ = json.Unmarshal(body, &payload)
		w.mu.Lock()
		w.got = append(w.got, payload.Text+payload.Content+payload.Message+string(body))
		w.mu.Unlock()
	}))
	t.Cleanup(w.srv.Close)
	return w
}

func (w *webhookRecorder) messages() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.got...)
}

func (w *webhookRecorder) config(on string) notify.Config {
	return notify.Config{On: on, WebhookEnabled: true, WebhookURL: w.srv.URL, WebhookFormat: "generic"}
}

func TestAnomalyNotificationsGateAndDedupe(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 11, 40*gib)
	f.run(t, id, "backup", f.now-60, 20*mib)
	wh := newWebhookRecorder(t)
	// An unset policy with a channel configured: the #195 shape, where a
	// hand-written "not chosen yet means off" gate makes every critical silent.
	if err := f.svc.SetNotifyConfig(wh.config("")); err != nil {
		t.Fatal(err)
	}

	f.pass(t)
	if got := wh.messages(); len(got) != 1 || !strings.Contains(got[0], "nextcloud") {
		t.Fatalf("want one message about the collapse, got %v", got)
	}

	f.pass(t)
	if got := wh.messages(); len(got) != 1 {
		t.Fatalf("a finding is sent once at its severity, got %d message(s)", len(got))
	}
}

// A warning stays below the default minimum until the item asks for it, and an
// item switched off hears nothing at all.
func TestAnomalyNotificationsFollowTheMinimum(t *testing.T) {
	for name, tc := range map[string]struct {
		itemMin string
		want    int
	}{
		"the item follows the global minimum": {"", 0},
		"the item asks for warnings":          {"warning", 1},
		"the item is switched off":            {"off", 0},
	} {
		t.Run(name, func(t *testing.T) {
			f := newEngineFixture(t)
			id := f.container(t, "nextcloud")
			for i := 12; i >= 1; i-- {
				f.run(t, id, "backup", f.now-int64(i)*86400, 40*gib, withMS(10*60*1000))
			}
			f.run(t, id, "backup", f.now-60, 40*gib, withMS(40*60*1000))
			wh := newWebhookRecorder(t)
			if err := f.svc.SetNotifyConfig(wh.config("always")); err != nil {
				t.Fatal(err)
			}
			if tc.itemMin != "" {
				if err := f.st.SetItemPrefs(id, store.ItemPrefs{NotifyMin: tc.itemMin}); err != nil {
					t.Fatal(err)
				}
			}

			f.pass(t)
			if row := onlyRow(t, f.openRows(t)); row.Severity != "warning" {
				t.Fatalf("this fixture is about a warning, got %s/%s", row.Metric, row.Severity)
			}
			if got := wh.messages(); len(got) != tc.want {
				t.Fatalf("want %d message(s), got %v", tc.want, got)
			}
		})
	}
}

// A failed backup already sends its own message, so the finding about the
// streak must not send a second one.
func TestAnomalyNotificationsStayQuietForFailures(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	for i := 6; i >= 1; i-- {
		failedRun(t, f, id, f.now-int64(i)*86400)
	}
	wh := newWebhookRecorder(t)
	if err := f.svc.SetNotifyConfig(wh.config("always")); err != nil {
		t.Fatal(err)
	}

	f.pass(t)
	if row := onlyRow(t, f.openRows(t)); row.Metric != metricFailureStreak || row.Severity != "critical" {
		t.Fatalf("this fixture is about a critical failure streak, got %s/%s", row.Metric, row.Severity)
	}
	if got := wh.messages(); len(got) != 0 {
		t.Fatalf("a failure streak has its own message already, got %v", got)
	}
}

// With notifications switched off nothing is stamped, so switching a channel
// on later still delivers what is open.
func TestAnomalyNotificationsKeepWhatWasNeverSent(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 11, 40*gib)
	f.run(t, id, "backup", f.now-60, 20*mib)
	wh := newWebhookRecorder(t)
	if err := f.svc.SetNotifyConfig(wh.config("never")); err != nil {
		t.Fatal(err)
	}

	f.pass(t)
	if got := wh.messages(); len(got) != 0 {
		t.Fatalf("an explicit never sends nothing, got %v", got)
	}

	if err := f.svc.SetNotifyConfig(wh.config("failure")); err != nil {
		t.Fatal(err)
	}
	f.e.MarkAllDirty()
	f.pass(t)
	if got := wh.messages(); len(got) != 1 {
		t.Fatalf("the open finding is delivered once the channel is on, got %v", got)
	}
}

// An event is about one run, and a message about a run days ago would be
// nothing the reader can act on.
func TestAnomalyEventNotificationsExpire(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	row := store.Anomaly{
		Fingerprint: "new_data|item|" + id + "|new_data_rewrite",
		Detector:    detectorNewData, Metric: metricNewDataRewrite, Severity: "critical",
		ScopeKind: anomalyScopeItem, ScopeID: id, TargetID: id, Domain: "container",
		LastRunAt: f.now - 2*anomalyEventMaxAge, FirstSeenAt: f.now, LastSeenAt: f.now,
	}
	if _, err := f.st.ApplyAnomalyChanges(store.AnomalyChanges{Now: f.now, Insert: []store.Anomaly{row}}); err != nil {
		t.Fatal(err)
	}
	wh := newWebhookRecorder(t)
	if err := f.svc.SetNotifyConfig(wh.config("always")); err != nil {
		t.Fatal(err)
	}

	if err := f.e.sendNotifications(context.Background(), mustEngineSettings(t, f)); err != nil {
		t.Fatal(err)
	}
	if got := wh.messages(); len(got) != 0 {
		t.Fatalf("a rewrite from two days ago is not tonight's news, got %v", got)
	}
}

// failedRun records one failed backup of the series.
func failedRun(t *testing.T, f *engineFixture, targetID string, at int64) {
	t.Helper()
	id, err := f.st.StartRun(targetID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.FinishRun(id, "failed", "", 0, "restic exited with 1"); err != nil {
		t.Fatal(err)
	}
	f.backdate(t, id, at)
}

func mustEngineSettings(t *testing.T, f *engineFixture) store.Settings {
	t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	return settings
}

// The page says when a finding reached nobody, so the summary has to know
// whether a message would go anywhere at all.
func TestSummarySaysWhenNothingIsPushed(t *testing.T) {
	f := newEngineFixture(t)
	f.pass(t)
	if !f.e.summary().NotifyMuted {
		t.Fatal("with no channel configured nothing is sent")
	}

	wh := newWebhookRecorder(t)
	if err := f.svc.SetNotifyConfig(wh.config("never")); err != nil {
		t.Fatal(err)
	}
	f.e.refresh()
	if !f.e.summary().NotifyMuted {
		t.Fatal("an explicit never sends nothing either")
	}

	if err := f.svc.SetNotifyConfig(wh.config("always")); err != nil {
		t.Fatal(err)
	}
	f.e.refresh()
	if f.e.summary().NotifyMuted {
		t.Fatal("a configured channel is not muted")
	}
}
