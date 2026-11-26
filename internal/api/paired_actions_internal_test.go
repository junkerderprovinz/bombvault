package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// pairedInstance is an instance whose requests go out with the session of a
// paired phone.
func pairedInstance(t *testing.T) *instance {
	t.Helper()
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	hash, epoch, _ := in.h.phoneSigning()
	in.session = listedPhoneSession(t, in.st, in.appKey, hash, epoch)
	return in
}

func TestAPairedPhoneReadsThatTheKeyIsLockedAndNothingElseOfTheEmergency(t *testing.T) {
	in := pairedInstance(t)
	if err := in.st.SetAdoptedKey([]byte("sealed under another APP_KEY"), "kit", true); err != nil {
		t.Fatal(err)
	}
	in.svc.forgetBackupKey()

	code, out := in.do(t, http.MethodGet, "/api/status", nil)
	if code != http.StatusOK || out["backupKey"] != backupKeyLocked {
		t.Fatalf("the overview with a paired session: %d, backupKey %v, want %q", code, out["backupKey"], backupKeyLocked)
	}
	if code, out := in.do(t, http.MethodGet, "/api/emergency", nil); code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
		t.Errorf("the emergency with a paired session: %d %v", code, out)
	}
}

func TestAPairedPhoneStartsARestoreProbeAndAStartTest(t *testing.T) {
	in := pairedInstance(t)
	for _, c := range []struct{ path, answer string }{
		{"/api/checks/probe/" + store.FlashTargetID, "no backup with data to check"},
		{"/api/checks/starttest/gone", "no container with id"},
	} {
		code, out := in.do(t, http.MethodPost, c.path, nil)
		if code != http.StatusOK || !strings.Contains(fmt.Sprint(out["error"]), c.answer) {
			t.Errorf("%s with a paired session: %d %v, want the check's own answer", c.path, code, out)
		}
	}
	// The probe of a ZFS item's snapshot access reads the host, which the
	// Verify tab does not.
	if code, out := in.do(t, http.MethodPost, "/api/zfs/datasets/abc/probe", nil); code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
		t.Errorf("the ZFS access probe with a paired session: %d %v", code, out)
	}
}

func TestAPairedPhoneChecksForAnUpdateAndReadsItsNotes(t *testing.T) {
	fakeGitHub(t, http.StatusOK)
	in := pairedInstance(t)
	code, out := in.do(t, http.MethodPost, "/api/update-check", nil)
	update, _ := out["update"].(map[string]any)
	if code != http.StatusOK || update["updateAvailable"] != true || update["latest"] != "v99.0.0" {
		t.Fatalf("the update check with a paired session: %d %v", code, out)
	}
	if code, out := in.do(t, http.MethodGet, "/api/release-notes?version=v99.0.0", nil); code != http.StatusOK || out["body"] != "## Added\n\n- v99.0.0" {
		t.Errorf("the notes of the newer release with a paired session: %d %v", code, out)
	}
}

