package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The Items tab is served from the same pass that raises the findings, so the
// two can never tell a different story about what an item usually does. The
// payload is pinned field by field here: the page reads these names.

// itemsPayload mirrors the JSON of GET /api/anomalies/items. Decoding into it
// with DisallowUnknownFields fails on a key the page does not know.
type itemsPayload struct {
	OK    bool `json:"ok"`
	Items []struct {
		TargetID           string `json:"targetId"`
		Domain             string `json:"domain"`
		Name               string `json:"name"`
		Scheduled          bool   `json:"scheduled"`
		Sensitivity        string `json:"sensitivity"`
		Effective          string `json:"effective"`
		NotifyMin          string `json:"notifyMin"`
		EffectiveNotifyMin string `json:"effectiveNotifyMin"`
		Learning           struct {
			Samples  int  `json:"samples"`
			Needed   int  `json:"needed"`
			NewData  int  `json:"newData"`
			Source   int  `json:"source"`
			Duration int  `json:"duration"`
			NoData   bool `json:"noData"`
		} `json:"learning"`
		Typical struct {
			SourceBytes  *int64 `json:"sourceBytes"`
			NewDataBytes *int64 `json:"newDataBytes"`
			ResticMS     *int64 `json:"resticMs"`
		} `json:"typical"`
		Dump *struct {
			Part     string `json:"part"`
			Learning struct {
				Samples int `json:"samples"`
				Needed  int `json:"needed"`
			} `json:"learning"`
			Typical struct {
				SourceBytes *int64 `json:"sourceBytes"`
				ResticMS    *int64 `json:"resticMs"`
			} `json:"typical"`
			Open          map[string]int `json:"open"`
			RetentionHeld bool           `json:"retentionHeld"`
		} `json:"dump"`
		Datasets       []any          `json:"datasets"`
		Open           map[string]int `json:"open"`
		RetentionHeld  bool           `json:"retentionHeld"`
		SelectionSince int64          `json:"selectionSince"`
		Expectations   []struct {
			Family    string  `json:"family"`
			ScopeKind string  `json:"scopeKind"`
			Part      string  `json:"part"`
			SinceAt   int64   `json:"sinceAt"`
			Ceiling   float64 `json:"ceiling"`
			UpdatedAt int64   `json:"updatedAt"`
		} `json:"expectations"`
	} `json:"items"`
}

func itemsHandler(f *engineFixture) *Handler {
	return &Handler{store: f.st, svc: f.svc}
}

func getItems(t *testing.T, h *Handler) itemsPayload {
	t.Helper()
	rec := httptest.NewRecorder()
	h.handleAnomalyItems(rec, httptest.NewRequest(http.MethodGet, "/api/anomalies/items", nil))
	dec := json.NewDecoder(strings.NewReader(rec.Body.String()))
	dec.DisallowUnknownFields()
	var payload itemsPayload
	if err := dec.Decode(&payload); err != nil {
		t.Fatalf("the items payload must decode into the shape the page reads: %v (body=%s)", err, rec.Body.String())
	}
	if !payload.OK {
		t.Fatalf("items envelope: %s", rec.Body.String())
	}
	return payload
}

