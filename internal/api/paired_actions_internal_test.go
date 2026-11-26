package api

import (
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
