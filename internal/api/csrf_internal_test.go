package api

// The cross-site write guard, and the notify-secret refill it sits next to.
//
// Both exist because authGate is a deliberate pass-through when no login
// password is set — the documented trusted-LAN default. That model assumes an
// attacker has to be on the LAN. A cross-site form post does not: it comes from
// the operator's own browser, aimed at a LAN address, with no preflight and
// needing no cookie.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/notify"
)

// TestCrossSiteWriteIsRefused: the exact shape of the attack. An HTML form with
// enctype="text/plain" can post a body the JSON decoder would accept, and the
// browser marks it Sec-Fetch-Site: cross-site.
func TestCrossSiteWriteIsRefused(t *testing.T) {
	h := csrfGate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("the handler must not be reached by a cross-site write")
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodPost, "/api/notify", strings.NewReader(`{"on":"always"}`))
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

// TestCrossSiteBodylessWriteIsRefused: the guard is middleware rather than a
// check inside decodeBody because plenty of state-changing routes carry no body
// — starting a backup, pruning a repo, running the whole-server pass. A
// body-only guard would have left exactly those reachable.
func TestCrossSiteBodylessWriteIsRefused(t *testing.T) {
	h := csrfGate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("the handler must not be reached by a cross-site write")
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodPost, "/api/backup-everything", nil)
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

// TestSameSiteAndNonBrowserWritesPass: everything that is not an explicit
// cross-site browser request keeps working. same-origin is the SPA; same-site is
// left alone because what a browser calls one "site" for a bare LAN IP is not
// worth betting an install on; an absent header is a non-browser client (curl,
// a peer's mesh POST, a script), which was never the threat.
func TestSameSiteAndNonBrowserWritesPass(t *testing.T) {
	for _, site := range []string{"same-origin", "same-site", "none", ""} {
		reached := false
		h := csrfGate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			reached = true
			w.WriteHeader(http.StatusOK)
		}))
		r := httptest.NewRequest(http.MethodPost, "/api/notify", strings.NewReader(`{}`))
		if site != "" {
			r.Header.Set("Sec-Fetch-Site", site)
		}
		h.ServeHTTP(httptest.NewRecorder(), r)
		if !reached {
			t.Errorf("Sec-Fetch-Site %q must be allowed through", site)
		}
	}
}

// TestCrossSiteReadsPass: safe methods are reachable cross-site by design. The
// browser's own same-origin policy keeps the response unreadable, and gating
// them would break the widget iframe and a peer's status poll, both GETs on
// purpose.
func TestCrossSiteReadsPass(t *testing.T) {
	reached := false
	h := csrfGate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	r := httptest.NewRequest(http.MethodGet, "/api/widget/data", nil)
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if !reached {
		t.Fatal("a cross-site GET must still be served")
	}
}

// TestNonJSONBodyIsRefused: the second half, and the one that does the work. A
// cross-origin form can only send urlencoded, multipart or text/plain; anything
// else needs a CORS preflight this server never answers. Requiring JSON removes
// the whole class, whatever the Sec-Fetch-Site header says.
func TestNonJSONBodyIsRefused(t *testing.T) {
	var body struct {
		On string `json:"on"`
	}
	r := httptest.NewRequest(http.MethodPost, "/api/notify", strings.NewReader(`{"on":"always"}`))
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	if decodeBody(w, r, &body) {
		t.Fatal("a body not declared as JSON must be refused")
	}
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnsupportedMediaType)
	}
}

// TestJSONBodyWithCharsetIsAccepted: the check parses the media type rather than
// comparing the raw header, so a client that spells out the charset is fine.
func TestJSONBodyWithCharsetIsAccepted(t *testing.T) {
	var body struct {
		On string `json:"on"`
	}
	r := httptest.NewRequest(http.MethodPost, "/api/notify", strings.NewReader(`{"on":"always"}`))
	r.Header.Set("Content-Type", "application/json; charset=utf-8")

	if !decodeBody(httptest.NewRecorder(), r, &body) {
		t.Fatal("application/json with a charset parameter must be accepted")
	}
	if body.On != "always" {
		t.Fatalf("body did not decode: %+v", body)
	}
}

// TestNotifySecretStaysWithItsDestination: POST /api/notify/test refills a blank
// Matrix token from the encrypted store while taking the destination from the
// request. A request naming its own homeserver with a blank token therefore had
// the real token attached and sent there as a bearer header — a read of a secret
// the API never hands back, dressed as a connection test.
func TestNotifySecretStaysWithItsDestination(t *testing.T) {
	svc := unraidNotifyService(t, nil)
	stored := notify.Config{
		On:               "always",
		MatrixEnabled:    true,
		MatrixHomeserver: "https://matrix.example",
		MatrixRoom:       "!room:example",
		MatrixToken:      "real-token",
		SMTPHost:         "smtp.example",
		SMTPPort:         587,
		SMTPUsername:     "bombvault",
		SMTPPassword:     "real-password",
	}
	if err := svc.SetNotifyConfig(stored); err != nil {
		t.Fatal(err)
	}
	h := &Handler{svc: svc}

	t.Run("another homeserver is refused, not silently answered", func(t *testing.T) {
		req := stored
		req.MatrixHomeserver = "https://attacker.example"
		req.MatrixToken = ""
		req.SMTPPassword = ""
		got, err := h.fillNotifySecrets(req)
		if err == nil {
			t.Fatalf("a changed homeserver with a blank token must be refused, got token %q", got.MatrixToken)
		}
		if got.MatrixToken == stored.MatrixToken {
			t.Fatal("the stored token must never reach a destination it was not saved for")
		}
	})

	t.Run("another SMTP server is refused too", func(t *testing.T) {
		req := stored
		req.SMTPHost = "smtp.attacker.example"
		req.SMTPPassword = ""
		req.MatrixToken = "supplied"
		if _, err := h.fillNotifySecrets(req); err == nil {
			t.Fatal("a changed SMTP host with a blank password must be refused")
		}
	})

	t.Run("the unchanged destination still gets its secrets back", func(t *testing.T) {
		req := stored
		req.MatrixToken = ""
		req.SMTPPassword = ""
		got, err := h.fillNotifySecrets(req)
		if err != nil {
			t.Fatalf("an unchanged destination must keep working: %v", err)
		}
		if got.MatrixToken != "real-token" || got.SMTPPassword != "real-password" {
			t.Fatalf("secrets were not refilled: %+v", got)
		}
	})
}
