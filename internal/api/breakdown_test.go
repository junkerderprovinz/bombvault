package api_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func breakdownRouter(t *testing.T, eng *fakeResticEngine) (http.Handler, *store.Repo) {
	t.Helper()
	h, st := newTestRouter(t, &fakeServiceDocker{listOut: []dockercli.ContainerInfo{runningContainer("plex")}}, eng)
	s := mustSettings(t, st)
	s.ContainersPath = "rest:http://192.168.1.9:8000/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	seedTarget(t, st, "plex")
	return h, st
}

func plexSnapshots() []restic.Snapshot {
	return []restic.Snapshot{
		{ID: "aaaa1111aaaa1111", Time: "2026-09-26T02:00:00Z", Tags: []string{"container:plex"}, Paths: []string{"/host/user/appdata/plex"}},
		{ID: "bbbb2222bbbb2222", Time: "2026-09-27T02:00:00Z", Tags: []string{"container:plex"}, Paths: []string{"/host/user/appdata/plex"}},
	}
}

func plexListing() []restic.FileEntry {
	return []restic.FileEntry{
		{Path: "/host", Type: "dir"},
		{Path: "/host/user", Type: "dir"},
		{Path: "/host/user/appdata", Type: "dir"},
		{Path: "/host/user/appdata/plex", Type: "dir"},
		{Path: "/host/user/appdata/plex/Library", Type: "dir"},
		{Path: "/host/user/appdata/plex/Library/db.sqlite", Type: "file", Size: 1000},
		{Path: "/host/user/appdata/plex/Library/Cache", Type: "dir"},
		{Path: "/host/user/appdata/plex/Library/Cache/x", Type: "file", Size: 300},
		{Path: "/host/user/appdata/plex/Preferences.xml", Type: "file", Size: 5},
	}
}

