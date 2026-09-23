package api

// Without a login password authGate lets every request through, on the
// assumption that an attacker has to be on the LAN. A cross-site form post
// comes from the operator's own browser instead, with no preflight and no
// cookie needed.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/notify"
)

// TestCrossSiteWriteIsRefused: a form with enctype="text/plain" can post a body
// the JSON decoder accepts, and the browser sends it with
// "Sec-Fetch-Site: cross-site".
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

// TestCrossSiteBodylessWriteIsRefused: the guard is middleware rather than part
// of decodeBody because many state-changing routes, such as starting a backup
// or pruning a repo, carry no body.
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

// TestSameSiteAndNonBrowserWritesPass: anything but an explicit cross-site
// browser request passes. What a browser counts as one site for a bare LAN IP
// is unreliable, so same-site is allowed, and a missing header means a
// non-browser client such as curl or a peer.
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

// TestCrossSiteReadsPass: cross-site GETs pass, because the same-origin policy
// keeps the response unreadable and the widget iframe and a peer's status poll
// depend on them.
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

// TestNonJSONBodyIsRefused: a cross-origin form can only send urlencoded,
// multipart or text/plain bodies; anything else needs a CORS preflight this
// server never answers. Requiring JSON closes that path whatever Sec-Fetch-Site
// says.
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

// TestJSONBodyWithCharsetIsAccepted: the media type is parsed, so a charset
// parameter is fine.
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

// TestNotifySecretStaysWithItsDestination: POST /api/notify/test refills blank
// secrets from the store but takes the destination from the request. Without
// this check, a request naming another homeserver would get the stored token
// sent there as a bearer header.
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
