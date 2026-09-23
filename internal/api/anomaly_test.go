package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The findings are served over the same envelope every other route uses, and
// the handlers do nothing but parse and wrap: the numbers come from the
// service, the sentence is built in the browser.

const anomalyDay = int64(86400)

// seedAnomaly writes one open finding with the fingerprint its scope and metric
// give it, and returns its id.
func seedAnomaly(t *testing.T, st *store.Repo, a store.Anomaly) string {
	t.Helper()
	a.Fingerprint = store.AnomalyFingerprint(a.Detector, a.ScopeKind, a.ScopeID, a.Metric)
	if a.FirstSeenAt == 0 {
		a.FirstSeenAt = a.LastSeenAt
	}
	if _, err := st.ApplyAnomalyChanges(store.AnomalyChanges{Insert: []store.Anomaly{a}, Now: a.LastSeenAt}); err != nil {
		t.Fatalf("seed finding %s: %v", a.ID, err)
	}
	return a.ID
}

// anomalyIDs reads the ids out of a listing, in the order it served them.
func anomalyIDs(t *testing.T, m map[string]any) []string {
	t.Helper()
	rows, ok := m["anomalies"].([]any)
	if !ok {
		t.Fatalf("no anomalies array in %v", m)
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.(map[string]any)["id"].(string))
	}
	return out
}

// startAnomalyEngine starts the worker and stops it with the test, so a test
// about the served summary sees the cache the way a running server fills it.
func startAnomalyEngine(t *testing.T, svc *api.Service) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	svc.StartAnomalyEngine(ctx)
}

func TestAnomaliesEmptyStore(t *testing.T) {
	h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})

	w, m := doJSON(t, h, http.MethodGet, "/api/anomalies", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("list: status %d body %s", w.Code, w.Body.String())
	}
	if rows, ok := m["anomalies"].([]any); !ok || len(rows) != 0 {
		t.Fatalf("an empty store has to answer with an empty array, got %s", w.Body.String())
	}
	if m["nextCursor"] != "" {
		t.Fatalf("nextCursor = %v, want the empty string", m["nextCursor"])
	}

	w, m = doJSON(t, h, http.MethodGet, "/api/anomalies/summary", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("summary: status %d body %s", w.Code, w.Body.String())
	}
	summary, ok := m["summary"].(map[string]any)
	if !ok {
		t.Fatalf("no summary in %s", w.Body.String())
	}
	if summary["ready"] != false {
		t.Fatalf("ready = %v before the first pass, want false", summary["ready"])
	}
	if _, has := summary["generation"]; !has {
		t.Fatalf("the summary has to carry a generation, got %s", w.Body.String())
	}
	open := summary["open"].(map[string]any)
	for _, severity := range []string{"critical", "warning", "info"} {
		if open[severity] != float64(0) {
			t.Fatalf("open %s = %v, want 0", severity, open[severity])
		}
	}
}

// The counts come from the table at startup, so a critical raised before the
// last restart is on the page immediately and not only after the first pass.
func TestSummaryShowsPersistedRowsBeforeTheFirstPass(t *testing.T) {
	h, st, svc := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	now := time.Now().Unix()
	seedAnomaly(t, st, store.Anomaly{
		ID: "a1", Detector: "source", Metric: "source_bytes_shrink", Severity: "critical",
		ScopeKind: "item", ScopeID: "t1", TargetID: "t1", Domain: "container",
		LastSeenAt: now,
	})

	startAnomalyEngine(t, svc)

	_, m := doJSON(t, h, http.MethodGet, "/api/anomalies/summary", "")
	summary := m["summary"].(map[string]any)
	if summary["ready"] != false {
		t.Fatalf("ready = %v before the first pass, want false", summary["ready"])
	}
	if open := summary["open"].(map[string]any); open["critical"] != float64(1) {
		t.Fatalf("open critical = %v, want the persisted row", open["critical"])
	}
	if summary["retentionHeld"] != float64(1) {
		t.Fatalf("retentionHeld = %v, want the persisted row", summary["retentionHeld"])
	}
}

