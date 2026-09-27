package api_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// recoveryKitText downloads the kit the way a browser does and returns it.
func recoveryKitText(t *testing.T) string {
	t.Helper()
	h, _, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	cookie := loginCookie(t, h, "correct horse battery staple")
	w := getRaw(t, h, "/api/recovery-kit", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

// The kit is read on the day BombVault is gone, so each kind of backup has to
// be findable in it without the app: the database dumps, the ZFS datasets, the
// warning that anomaly detection exists for, and what a restored configuration
// does to the MCP keys.
func TestRecoveryKitCoversDumpsZFSAnomaliesAndMCP(t *testing.T) {
	kit := recoveryKitText(t)
	for _, want := range []string{
		"## Database dumps without BombVault",
		"## ZFS datasets",
		"- zfs (local):",
		"Anomalies page",
		"revokes every MCP key",
	} {
		if !strings.Contains(kit, want) {
			t.Errorf("the recovery kit does not say %q", want)
		}
	}
}

var restoresLatest = regexp.MustCompile(`restic .*\blatest\b`)

// After ransomware or an emptied folder the newest backup is the bad one, so
// every chapter that hands out a command restoring "latest" has to say how to
// pick an older one.
func TestRecoveryKitWarnsWhereverItRestoresTheNewestBackup(t *testing.T) {
	kit := recoveryKitText(t)
	chapters := strings.Split(kit, "\n## ")
	checked := 0
	for _, chapter := range chapters {
		if !restoresLatest.MatchString(chapter) {
			continue
		}
		checked++
		title, _, _ := strings.Cut(chapter, "\n")
		if !strings.Contains(chapter, "Anomalies page") {
			t.Errorf("the chapter %q restores the newest backup without pointing at the Anomalies page", title)
		}
	}
	if checked < 2 {
		t.Fatalf("expected the dump and ZFS chapters to restore the newest backup, found %d chapters that do", checked)
	}
}
