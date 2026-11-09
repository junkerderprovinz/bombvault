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

	kitnotify "github.com/junkerderprovinz/parleyport/kit/notify"

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

// TestNotifySecretStaysWithItsDestination: the API puts stored secrets back
// where the browser sent a placeholder, but takes the destination from the
// request. Without this check, a request naming another homeserver would get
// the stored token sent there as a bearer header.
func TestNotifySecretStaysWithItsDestination(t *testing.T) {
	svc := unraidNotifyService(t, nil)
	stored := oldNotify(map[string]any{"on": "always", "matrixEnabled": true, "matrixHomeserver": "https://matrix.example", "matrixRoom": "!room:example", "matrixToken": "real-token", "smtpEnabled": true, "smtpHost": "smtp.example", "smtpPort": 587, "smtpUsername": "bombvault", "smtpPassword": "real-password", "smtpFrom": "bv@example", "smtpTo": "me@example", "gotifyEnabled": true, "gotifyUrl": "https://push.example", "gotifyToken": "real-app-token"})
	if err := svc.SetNotifyConfig(stored); err != nil {
		t.Fatal(err)
	}
	h := &Handler{svc: svc}
	browser := func() notify.Config { return notify.Config{Channels: kitnotify.Redacted(stored.Channels)} }

	t.Run("another homeserver gets no token", func(t *testing.T) {
		req := browser()
		channelOf(req, "matrix").Matrix.Homeserver = "https://attacker.example"
		got, err := h.withStoredSecrets(req.Channels)
		if err != nil {
			t.Fatal(err)
		}
		if tok := channelOf(notify.Config{Channels: got}, "matrix").Matrix.Token; tok != "" {
			t.Fatalf("the stored token went to another homeserver: %q", tok)
		}
	})

	t.Run("another SMTP server gets no password", func(t *testing.T) {
		req := browser()
		channelOf(req, "smtp").SMTP.Host = "smtp.attacker.example"
		got, _ := h.withStoredSecrets(req.Channels)
		if pw := channelOf(notify.Config{Channels: got}, "smtp").SMTP.Password; pw != "" {
			t.Fatalf("the stored password went to another server: %q", pw)
		}
	})

	t.Run("another Gotify server gets no token", func(t *testing.T) {
		req := browser()
		channelOf(req, "gotify").Gotify.URL = "https://attacker.example"
		got, _ := h.withStoredSecrets(req.Channels)
		if tok := channelOf(notify.Config{Channels: got}, "gotify").Gotify.Token; tok != "" {
			t.Fatalf("the stored token went to another server: %q", tok)
		}
	})

	t.Run("the unchanged destination still gets its secrets back", func(t *testing.T) {
		got, err := h.withStoredSecrets(browser().Channels)
		if err != nil {
			t.Fatal(err)
		}
		c := notify.Config{Channels: got}
		if matrixToken(c) != "real-token" || channelOf(c, "smtp").SMTP.Password != "real-password" || channelOf(c, "gotify").Gotify.Token != "real-app-token" {
			t.Fatalf("secrets were not put back: %+v", got)
		}
	})
}
