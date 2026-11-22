package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeGitHub names v99.0.0 as the newest release and answers every release
// with notes naming its tag, or with status when that is not 200. It counts
// the requests for notes.
func fakeGitHub(t *testing.T, status int) *atomic.Int32 {
	t.Helper()
	var notes atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/latest") {
			_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v99.0.0"})
			return
		}
		notes.Add(1)
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		tag := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		_ = json.NewEncoder(w).Encode(map[string]string{"body": "## Added\n\n- " + tag})
	}))
	t.Cleanup(srv.Close)
	oldLatest, oldByTag, oldClient, oldNotes, oldVersion := latestReleaseURL, releaseByTagURL, updateHTTPClient, githubNotes, Version
	latestReleaseURL, releaseByTagURL, updateHTTPClient, githubNotes, Version = srv.URL+"/latest", srv.URL+"/tags/", srv.Client(), &remoteNotes{}, "v10.0.0"
	t.Cleanup(func() {
		latestReleaseURL, releaseByTagURL, updateHTTPClient, githubNotes, Version = oldLatest, oldByTag, oldClient, oldNotes, oldVersion
	})
	return &notes
}

func checkUpdate(t *testing.T) {
	t.Helper()
	rec := httptest.NewRecorder()
	(&Handler{}).handleUpdateCheck(rec, httptest.NewRequest(http.MethodPost, "/api/update-check", nil))
	if !strings.Contains(rec.Body.String(), `"updateAvailable":true`) {
		t.Fatalf("update check: %s", rec.Body)
	}
}

func askNotes(t *testing.T, version string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	(&Handler{}).handleReleaseNotes(rec, httptest.NewRequest(http.MethodGet, "/api/release-notes?version="+version, nil))
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestReleaseNotesOfTheReleaseTheUpdateCheckFoundComeFromGitHubOncePerWindow(t *testing.T) {
	notes := fakeGitHub(t, http.StatusOK)
	checkUpdate(t)
	for range 3 {
		got := askNotes(t, "v99.0.0")
		if got["ok"] != true || got["body"] != "## Added\n\n- v99.0.0" {
			t.Fatalf("got %v", got)
		}
	}
	if notes.Load() != 1 {
		t.Fatalf("GitHub asked %d times", notes.Load())
	}
}

func TestReleaseNotesOfAnyOtherTagCostNoRequest(t *testing.T) {
	notes := fakeGitHub(t, http.StatusOK)
	if got := askNotes(t, "v99.0.0"); got["ok"] != false {
		t.Fatalf("before any update check: got %v", got)
	}
	checkUpdate(t)
	for _, v := range []string{"v98.0.0", "v99.0.1", "v1.2.3", "dev"} {
		if got := askNotes(t, v); got["ok"] != false {
			t.Errorf("%s: got notes %v", v, got)
		}
	}
	if notes.Load() != 0 {
		t.Fatalf("GitHub asked %d times", notes.Load())
	}
}

func TestReleaseNotesThatGitHubRefusesLeaveTheLinkToGitHub(t *testing.T) {
	notes := fakeGitHub(t, http.StatusNotFound)
	checkUpdate(t)
	got := askNotes(t, "v99.0.0")
	if got["ok"] != false || got["htmlUrl"] != "https://github.com/junkerderprovinz/bombvault/releases/tag/v99.0.0" {
		t.Fatalf("got %v", got)
	}
	askNotes(t, "v99.0.0")
	if notes.Load() != 1 {
		t.Fatalf("a refusal was not remembered, GitHub asked %d times", notes.Load())
	}
}

// NoGitHubNotes lets the handler tests outside the package ask for a release
// without notes here, with GitHub answering that it has none either.
func NoGitHubNotes(t *testing.T) {
	fakeGitHub(t, http.StatusNotFound)
}