func TestAPairedPhoneTakesAnEntryOutOfTheScheduleAndBackIn(t *testing.T) {
	in := pairedInstance(t)
	in.h.scheduler = schedule.New(func(string) error { return nil }, in.st.ListTargets)
	set, err := in.st.CreateFileSet(store.FileSet{Name: "docs", Path: "docs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	ds, err := in.st.CreateZFSDataset(store.ZFSDataset{Dataset: "tank/data", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	code, out := in.do(t, http.MethodPut, "/api/items/files/"+set.ID+"/included", map[string]any{"included": false})
	item, _ := out["item"].(map[string]any)
	if code != http.StatusOK || out["ok"] != true || item["included"] != false {
		t.Fatalf("pausing a folder set with a paired session: %d %v", code, out)
	}
	if got, _ := in.st.GetFileSet(set.ID); got.Enabled {
		t.Error("the folder set is still in the schedule")
	}
	for _, included := range []bool{false, true} {
		if code, out := in.do(t, http.MethodPut, "/api/items/zfs/"+ds.ID+"/included", map[string]any{"included": included}); code != http.StatusOK || out["ok"] != true {
			t.Fatalf("setting a ZFS item's schedule to %v with a paired session: %d %v", included, code, out)
		}
		if got, _ := in.st.GetZFSDataset(ds.ID); got.Enabled != included {
			t.Errorf("the ZFS item is in the schedule: %v, want %v", got.Enabled, included)
		}
	}
	if code, out := in.do(t, http.MethodPut, "/api/items/files/gone/included", map[string]any{"included": true}); code != http.StatusNotFound || out["code"] != "not-found" {
		t.Errorf("an entry that is not there: %d %v", code, out)
	}
}

func TestTheScheduleRouteChangesNothingElseOfAnEntry(t *testing.T) {
	in := pairedInstance(t)
	set, err := in.st.CreateFileSet(store.FileSet{Name: "docs", Path: "docs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`{"included":false,"path":"elsewhere"}`,
		`{"included":false,"preHook":"touch /tmp/x"}`,
		`{"included":false,"home":{"repo":"abc"}}`,
		`{"included":"no"}`,
		`{}`,
		`not json`,
	} {
		req := httptest.NewRequest(http.MethodPut, "/api/items/files/"+set.ID+"/included", strings.NewReader(body))
		req.Host = "localhost"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.AddCookie(sessionCookieFor(in.session))
		w := httptest.NewRecorder()
		in.router.ServeHTTP(w, req)
		if strings.Contains(w.Body.String(), `"ok":true`) {
			t.Errorf("%s was taken: %d %s", body, w.Code, w.Body)
		}
	}
	got, err := in.st.GetFileSet(set.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Enabled || got.Path != "docs" {
		t.Errorf("a refused body changed the set: enabled %v, path %q", got.Enabled, got.Path)
	}
	for _, c := range []struct{ path, body string }{
		{"/api/files/sets/" + set.ID, `{"enabled":false}`},
		{"/api/zfs/datasets/abc", `{"enabled":false}`},
		{"/api/containers/plex", `{"includeInSchedule":false}`},
		{"/api/vms/win", `{"includeInSchedule":false}`},
	} {
		var body map[string]any
		_ = json.Unmarshal([]byte(c.body), &body)
		if code, out := in.do(t, http.MethodPatch, c.path, body); code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
			t.Errorf("PATCH %s with a paired session: %d %v", c.path, code, out)
		}
	}
}

func TestAPairedPhoneDeclinesAnEntryTagsABackupAndSweepsAZFSTree(t *testing.T) {
	in := pairedInstance(t)
	set, err := in.st.CreateFileSet(store.FileSet{Name: "docs", Path: "/mnt/user/docs"})
	if err != nil {
		t.Fatal(err)
	}
	code, out := in.do(t, http.MethodPut, "/api/items/files/"+set.ID+"/declined", map[string]any{"declined": true})
	item, _ := out["item"].(map[string]any)
	if code != http.StatusOK || item["backupDeclined"] != true {
		t.Fatalf("declining a folder set with a paired session: %d %v", code, out)
	}
	if code, out := in.do(t, http.MethodPut, "/api/items/files/"+set.ID+"/declined", map[string]any{"declined": false}); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("taking the decline back with a paired session: %d %v", code, out)
	}

	// The tags BombVault reads a meaning from stay out of reach, since
	// retention keeps an imported backup by its tag.
	code, out = in.do(t, http.MethodPost, "/api/items/files/"+set.ID+"/tag", map[string]any{"snapshotId": strings.Repeat("a", 64), "remove": []string{"imported"}})
	if code != http.StatusOK || out["ok"] != false || !strings.Contains(fmt.Sprint(out["error"]), "reserved") {
		t.Errorf("removing the imported tag with a paired session: %d %v", code, out)
	}

	code, out = in.do(t, http.MethodPost, "/api/zfs/datasets/gone/sweep", nil)
	if code == http.StatusForbidden || out["code"] == codeOwnLoginNeeded {
		t.Errorf("the sweep with a paired session: %d %v", code, out)
	}
	// The safety snapshots a restore kept are another matter: removing them
	// takes the way back from that restore.
	if code, out := in.do(t, http.MethodDelete, "/api/zfs/datasets/gone/safety-snapshots", nil); code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
		t.Errorf("removing the safety snapshots with a paired session: %d %v", code, out)
	}
}

func TestAPairedPhoneDiscoversEntriesWithoutBeingToldWhereTheyLie(t *testing.T) {
	f := newPlacementFixture(t)
	f.hold(f.domainPath("files"), snap("aaaa0001", 200, "fileset:Photos"))
	discover := func(paired bool) map[string]any {
		req := jsonReq(http.MethodPost, "/api/files/discover", nil)
		if paired {
			req = withPaired(req)
		}
		w := httptest.NewRecorder()
		f.h.handleDiscoverFiles(w, req)
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if out := discover(false); out["repo"] == nil {
		t.Fatalf("the person is not told which repository was read, so this test proves nothing: %v", out)
	}
	out := discover(true)
	if out["ok"] != true || out["discovered"] == nil {
		t.Fatalf("discover with a paired session: %v", out)
	}
	if _, told := out["repo"]; told {
		t.Errorf("a paired phone was told the repository: %v", out["repo"])
	}
	if strings.Contains(fmt.Sprint(out), f.root) {
		t.Errorf("a paired phone was told where the backups lie: %v", out)
	}
}

// heldFinding raises a collapse of the nextcloud container, which keeps its
// old backups, and returns the finding and the series it holds.
func heldFinding(t *testing.T, f *engineFixture) (store.Anomaly, anomalyScope) {
	t.Helper()
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 11, 40*gib)
	f.run(t, id, "backup", f.now-3600, 20*mib)
	f.pass(t)
	row := onlyRow(t, f.openRows(t))
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !anomalyHolds(row, settings) {
		t.Fatalf("the collapse %s keeps no backups, so this test proves nothing", row.Metric)
	}
	return row, anomalyScope{Kind: row.ScopeKind, ID: row.ScopeID}
}

// settleAs sends one of the anomaly actions straight to its handler, as a
// paired phone or as the person.
func settleAs(t *testing.T, handle http.HandlerFunc, path string, paired bool, ids ...string) map[string]any {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"ids": ids})
	req := jsonReq(http.MethodPost, path, strings.NewReader(string(b)))
	if paired {
		req = withPaired(req)
	}
	rec := httptest.NewRecorder()
	handle(rec, req)
	return decodeEnvelope(t, rec)
}