// breakdownUntilReady polls like the panel does until the worker is done.
func breakdownUntilReady(t *testing.T, h http.Handler, query string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		w, m := doJSON(t, h, http.MethodGet, "/api/breakdown?"+query, "")
		if w.Code != http.StatusOK || m["ok"] != true {
			t.Fatalf("breakdown: %d %v", w.Code, m)
		}
		b := m["breakdown"].(map[string]any)
		if b["state"] != "running" {
			return b
		}
		if time.Now().After(deadline) {
			t.Fatal("the breakdown never finished")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSizeBreakdownShowsTheFoldersAndWhatTheLastBackupAdded(t *testing.T) {
	eng := &fakeResticEngine{
		snaps:         plexSnapshots(),
		noLockEntries: plexListing(),
		diffChanges: []restic.DiffChange{
			{Path: "/host/user/appdata/plex/Library/", Modifier: "U"},
			{Path: "/host/user/appdata/plex/Library/db.sqlite", Modifier: "M"},
			{Path: "/host/user/appdata/plex/Preferences.xml", Modifier: "U"},
		},
	}
	h, _ := breakdownRouter(t, eng)
	b := breakdownUntilReady(t, h, "domain=containers&item=plex")
	if b["state"] != "ready" || b["size"] != float64(1305) || b["added"] != float64(1000) || b["first"] == true {
		t.Fatalf("top = %v", b)
	}
	if b["snapshot"] != "bbbb2222" {
		t.Fatalf("snapshot = %v", b["snapshot"])
	}
	lib := b["children"].([]any)[0].(map[string]any)
	if lib["name"] != "Library" || lib["open"] != true || lib["added"] != float64(1000) {
		t.Fatalf("first row = %v", lib)
	}
	inner := breakdownUntilReady(t, h, "domain=containers&item=plex&path="+url.QueryEscape("Library"))
	rows := inner["children"].([]any)
	if len(rows) != 2 || rows[0].(map[string]any)["name"] != "db.sqlite" || rows[1].(map[string]any)["name"] != "Cache" {
		t.Fatalf("Library rows = %v", rows)
	}
}

func TestSizeBreakdownOfAFirstBackupCountsEverythingAsAdded(t *testing.T) {
	eng := &fakeResticEngine{snaps: plexSnapshots()[1:], noLockEntries: plexListing()}
	h, _ := breakdownRouter(t, eng)
	b := breakdownUntilReady(t, h, "domain=containers&item=plex")
	if b["first"] != true || b["added"] != b["size"] {
		t.Fatalf("top = %v", b)
	}
}

func TestSizeBreakdownOfAnItemWithoutBackupsSaysSo(t *testing.T) {
	h, _ := breakdownRouter(t, &fakeResticEngine{})
	if b := breakdownUntilReady(t, h, "domain=containers&item=plex"); b["state"] != "none" {
		t.Fatalf("state = %v", b["state"])
	}
}

func TestSizeBreakdownReportsAFailureAndTriesAgainOnRequest(t *testing.T) {
	eng := &fakeResticEngine{snaps: plexSnapshots(), noLockEntries: plexListing(), breakdownErr: errString("repository busy")}
	h, _ := breakdownRouter(t, eng)
	b := breakdownUntilReady(t, h, "domain=containers&item=plex")
	if b["state"] != "failed" || b["error"] == "" {
		t.Fatalf("breakdown = %v", b)
	}
	if again := breakdownUntilReady(t, h, "domain=containers&item=plex"); again["state"] != "failed" {
		t.Fatalf("a failure was retried without being asked: %v", again)
	}
	eng.breakdownErr = nil
	if b := breakdownUntilReady(t, h, "domain=containers&item=plex&retry=1"); b["state"] != "ready" {
		t.Fatalf("after a retry: %v", b)
	}
}

func TestSizeBreakdownRefusesOtherDomains(t *testing.T) {
	h, _ := breakdownRouter(t, &fakeResticEngine{})
	_, m := doJSON(t, h, http.MethodGet, "/api/breakdown?domain=flash", "")
	if m["ok"] != false {
		t.Fatalf("flash = %v", m)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestABackupRefreshesABreakdownSomeoneAskedFor(t *testing.T) {
	eng := &fakeResticEngine{snaps: plexSnapshots(), noLockEntries: plexListing()}
	h, st := breakdownRouter(t, eng)
	breakdownUntilReady(t, h, "domain=containers&item=plex")
	tg, err := st.GetTargetByContainer("plex")
	if err != nil {
		t.Fatal(err)
	}
	const next = "cccc3333cccc3333"
	eng.snaps = append(plexSnapshots(), restic.Snapshot{ID: next, Time: "2026-09-28T02:00:00Z", Tags: []string{"container:plex"}, Paths: []string{"/host/user/appdata/plex"}})
	seedRun(t, st, tg.ID, "backup", "success", store.RunMeta{})
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok, _ := st.GetSizeBreakdown(tg.ID, next); ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the new backup got no breakdown")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestMCPGetSizeBreakdownLeavesTheHostPathOut(t *testing.T) {
	eng := &fakeResticEngine{snaps: plexSnapshots(), noLockEntries: plexListing()}
	docker := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{runningContainer("plex")}}
	h, st, _, key := newMCPToolRouter(t, docker, eng)
	s := mustSettings(t, st)
	s.ContainersPath = "rest:http://192.168.1.9:8000/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	seedTarget(t, st, "plex")
	deadline := time.Now().Add(5 * time.Second)
	for {
		res := mcpCallTool(t, h, key, "get_size_breakdown", `{"domain":"containers","item":"plex"}`)
		if res.IsError {
			t.Fatalf("get_size_breakdown: %v", res.Structured)
		}
		b := res.Structured["breakdown"].(map[string]any)
		if b["state"] == "ready" {
			if _, ok := b["root"]; ok {
				t.Fatalf("the host path reached the assistant: %v", b)
			}
			if b["size"] != float64(1305) {
				t.Fatalf("breakdown = %v", b)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("still %v", b["state"])
		}
		time.Sleep(20 * time.Millisecond)
	}
	if code := mcpCallTool(t, h, key, "get_size_breakdown", `{"domain":"zfs","item":"x"}`).code(t); code != "invalid_argument" {
		t.Fatalf("zfs gives %q", code)
	}
}
