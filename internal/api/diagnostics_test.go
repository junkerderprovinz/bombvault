package api_test

// GET /api/diagnostics — the support bundle.
//
// A file made to be attached to a bug report is the worst possible place for a
// credential, and it is also the file most likely to be posted in public. So it
// is gated like the recovery kit, and every member inside it is checked, not
// just the response as a whole: a ZIP is a container, and "the secret is not in
// the response body" is trivially true of any compressed archive.

import (
	"archive/zip"
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/logring"
)

// zipMembers unpacks the response and returns each member's name and contents,
// so assertions run against what a recipient actually opens.
func zipMembers(t *testing.T, raw []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("the bundle is not a readable zip: %v", err)
	}
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		out[f.Name] = string(b)
	}
	return out
}

// TestDiagnosticsRefusedWhenAuthDisabled: the bundle carries the whole
// configuration and the recent log, so it fails closed exactly as the recovery
// kit does rather than being fetchable by anyone on the LAN.
func TestDiagnosticsRefusedWhenAuthDisabled(t *testing.T) {
	h, _, svc := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	seedExportSecrets(t, svc)

	w := getRaw(t, h, "/api/diagnostics", nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 with auth off", w.Code)
	}
	if !strings.Contains(w.Body.String(), "login password") {
		t.Fatalf("a refusal must say what to do, got %s", w.Body.String())
	}
	if strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("a refusal must not stream a file")
	}
}

// TestDiagnosticsCarriesNoSecretInAnyMember is the test the whole feature hangs
// on. Every seeded credential is checked against every unpacked member, so a
// new member added later cannot quietly become a leak.
func TestDiagnosticsCarriesNoSecretInAnyMember(t *testing.T) {
	h, _, svc := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	seedExportSecrets(t, svc)
	cookie := loginCookie(t, h, "correct horse battery staple")

	w := getRaw(t, h, "/api/diagnostics", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/zip" {
		t.Fatalf("content type = %q, want application/zip", ct)
	}
	if !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("the bundle must download as a file, got %s", w.Header().Get("Content-Disposition"))
	}

	members := zipMembers(t, w.Body.Bytes())
	for name, body := range members {
		for _, secret := range allSeededSecrets() {
			if strings.Contains(body, secret) {
				t.Fatalf("%s carries a stored secret. A diagnostics bundle is made to be "+
					"attached to a bug report and posted in public.", name)
			}
		}
	}
}

// TestDiagnosticsCarriesTheMembersSupportNeeds: the gate and the redaction are
// worth nothing if the file is empty. These are the four questions a support
// thread opens with — what does the host look like, how is it configured, what
// ran recently, and what is scheduled next.
func TestDiagnosticsCarriesTheMembersSupportNeeds(t *testing.T) {
	h, _, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	cookie := loginCookie(t, h, "correct horse battery staple")

	w := getRaw(t, h, "/api/diagnostics", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}

	members := zipMembers(t, w.Body.Bytes())
	for _, want := range []string{"manifest.json", "spike.json", "settings.json", "runs.json", "scheduler.json", "log.txt"} {
		if _, ok := members[want]; !ok {
			t.Fatalf("the bundle is missing %s. Members present: %v", want, memberNames(members))
		}
	}

	// The manifest has to say the file is redacted, or a recipient cannot tell
	// it apart from a configuration backup and may treat it as one.
	if !strings.Contains(members["manifest.json"], "redacted") {
		t.Fatalf("the manifest must state that the bundle is redacted: %s", members["manifest.json"])
	}
}

func memberNames(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestDiagnosticsScrubsTheLog closes the loop the log member opens. Run errors
// are scrubbed because restic and rclone print their repository URL, credential
// and all, on failure — and the same output goes to the standard logger, which
// is what the ring records. A bundle that redacted the runs table and then
// shipped the identical string inside log.txt would be redacted in name only.
func TestDiagnosticsScrubsTheLog(t *testing.T) {
	h, _, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	cookie := loginCookie(t, h, "correct horse battery staple")

	// main() is what tees the standard logger into the ring, and main does not
	// run here. Without this the ring stays empty, log.txt comes out blank, and
	// the assertion below would pass while proving nothing at all.
	prev := log.Writer()
	log.SetOutput(logring.Default.Tee(io.Discard))
	t.Cleanup(func() { log.SetOutput(prev) })

	// Exactly the shape restic fails with, written through the standard logger
	// so it travels the real path into the ring.
	const logged = "restic: Fatal: unable to open repository at rest:https://admin:LOGGED-PASSWORD@backup.example:8000/repo"
	log.Print(logged)

	w := getRaw(t, h, "/api/diagnostics", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}

	members := zipMembers(t, w.Body.Bytes())
	// The line has to be IN there, or this test is green because the bundle
	// carries no log rather than because it carries a redacted one.
	if !strings.Contains(members["log.txt"], "unable to open repository") {
		t.Fatalf("log.txt did not capture the logged line at all, so this test proves nothing: %q", members["log.txt"])
	}
	if strings.Contains(members["log.txt"], "LOGGED-PASSWORD") {
		t.Fatalf("log.txt carries a password that was logged during a failure.\n" +
			"The runs table is scrubbed for exactly this reason; the log has to be too.")
	}
}