// stillHeld reports whether every view of the hold says the old backups of sc
// are kept: the retention guard, the tags a pass leaves alone and the finding.
func stillHeld(t *testing.T, f *engineFixture, sc anomalyScope, id string) bool {
	t.Helper()
	held, _, err := f.e.RetentionHeld(context.Background(), sc)
	if err != nil {
		t.Fatal(err)
	}
	tags, err := f.e.HeldIdentityTags()
	if err != nil {
		t.Fatal(err)
	}
	view, _, err := f.svc.GetAnomaly(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if held != tags.holds("container:nextcloud") || held != view.RetentionHeld {
		t.Fatalf("the views of the hold disagree: guard %v, tags %v, finding %v", held, tags.holds("container:nextcloud"), view.RetentionHeld)
	}
	return held
}

func TestAFindingAPairedPhoneSettlesKeepsTheOldBackups(t *testing.T) {
	for _, c := range []struct {
		name   string
		path   string
		handle func(h *Handler) http.HandlerFunc
		state  string
	}{
		{"acknowledged", "/api/anomalies/acknowledge", func(h *Handler) http.HandlerFunc { return h.handleAcknowledgeAnomalies }, "acknowledged"},
		{"marked as expected", "/api/anomalies/expected", func(h *Handler) http.HandlerFunc { return h.handleAnomaliesExpected }, "expected"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newEngineFixture(t)
			h := &Handler{store: f.st, svc: f.svc}
			row, sc := heldFinding(t, f)

			out := settleAs(t, c.handle(h), c.path, true, row.ID)
			if out["ok"] != true || out["changed"] != float64(1) || out["released"] != float64(0) || out["kept"] != float64(1) {
				t.Fatalf("settling the finding with a paired session: %v", out)
			}
			got, _, err := f.st.GetAnomaly(row.ID)
			if err != nil || got.State != c.state {
				t.Fatalf("the finding is %q (%v), want %q", got.State, err, c.state)
			}
			if !stillHeld(t, f, sc, row.ID) {
				t.Fatal("a paired phone ended the hold on the old backups")
			}
			f.e.refresh()
			if n := f.e.summary().RetentionHeld; n != 1 {
				t.Errorf("the summary counts %d held item(s), want 1", n)
			}
			page, err := f.svc.ListAnomalies(context.Background(), store.AnomalyFilter{States: []string{c.state}, Since: f.now + 86400})
			if err != nil || len(page.Anomalies) != 1 || !page.Anomalies[0].RetentionHeld {
				t.Errorf("the settled finding that keeps backups is not listed with its hold: %+v (%v)", page.Anomalies, err)
			}

			if out := settleAs(t, h.handleReleaseAnomalyHolds, "/api/anomalies/release", false, row.ID); out["ok"] != true || out["released"] != float64(1) {
				t.Fatalf("the person ending the hold: %v", out)
			}
			if stillHeld(t, f, sc, row.ID) {
				t.Error("the hold outlived the person ending it")
			}
		})
	}
}

