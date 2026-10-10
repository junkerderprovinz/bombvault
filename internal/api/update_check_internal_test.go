package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// releaseServer answers like GitHub's latest-release endpoint and counts how
// often it was asked.
type releaseServer struct {
	*httptest.Server
	asked     int
	userAgent string
}

func newReleaseServer(t *testing.T, answer http.HandlerFunc) *releaseServer {
	t.Helper()
	rs := &releaseServer{}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.asked++
		rs.userAgent = r.Header.Get("User-Agent")
		answer(w, r)
	}))
	t.Cleanup(rs.Close)
	return rs
}

func latestIs(tag string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": tag,
			"html_url": "https://example.org/somewhere-else",
		})
	}
}

func (rs *releaseServer) check(t *testing.T, running string) (UpdateCheck, string, error) {
	t.Helper()
	return checkForUpdate(context.Background(), rs.Client(), rs.URL, running)
}

func TestUpdateCheckFindsANewerRelease(t *testing.T) {
	rs := newReleaseServer(t, latestIs("v10.1.0"))

	got, reason, err := rs.check(t, "v10.0.0")
	if err != nil || reason != "" {
		t.Fatalf("check failed: %s %v", reason, err)
	}
	want := UpdateCheck{
		Current: "v10.0.0", Latest: "v10.1.0", UpdateAvailable: true,
		ReleaseURL: "https://github.com/junkerderprovinz/bombvault/releases/tag/v10.1.0",
	}
	if got != want {
		t.Fatalf("check = %+v, want %+v", got, want)
	}
	if rs.userAgent != "BombVault/10.0.0" {
		t.Fatalf("User-Agent = %q, want it to name BombVault and its version", rs.userAgent)
	}
}

func TestUpdateCheckComparesSemanticVersions(t *testing.T) {
	for _, c := range []struct {
		name, running, latest string
		update                bool
	}{
		{"the same release", "v10.0.0", "v10.0.0", false},
		{"an edge build of the newest release", "v10.0.0+main.59b73a6", "v10.0.0", false},
		{"a build ahead of the newest release", "v10.0.1", "v10.0.0", false},
		{"a two-digit minor after a one-digit one", "v9.9.1", "v9.10.0", true},
		{"a release candidate of the newest release", "v10.0.0-rc.1", "v10.0.0", true},
		{"a version stamped without the v", "9.9.1", "v10.0.0", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, reason, err := newReleaseServer(t, latestIs(c.latest)).check(t, c.running)
			if err != nil {
				t.Fatalf("check failed: %s %v", reason, err)
			}
			if got.UpdateAvailable != c.update || got.Latest != c.latest || got.Current != c.running {
				t.Fatalf("running %s against %s = %+v, want update %v", c.running, c.latest, got, c.update)
			}
		})
	}
}

func TestUpdateCheckSaysWhyItCannotRun(t *testing.T) {
	rateLimit := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		http.Error(w, `{"message":"API rate limit exceeded"}`, http.StatusForbidden)
	}
	for _, c := range []struct {
		name   string
		answer http.HandlerFunc
		want   string
	}{
		{"rate limited", rateLimit, updateCheckRateLimited},
		{"too many requests", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "slow down", http.StatusTooManyRequests)
		}, updateCheckRateLimited},
		{"refused for another reason", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "blocked by the proxy", http.StatusForbidden)
		}, updateCheckBadResponse},
		{"no release published", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		}, updateCheckBadResponse},
		{"not JSON", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("<html>captive portal</html>"))
		}, updateCheckBadResponse},
		{"a tag that is no version", latestIs("nightly"), updateCheckBadResponse},
		{"no tag at all", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{}`))
		}, updateCheckBadResponse},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, reason, err := newReleaseServer(t, c.answer).check(t, "v10.0.0")
			if err == nil || reason != c.want {
				t.Fatalf("check = %+v, %q, %v; want it refused as %q", got, reason, err, c.want)
			}
		})
	}
}

func TestUpdateCheckWithoutNetwork(t *testing.T) {
	rs := newReleaseServer(t, latestIs("v10.1.0"))
	rs.Close()

	_, reason, err := rs.check(t, "v10.0.0")
	if err == nil || reason != updateCheckOffline {
		t.Fatalf("an unreachable server = %q, %v; want %q", reason, err, updateCheckOffline)
	}
}

func TestDevBuildAsksNobody(t *testing.T) {
	rs := newReleaseServer(t, latestIs("v10.1.0"))

	_, reason, err := rs.check(t, "dev")
	if err == nil || reason != updateCheckDevBuild {
		t.Fatalf("a dev build = %q, %v; want %q", reason, err, updateCheckDevBuild)
	}
	if rs.asked != 0 {
		t.Fatalf("a build with nothing to compare made %d outbound call(s)", rs.asked)
	}
}

func postUpdateCheck(t *testing.T) string {
	t.Helper()
	rec := httptest.NewRecorder()
	(&Handler{}).handleUpdateCheck(rec, httptest.NewRequest(http.MethodPost, "/api/update-check", nil))
	return strings.TrimSpace(rec.Body.String())
}

func TestUpdateCheckPayload(t *testing.T) {
	url, running := latestReleaseURL, Version
	t.Cleanup(func() { latestReleaseURL, Version = url, running })
	Version = "v10.0.0"

	latestReleaseURL = newReleaseServer(t, latestIs("v10.1.0")).URL
	want := `{"ok":true,"update":{"current":"v10.0.0","latest":"v10.1.0","updateAvailable":true,` +
		`"releaseUrl":"https://github.com/junkerderprovinz/bombvault/releases/tag/v10.1.0"}}`
	if got := postUpdateCheck(t); got != want {
		t.Fatalf("payload = %s\nwant      %s", got, want)
	}

	latestReleaseURL = newReleaseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}).URL
	var refused struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal([]byte(postUpdateCheck(t)), &refused); err != nil {
		t.Fatal(err)
	}
	if refused.OK || refused.Code != updateCheckRateLimited || refused.Error == "" {
		t.Fatalf("a refused check = %+v, want ok false with the reason as code", refused)
	}
}