func TestAnomaliesListFiltersAndBadFilter(t *testing.T) {
	h, st := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	now := time.Now().Unix()

	seedAnomaly(t, st, store.Anomaly{
		ID: "open-critical", Detector: "source", Metric: "source_bytes_shrink", Severity: "critical",
		ScopeKind: "item", ScopeID: "t1", TargetID: "t1", Domain: "container", LastSeenAt: now - 10,
	})
	seedAnomaly(t, st, store.Anomaly{
		ID: "open-warning", Detector: "duration", Metric: "duration_slower", Severity: "warning",
		ScopeKind: "item", ScopeID: "t1", TargetID: "t1", Domain: "container", LastSeenAt: now,
	})
	seedAnomaly(t, st, store.Anomaly{
		ID: "open-dump", Detector: "source", Metric: "dump_bytes_shrink", Severity: "warning",
		ScopeKind: "dump", ScopeID: "t1", TargetID: "t1", Domain: "container", LastSeenAt: now,
	})
	seedAnomaly(t, st, store.Anomaly{
		ID: "open-zfs", Detector: "source", Metric: "source_bytes_shrink", Severity: "warning",
		ScopeKind: "zfsds", ScopeID: "tank/data", TargetID: "t1", Domain: "zfs", LastSeenAt: now,
	})
	seedAnomaly(t, st, store.Anomaly{
		ID: "old-closed", Detector: "new_data", Metric: "new_data", Severity: "info",
		ScopeKind: "item", ScopeID: "t2", TargetID: "t2", Domain: "vm", LastSeenAt: now - 60*anomalyDay,
	})
	if _, err := st.AcknowledgeAnomalies([]string{"old-closed"}, "seen", now-60*anomalyDay); err != nil {
		t.Fatalf("close the old row: %v", err)
	}

	// The default listing is the open rows, critical first, and it is never
	// narrowed by time.
	_, m := doJSON(t, h, http.MethodGet, "/api/anomalies", "")
	got := anomalyIDs(t, m)
	if len(got) != 4 || got[0] != "open-critical" {
		t.Fatalf("open listing = %v, want the four open rows with the critical first", got)
	}

	_, m = doJSON(t, h, http.MethodGet, "/api/anomalies?state=closed", "")
	if got := anomalyIDs(t, m); len(got) != 0 {
		t.Fatalf("closed listing = %v, want nothing older than thirty days", got)
	}
	_, m = doJSON(t, h, http.MethodGet, "/api/anomalies?state=closed&since=0", "")
	if got := anomalyIDs(t, m); len(got) != 1 || got[0] != "old-closed" {
		t.Fatalf("closed listing without a time limit = %v, want the old row", got)
	}

	_, m = doJSON(t, h, http.MethodGet, "/api/anomalies?severity=critical", "")
	if got := anomalyIDs(t, m); len(got) != 1 || got[0] != "open-critical" {
		t.Fatalf("severity filter = %v", got)
	}

	// An item's scope covers every series that belongs to it.
	_, m = doJSON(t, h, http.MethodGet, "/api/anomalies?scope=item:t1", "")
	got = anomalyIDs(t, m)
	if len(got) != 4 {
		t.Fatalf("scope=item:t1 = %v, want the item, dump and dataset rows", got)
	}

	_, m = doJSON(t, h, http.MethodGet, "/api/anomalies?domain=zfs", "")
	if got := anomalyIDs(t, m); len(got) != 1 || got[0] != "open-zfs" {
		t.Fatalf("domain filter = %v", got)
	}

	_, m = doJSON(t, h, http.MethodGet, "/api/anomalies?detector=duration", "")
	if got := anomalyIDs(t, m); len(got) != 1 || got[0] != "open-warning" {
		t.Fatalf("detector filter = %v", got)
	}

	for _, query := range []string{"severity=bogus", "state=bogus", "detector=bogus", "domain=bogus", "scope=bogus", "limit=900"} {
		w, m := doJSON(t, h, http.MethodGet, "/api/anomalies?"+query, "")
		if w.Code != http.StatusOK || m["ok"] != false || m["code"] != "bad-filter" {
			t.Fatalf("%s: status %d body %s, want a bad-filter refusal", query, w.Code, w.Body.String())
		}
	}

	w, m := doJSON(t, h, http.MethodGet, "/api/anomalies/open-critical", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("read one: status %d body %s", w.Code, w.Body.String())
	}
	if row := m["anomaly"].(map[string]any); row["metric"] != "source_bytes_shrink" {
		t.Fatalf("read one = %v", row)
	}
	_, m = doJSON(t, h, http.MethodGet, "/api/anomalies/nope", "")
	if m["ok"] != false || m["code"] != "not-found" {
		t.Fatalf("unknown id = %v, want a not-found refusal", m)
	}
}