func TestAnomalyItemsAndPrefs(t *testing.T) {
	f := newEngineFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersEnabled, settings.VMsEnabled, settings.FilesEnabled = true, true, true
	settings.FlashEnabled, settings.ConfigEnabled = true, true
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	plex := f.container(t, "plex")
	f.steadySeries(t, plex, "backup", 12, 40*gib)
	f.steadySeries(t, plex, "dbdump", 12, 300*mib)

	excluded, err := f.st.UpsertTarget(store.Target{ContainerName: "archive"})
	if err != nil {
		t.Fatal(err)
	}
	f.steadySeries(t, excluded.ID, "backup", 12, 5*gib)

	vm, err := f.st.UpsertVMTarget(store.VMTarget{Name: "win11", IncludeInSchedule: true})
	if err != nil {
		t.Fatal(err)
	}
	folders, err := f.st.CreateFileSet(store.FileSet{Name: "photos", Path: "photos", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.CreateFileSet(store.FileSet{Name: "scratch", Path: "scratch"}); err != nil {
		t.Fatal(err)
	}

	f.e.MarkAllDirty()
	f.pass(t)

	h := itemsHandler(f)
	items := getItems(t, h).Items
	listed := map[string]int{}
	for i, item := range items {
		listed[item.TargetID] = i
	}
	for _, want := range []string{plex, excluded.ID, vm.ID, folders.ID, store.FlashTargetID, store.ConfigTargetID} {
		if _, ok := listed[want]; !ok {
			t.Fatalf("item %s is not listed: %+v", want, items)
		}
	}
	if _, ok := listed["scratch"]; ok {
		t.Fatal("a folder set that is switched off and never ran is nothing to watch")
	}

	container := items[listed[plex]]
	if container.Domain != "container" || container.Name != "plex" || !container.Scheduled {
		t.Fatalf("the container row = %+v", container)
	}
	if container.Learning.Samples != container.Learning.Needed || container.Typical.SourceBytes == nil {
		t.Fatalf("twelve backups are enough to know the usual size: %+v", container.Learning)
	}
	if container.Dump == nil || container.Dump.Typical.SourceBytes == nil {
		t.Fatalf("a container that dumps its database carries that series too: %+v", container.Dump)
	}
	if container.Datasets == nil {
		t.Fatal("datasets has to be an array, so the page can loop over it")
	}

	if row := items[listed[excluded.ID]]; row.Scheduled || row.Learning.Samples != 0 {
		t.Fatalf("an excluded container with history is listed without a learning state: %+v", row)
	}
	// The VM, the folder set, flash and the configuration have no history yet;
	// the container that is excluded from the schedule is not learning at all.
	if summary := f.e.summary(); summary.LearningItems != 4 {
		t.Fatalf("learning items = %d, want the four scheduled items without history", summary.LearningItems)
	}
	if row := items[listed[store.FlashTargetID]]; row.Domain != "flash" || row.Name != "" {
		t.Fatalf("flash is named by the page, not here: %+v", row)
	}

	// The two controls save on their own, so one of them must not carry the
	// other back to what it was when the page loaded.
	if env := putPrefs(t, h, plex, `{"sensitivity":"strict"}`); env["ok"] != true {
		t.Fatalf("prefs: %v", env)
	}
	if env := putPrefs(t, h, plex, `{"notifyMin":"warning"}`); env["ok"] != true {
		t.Fatalf("prefs: %v", env)
	}
	row := getItems(t, h).Items[listed[plex]]
	if row.Sensitivity != "strict" || row.Effective != "strict" {
		t.Fatalf("sensitivity = %q/%q", row.Sensitivity, row.Effective)
	}
	if row.NotifyMin != "warning" || row.EffectiveNotifyMin != "warning" {
		t.Fatalf("notification minimum = %q/%q", row.NotifyMin, row.EffectiveNotifyMin)
	}

	if env := putPrefs(t, h, "nobody", `{"sensitivity":"strict"}`); env["code"] != "not-found" {
		t.Fatalf("an unknown item = %v, want a not-found refusal", env)
	}
	if env := putPrefs(t, h, plex, `{"sensitivity":"bogus"}`); env["code"] != "bad-request" {
		t.Fatalf("an unknown preset = %v, want a bad-request refusal", env)
	}
	if env := putPrefs(t, h, plex, `{"notifyMin":"bogus"}`); env["code"] != "bad-request" {
		t.Fatalf("an unknown minimum = %v, want a bad-request refusal", env)
	}

	if err := f.st.UpsertAnomalyExpectation(store.AnomalyExpectation{
		ScopeKind: anomalyScopeDump, ScopeID: plex, TargetID: plex,
		Family: familyDumpBytesDown, UpdatedAt: f.now,
	}); err != nil {
		t.Fatal(err)
	}
	f.e.refresh()
	if row := getItems(t, h).Items[listed[plex]]; len(row.Expectations) != 1 {
		t.Fatalf("the expectation must show on its item: %+v", row.Expectations)
	}
	if env := forgetExpectation(t, h, plex, familyDumpBytesDown, "?scope=dump"); env["ok"] != true {
		t.Fatalf("forget: %v", env)
	}
	if row := getItems(t, h).Items[listed[plex]]; len(row.Expectations) != 0 {
		t.Fatalf("a forgotten expectation is gone from the item: %+v", row.Expectations)
	}
	if env := forgetExpectation(t, h, plex, familyNewData, "?scope=zfsds"); env["code"] != "bad-request" {
		t.Fatalf("a dataset scope without a dataset = %v, want a bad-request refusal", env)
	}
}

func putPrefs(t *testing.T, h *Handler, targetID, body string) map[string]any {
	t.Helper()
	r := jsonReq(http.MethodPut, "/api/anomalies/items/"+targetID+"/prefs", strings.NewReader(body))
	r.SetPathValue("targetId", targetID)
	rec := httptest.NewRecorder()
	h.handleAnomalyItemPrefs(rec, r)
	return decodeEnvelope(t, rec)
}

func forgetExpectation(t *testing.T, h *Handler, targetID, family, query string) map[string]any {
	t.Helper()
	r := httptest.NewRequest(http.MethodDelete,
		"/api/anomalies/items/"+targetID+"/expectations/"+family+query, nil)
	r.SetPathValue("targetId", targetID)
	r.SetPathValue("family", family)
	rec := httptest.NewRecorder()
	h.handleForgetAnomalyExpectation(rec, r)
	return decodeEnvelope(t, rec)
}
