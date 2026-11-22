package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeGitHub answers every release with notes naming its tag, or with status
// when that is not 200, and counts the calls.
func fakeGitHub(t *testing.T, status int) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		tag := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		_ = json.NewEncoder(w).Encode(map[string]string{"body": "## Added\n\n- " + tag})
	}))
	t.Cleanup(srv.Close)
	oldURL, oldClient, oldNotes := releaseByTagURL, updateHTTPClient, githubNotes
	releaseByTagURL, updateHTTPClient, githubNotes = srv.URL+"/tags/", srv.Client(), &remoteNotes{}
	t.Cleanup(func() { releaseByTagURL, updateHTTPClient, githubNotes = oldURL, oldClient, oldNotes })
	return &calls
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

func TestReleaseNotesOfANewerReleaseComeFromGitHubOncePerWindow(t *testing.T) {
	calls := fakeGitHub(t, http.StatusOK)
	for range 3 {
		got := askNotes(t, "v99.0.0")
		if got["ok"] != true || got["body"] != "## Added\n\n- v99.0.0" {
			t.Fatalf("got %v", got)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("GitHub asked %d times", calls.Load())
	}
}

func TestReleaseNotesThatGitHubRefusesLeaveTheLinkToGitHub(t *testing.T) {
	calls := fakeGitHub(t, http.StatusNotFound)
	got := askNotes(t, "v99.0.1")
	if got["ok"] != false || got["htmlUrl"] != "https://github.com/junkerderprovinz/bombvault/releases/tag/v99.0.1" {
		t.Fatalf("got %v", got)
	}
	askNotes(t, "v99.0.1")
	if calls.Load() != 1 {
		t.Fatalf("a refusal was not remembered, GitHub asked %d times", calls.Load())
	}
}

func TestReleaseNotesAskGitHubOnlyForAPlainTag(t *testing.T) {
	calls := fakeGitHub(t, http.StatusOK)
	for _, v := range []string{"dev", "v99", "0.0.0"} {
		if got := askNotes(t, v); got["ok"] == true {
			t.Errorf("%s: got notes %v", v, got)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("GitHub asked %d times", calls.Load())
	}
	// Whatever surrounds a version, only the plain tag reaches GitHub.
	if got := askNotes(t, "../v99.0.0-rc1"); got["version"] != "v99.0.0" || got["body"] != "## Added\n\n- v99.0.0" {
		t.Fatalf("got %v", got)
	}
}

func TestReleaseNotesKeepABoundedMemory(t *testing.T) {
	fakeGitHub(t, http.StatusOK)
	for i := range remoteNotesKeys + 10 {
		askNotes(t, fmt.Sprintf("v99.1.%d", i))
	}
	if n := len(githubNotes.entries); n > remoteNotesKeys {
		t.Fatalf("%d answers kept", n)
	}
}

// NoGitHubNotes lets the handler tests outside the package ask for a release
// without notes here, with GitHub answering that it has none either.
func NoGitHubNotes(t *testing.T) {
	fakeGitHub(t, http.StatusNotFound)
}