func TestAnomaliesAcknowledgeAndExpected(t *testing.T) {
	h, st := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	now := time.Now().Unix()
	target, err := st.UpsertTarget(store.Target{ContainerName: "plex", IncludeInSchedule: true})
	if err != nil {
		t.Fatal(err)
	}
	seedAnomaly(t, st, store.Anomaly{
		ID: "shrink", Detector: "source", Metric: "source_bytes_shrink", Severity: "critical",
		ScopeKind: "item", ScopeID: target.ID, TargetID: target.ID, Domain: "container",
		Observed: 400, Expected: 40000, LastRunAt: now - 60, LastSeenAt: now,
	})
	seedAnomaly(t, st, store.Anomaly{
		ID: "streak", Detector: "reliability", Metric: "failure_streak", Severity: "warning",
		ScopeKind: "item", ScopeID: target.ID, TargetID: target.ID, Domain: "container", LastSeenAt: now,
	})

	w, m := doJSON(t, h, http.MethodPost, "/api/anomalies/acknowledge",
		`{"ids":["shrink","shrink","nope"],"note":"restored from yesterday"}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("acknowledge: status %d body %s", w.Code, w.Body.String())
	}
	if m["changed"] != float64(1) || m["skipped"] != float64(1) || m["released"] != float64(1) {
		t.Fatalf("acknowledge = %v, want one changed, one skipped and one hold released", m)
	}

	// A failed backup says what it is; there is nothing to call normal about it.
	_, m = doJSON(t, h, http.MethodPost, "/api/anomalies/expected", `{"ids":["streak"]}`)
	if m["changed"] != float64(0) || m["skipped"] != float64(1) {
		t.Fatalf("expected on a failure streak = %v, want it skipped", m)
	}

	seedAnomaly(t, st, store.Anomaly{
		ID: "shrink2", Detector: "source", Metric: "source_bytes_shrink", Severity: "critical",
		ScopeKind: "item", ScopeID: target.ID, TargetID: target.ID, Domain: "container",
		Observed: 400, Expected: 40000, LastRunAt: now - 30, LastSeenAt: now + 1,
	})
	_, m = doJSON(t, h, http.MethodPost, "/api/anomalies/expected", `{"ids":["shrink2"]}`)
	if m["changed"] != float64(1) {
		t.Fatalf("expected on a shrink = %v, want it accepted", m)
	}
	expectations, err := st.ListAnomalyExpectationsForTarget(target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(expectations) != 1 || expectations[0].Family != "source_bytes_down" {
		t.Fatalf("expectations = %+v, want one for the shrinking source", expectations)
	}

	ids := make([]string, 501)
	for i := range ids {
		ids[i] = fmt.Sprintf("id-%d", i)
	}
	body, err := json.Marshal(map[string]any{"ids": ids})
	if err != nil {
		t.Fatal(err)
	}
	_, m = doJSON(t, h, http.MethodPost, "/api/anomalies/acknowledge", string(body))
	if m["ok"] != false || m["code"] != "bad-request" {
		t.Fatalf("501 ids = %v, want a bad-request refusal", m)
	}
	_, m = doJSON(t, h, http.MethodPost, "/api/anomalies/acknowledge", `{"ids":[]}`)
	if m["ok"] != false || m["code"] != "bad-request" {
		t.Fatalf("no ids = %v, want a bad-request refusal", m)
	}

	r := httptest.NewRequest(http.MethodPost, "/api/anomalies/acknowledge", strings.NewReader(`{"ids":["streak"]}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	cross := httptest.NewRecorder()
	h.ServeHTTP(cross, r)
	if cross.Code != http.StatusForbidden {
		t.Fatalf("cross-site acknowledge: status %d, want it refused", cross.Code)
	}
}

// Flash and the app's own configuration have no name of their own: the backend
// sends the domain and the page writes the label in the reader's language.
func TestAnomalyNamesAreNotTranslatedInBackend(t *testing.T) {
	h, st := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	now := time.Now().Unix()
	for id, domain := range map[string]string{store.FlashTargetID: "flash", store.ConfigTargetID: "config"} {
		seedAnomaly(t, st, store.Anomaly{
			ID: domain, Detector: "duration", Metric: "duration_slower", Severity: "warning",
			ScopeKind: "item", ScopeID: id, TargetID: id, Domain: domain, LastSeenAt: now,
		})
	}

	_, m := doJSON(t, h, http.MethodGet, "/api/anomalies", "")
	rows := m["anomalies"].([]any)
	if len(rows) != 2 {
		t.Fatalf("want both rows, got %v", rows)
	}
	for _, raw := range rows {
		row := raw.(map[string]any)
		if row["name"] != "" {
			t.Fatalf("%v carries a name; the page translates flash and config", row)
		}
		if row["domain"] != "flash" && row["domain"] != "config" {
			t.Fatalf("domain = %v", row["domain"])
		}
	}
}

func TestAnomalyRoutesRequireSession(t *testing.T) {
	h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	loginCookie(t, h, "correct horse battery staple")

	calls := []struct{ method, path, body string }{
		{http.MethodGet, "/api/anomalies", ""},
		{http.MethodGet, "/api/anomalies/summary", ""},
		{http.MethodGet, "/api/anomalies/items", ""},
		{http.MethodGet, "/api/anomalies/some-id", ""},
		{http.MethodPost, "/api/anomalies/acknowledge", `{"ids":["a"]}`},
		{http.MethodPost, "/api/anomalies/expected", `{"ids":["a"]}`},
		{http.MethodPut, "/api/anomalies/items/t1/prefs", `{"sensitivity":"strict"}`},
		{http.MethodDelete, "/api/anomalies/items/t1/expectations/new_data", ""},
	}
	for _, c := range calls {
		w, _ := doJSON(t, h, c.method, c.path, c.body)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: status %d, want 401 without a session", c.method, c.path, w.Code)
		}
	}
}

