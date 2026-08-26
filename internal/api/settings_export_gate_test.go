package api_test

// GET /api/settings/export?includeCredentials — the second gate.
//
// authGate is a pass-through when no login password is set: that is the
// trusted-LAN model, and it is fine for CURRENT data. The credentialed export
// is not current data — it is the keys to it: the S3 access key and secret, the
// restic-REST password, the entire rclone config (every remote's tokens), the
// SMTP password and the Matrix access token, all decrypted. That is the
// recovery kit's class of payload, and the recovery kit fails closed for
// exactly this reason. This one did not, so any host on the LAN could fetch
// every backend credential the instance held with one unauthenticated GET.
//
// The tests below pin all three halves of the fix: refused without auth,
// allowed with auth, and the plain (secret-free) export still open — a gate
// that also blocked the harmless variant would just be a different bug.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/notify"
)

// Distinctive values, so an assertion on the response body cannot pass by
// accident.
const (
	exportS3Secret  = "S3-SECRET-THAT-MUST-NEVER-LEAK"
	exportRESTPass  = "REST-PASSWORD-THAT-MUST-NEVER-LEAK"
	exportSMTPPass  = "SMTP-PASSWORD-THAT-MUST-NEVER-LEAK" //nolint:gosec // G101: a test fixture string, not a credential
	exportMatrixTok = "MATRIX-TOKEN-THAT-MUST-NEVER-LEAK"
	exportRclone    = "[remote]\ntype = s3\nsecret_access_key = RCLONE-SECRET-THAT-MUST-NEVER-LEAK\n"
)

// seedExportSecrets stores one secret of every kind the credentialed export
// hands out.
func seedExportSecrets(t *testing.T, svc *api.Service) {
	t.Helper()
	if err := svc.SetCloudCreds(api.CloudCreds{
		S3KeyID: "AKIAEXAMPLE", S3Secret: exportS3Secret,
		RESTUser: "bombvault", RESTPassword: exportRESTPass,
	}); err != nil {
		t.Fatalf("seed cloud creds: %v", err)
	}
	if err := svc.SetRcloneConf(exportRclone); err != nil {
		t.Fatalf("seed rclone conf: %v", err)
	}
	if err := svc.SetNotifyConfig(notify.Config{
		On: "failure", SMTPEnabled: true, SMTPHost: "smtp.example", SMTPUsername: "bv", SMTPPassword: exportSMTPPass,
		MatrixEnabled: true, MatrixHomeserver: "https://matrix.example", MatrixRoom: "!r:example", MatrixToken: exportMatrixTok,
	}); err != nil {
		t.Fatalf("seed notify config: %v", err)
	}
}

// allSeededSecrets is every value that must never appear in a response the
// caller was not authorized to receive.
func allSeededSecrets() []string {
	return []string{exportS3Secret, exportRESTPass, exportSMTPPass, exportMatrixTok, "RCLONE-SECRET-THAT-MUST-NEVER-LEAK"}
}

func getRaw(t *testing.T, h http.Handler, path string, c *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	if c != nil {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// TestCredentialedExportRefusedWhenAuthDisabled is the regression proof: with
// no login password set, the credentialed export must refuse — exactly as the
// recovery kit does — and must not put a single stored secret on the wire.
func TestCredentialedExportRefusedWhenAuthDisabled(t *testing.T) {
	h, _, svc := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	seedExportSecrets(t, svc)

	// The recovery kit, the acknowledged precedent, refuses here.
	if kit := getRaw(t, h, "/api/recovery-kit", nil); kit.Code != http.StatusForbidden {
		t.Fatalf("precondition: recovery kit status = %d, want 403 with auth off", kit.Code)
	}

	// ?includeCredentials accepts every truthy spelling; each must be gated.
	for _, q := range []string{"true", "1", "yes", "on", "TRUE", "On"} {
		w := getRaw(t, h, "/api/settings/export?includeCredentials="+q, nil)
		if w.Code != http.StatusForbidden {
			t.Fatalf("includeCredentials=%s: status = %d, want 403 — the credentialed export must fail closed when auth is off", q, w.Code)
		}
		body := w.Body.String()
		for _, secret := range allSeededSecrets() {
			if strings.Contains(body, secret) {
				t.Fatalf("includeCredentials=%s: the response leaked a stored secret", q)
			}
		}
		if !strings.Contains(body, "login password") {
			t.Fatalf("includeCredentials=%s: refusal must say what to do, got %s", q, body)
		}
		if strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
			t.Fatalf("includeCredentials=%s: a refusal must not stream a file", q)
		}
	}
}

// TestPlainExportStillWorksWithoutAuth: the gate must be on the SECRETS, not on
// the export. The plain file carries no credentials (tokens blanked, registry
// auths dropped), so it stays available in trusted-LAN mode like the rest of
// the read API.
func TestPlainExportStillWorksWithoutAuth(t *testing.T) {
	h, _, svc := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	seedExportSecrets(t, svc)

	for _, q := range []string{"", "?includeCredentials=false", "?includeCredentials=0"} {
		w := getRaw(t, h, "/api/settings/export"+q, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("plain export%q: status = %d, want 200", q, w.Code)
		}
		if !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
			t.Fatalf("plain export%q: expected the file to stream, got %s", q, w.Body.String())
		}
		body := w.Body.String()
		for _, secret := range allSeededSecrets() {
			if strings.Contains(body, secret) {
				t.Fatalf("plain export%q leaked a stored secret — it must never carry credentials at all", q)
			}
		}
	}
}

// TestCredentialedExportWorksWhenAuthEnabled: with a login password set and a
// valid session, the export does its job. Otherwise the gate would have turned
// a leak into a broken feature.
func TestCredentialedExportWorksWhenAuthEnabled(t *testing.T) {
	h, _, svc := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	seedExportSecrets(t, svc)
	cookie := loginCookie(t, h, "correct horse battery staple")

	w := getRaw(t, h, "/api/settings/export?includeCredentials=true", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("expected the file to stream, got %s", w.Body.String())
	}
	body := w.Body.String()
	for _, secret := range allSeededSecrets() {
		if !strings.Contains(body, secret) {
			t.Fatalf("the authorized credentialed export is missing a secret it is supposed to carry")
		}
	}

	// …and the same request without the session cookie is refused by authGate.
	if unauth := getRaw(t, h, "/api/settings/export?includeCredentials=true", nil); unauth.Code != http.StatusUnauthorized {
		t.Fatalf("no cookie with auth on: status = %d, want 401", unauth.Code)
	}
}