func TestThePersonSettlingAFindingEndsItsHold(t *testing.T) {
	f := newEngineFixture(t)
	h := &Handler{store: f.st, svc: f.svc}
	row, sc := heldFinding(t, f)
	out := settleAs(t, h.handleAcknowledgeAnomalies, "/api/anomalies/acknowledge", false, row.ID)
	if out["ok"] != true || out["released"] != float64(1) || out["kept"] != nil {
		t.Fatalf("acknowledging the finding as the person: %v", out)
	}
	if stillHeld(t, f, sc, row.ID) {
		t.Error("the hold outlived the acknowledgement")
	}
}

func TestAPairedPhoneEndsNoHold(t *testing.T) {
	f := newEngineFixture(t)
	h := &Handler{store: f.st, svc: f.svc}
	row, _ := heldFinding(t, f)
	settleAs(t, h.handleAcknowledgeAnomalies, "/api/anomalies/acknowledge", true, row.ID)
	if out := settleAs(t, h.handleReleaseAnomalyHolds, "/api/anomalies/release", true, row.ID); out["ok"] != false || out["code"] != codeOwnLoginNeeded {
		t.Errorf("a paired phone ending the hold at the handler: %v", out)
	}

	in := pairedInstance(t)
	if code, out := in.do(t, http.MethodPost, "/api/anomalies/release", map[string]any{"ids": []string{"abc"}}); code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
		t.Errorf("ending a hold with a paired session: %d %v", code, out)
	}
	for _, path := range []string{"/api/anomalies/acknowledge", "/api/anomalies/expected"} {
		if code, out := in.do(t, http.MethodPost, path, map[string]any{"ids": []string{"abc"}}); code != http.StatusOK || out["ok"] != true {
			t.Errorf("%s with a paired session: %d %v", path, code, out)
		}
	}
}

func TestAPairedPhoneSetsAnEntrysSensitivityAndNothingElseOfIt(t *testing.T) {
	in := pairedInstance(t)
	set, err := in.st.CreateFileSet(store.FileSet{Name: "docs", Path: "docs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/anomalies/items/" + set.ID + "/sensitivity"
	if code, out := in.do(t, http.MethodPut, path, map[string]any{"sensitivity": "strict"}); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("setting the sensitivity with a paired session: %d %v", code, out)
	}
	for _, body := range []any{
		map[string]any{"sensitivity": "permissive", "notifyMin": "none"},
		map[string]any{"notifyMin": "none"},
		map[string]any{"sensitivity": "loud"},
		map[string]any{},
		[]string{"permissive"},
	} {
		if code, out := in.do(t, http.MethodPut, path, body); code != http.StatusOK || out["ok"] != false {
			t.Errorf("%v with a paired session: %d %v", body, code, out)
		}
	}
	prefs, err := in.st.ListItemPrefs()
	if err != nil {
		t.Fatal(err)
	}
	if got := prefs[set.ID]; got.Sensitivity != "strict" || got.NotifyMin != "" {
		t.Errorf("the entry's preferences are %+v, want the strict sensitivity alone", got)
	}
	if code, out := in.do(t, http.MethodPut, "/api/anomalies/items/"+set.ID+"/prefs", map[string]any{"notifyMin": "none"}); code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
		t.Errorf("the whole preferences with a paired session: %d %v", code, out)
	}
	code, out := in.do(t, http.MethodDelete, "/api/anomalies/items/"+set.ID+"/expectations/"+familyNewData, nil)
	if code != http.StatusOK || out["ok"] != true {
		t.Errorf("forgetting an expectation with a paired session: %d %v", code, out)
	}
}