// Every card on the Settings page posts the whole settings object, so a save
// from a tab that predates detection must not switch it off on its way past.
func TestSettingsSaveWithoutAnomalyKeysKeepsThem(t *testing.T) {
	h, st := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	s := mustSettings(t, st)
	s.AnomalySensitivity, s.AnomalyNotifyMin = "strict", "warning"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	if _, m := doJSON(t, h, http.MethodPut, "/api/settings", settingsFormBody); m["ok"] != true {
		t.Fatalf("save: %v", m)
	}
	got := mustSettings(t, st)
	if !got.AnomalyEnabled || !got.AnomalyRetentionHold {
		t.Fatalf("detection and the pause must survive a save that does not name them: %+v", got)
	}
	if got.AnomalySensitivity != "strict" || got.AnomalyNotifyMin != "warning" {
		t.Fatalf("preset and minimum must survive it too: %q/%q", got.AnomalySensitivity, got.AnomalyNotifyMin)
	}

	_, m := doJSON(t, h, http.MethodPut, "/api/settings", `{"anomalySensitivity":"wild"}`)
	if msg, _ := m["error"].(string); m["ok"] != false || !strings.Contains(msg, "unknown anomaly sensitivity") {
		t.Fatalf("an unknown preset = %v, want the same refusal the import gives", m)
	}

	if _, m := doJSON(t, h, http.MethodPut, "/api/settings", `{"anomalyEnabled":false,"anomalyNotifyMin":"off"}`); m["ok"] != true {
		t.Fatalf("save: %v", m)
	}
	if got := mustSettings(t, st); got.AnomalyEnabled || got.AnomalyNotifyMin != "off" {
		t.Fatalf("a save that names them has to write them: %+v", got)
	}
}
