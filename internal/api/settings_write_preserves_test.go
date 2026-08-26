package api_test

// PUT /api/settings must not destroy what it does not manage.
//
// The settings form owns a fixed set of columns. Everything else in the row —
// the login password, the session epoch, the encrypted credential blobs — is
// owned by its own endpoint. Building a whole store.Settings literal and
// writing it back makes those columns the form's responsibility by accident:
// a column nobody remembered to copy across is wiped on every save, silently,
// for as long as it takes someone to notice. cloud_cred_sets was exactly that
// (every named S3/REST credential set disappeared the next time any settings
// field was saved from any tab).
//
// These tests pin the columns the form does not own, so the same omission
// cannot come back the next time a column is added.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// loginCookie sets a login password through the real endpoint and logs in,
// returning the session cookie. Used so the assertions below can run with auth
// ENABLED — the stored password hash is the single most damaging column a
// settings save could revert (reverting it turns authentication back off).
func loginCookie(t *testing.T, h http.Handler, password string) *http.Cookie {
	t.Helper()
	if _, m := doJSON(t, h, http.MethodPost, "/api/auth/password", `{"password":"`+password+`"}`); m["ok"] != true {
		t.Fatalf("set password: %v", m)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"password":"`+password+`"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	for _, c := range w.Result().Cookies() {
		if c.Value != "" {
			return c
		}
	}
	t.Fatalf("login did not set a session cookie: %s", w.Body.String())
	return nil
}

// putWithCookie is doJSON for a request that must carry a session cookie.
func putWithCookie(t *testing.T, h http.Handler, path, body string, c *http.Cookie) map[string]any {
	t.Helper()
	r := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(c)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("put %s: status %d body %s", path, w.Code, w.Body.String())
	}
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("put %s: status %d body %s", path, w.Code, w.Body.String())
	}
	return m
}

// A minimal, valid settings form — the shape any card on the Settings page
// PUTs. It names none of the columns asserted below.
const settingsFormBody = `{
	"containersPath": "backups/c",
	"vmsPath": "backups/v",
	"flashPath": "backups/f",
	"containersSchedule": "daily 02:30",
	"vmsSchedule": "off",
	"flashSchedule": "off"
}`

// TestSettingsSaveKeepsNamedCloudCredentialSets is the regression proof: named
// credential sets survive a plain settings save. They are managed by
// POST /api/cloud/creds-sets and are not part of the settings form at all.
func TestSettingsSaveKeepsNamedCloudCredentialSets(t *testing.T) {
	h, st, svc := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})

	if err := svc.SetCloudCredSets([]api.CloudCredSet{{
		ID: "set-1", Name: "Hetzner",
		CloudCreds: api.CloudCreds{S3KeyID: "AKIA-EXAMPLE", S3Secret: "s3cr3t"},
	}}); err != nil {
		t.Fatalf("seed credential set: %v", err)
	}
	before, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if before.CloudCredSets == "" {
		t.Fatal("precondition: the credential set was not stored")
	}

	w, m := doJSON(t, h, http.MethodPut, "/api/settings", settingsFormBody)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("put status = %d body = %s", w.Code, w.Body.String())
	}

	after, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if after.CloudCredSets != before.CloudCredSets {
		t.Fatalf("cloud_cred_sets = %q after a settings save (was %q) — saving unrelated settings destroyed every named credential set",
			after.CloudCredSets, before.CloudCredSets)
	}
	sets, err := svc.CloudCredSets()
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 1 || sets[0].Name != "Hetzner" {
		t.Fatalf("credential sets after a settings save = %+v, want the seeded one", sets)
	}
}

// TestSettingsSaveKeepsInstanceOwnedColumns covers the rest of the row the form
// does not own. Each of these has its own endpoint, so a settings save must
// leave every one of them exactly as it found it.
func TestSettingsSaveKeepsInstanceOwnedColumns(t *testing.T) {
	h, st, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})

	if _, err := st.MutateSettings(func(s *store.Settings) error {
		s.SessionEpoch = "0123456789abcdef0123456789abcdef"
		s.RcloneConf = "rclone-blob"
		s.NotifyConf = "notify-blob"
		s.CloudConf = "cloud-blob"
		s.RegistryAuths = "registry-blob"
		s.MetricsToken = "metrics-token"
		s.WidgetToken = "widget-token"
		s.FleetToken = "fleet-token"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Auth ON, so the password hash under test is a real one and the save below
	// is a real logged-in request. (Seeded first: the session token is bound to
	// the epoch, so rotating the epoch afterwards would revoke the cookie.)
	cookie := loginCookie(t, h, "correct horse battery staple")

	seeded, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if seeded.AuthPasswordHash == "" {
		t.Fatal("precondition: the login password was not stored")
	}

	putWithCookie(t, h, "/api/settings", settingsFormBody, cookie)

	after, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, want, got string }{
		{"AuthPasswordHash", seeded.AuthPasswordHash, after.AuthPasswordHash},
		{"SessionEpoch", seeded.SessionEpoch, after.SessionEpoch},
		{"RcloneConf", seeded.RcloneConf, after.RcloneConf},
		{"NotifyConf", seeded.NotifyConf, after.NotifyConf},
		{"CloudConf", seeded.CloudConf, after.CloudConf},
		{"RegistryAuths", seeded.RegistryAuths, after.RegistryAuths},
		{"MetricsToken", seeded.MetricsToken, after.MetricsToken},
		{"WidgetToken", seeded.WidgetToken, after.WidgetToken},
		{"FleetToken", seeded.FleetToken, after.FleetToken},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q after a settings save, want %q — the form wrote a column it does not own", c.name, c.got, c.want)
		}
	}
	// …and the fields the form DOES own really were saved.
	if after.ContainersPath != "backups/c" || after.ContainersSchedule != "daily 02:30" {
		t.Fatalf("the form's own fields were not saved: %+v", after)
	}
}

// rotatingSSH is an api.HostSSH whose connection test runs a hook once. It is
// the seam that makes the settings-save window deterministic: the VM SSH test
// is the ONLY thing handlePutSettings does between reading its pre-transaction
// snapshot and opening the write transaction, so a hook there lands exactly
// where a save from another tab would.
type rotatingSSH struct {
	onTest func()
	fired  bool
}

var _ api.HostSSH = (*rotatingSSH)(nil)

func (s *rotatingSSH) Test(context.Context) error {
	if !s.fired {
		s.fired = true
		s.onTest()
	}
	return nil
}
func (s *rotatingSSH) EnsureKnownHost(context.Context) error            { return nil }
func (s *rotatingSSH) ReadFile(context.Context, string) ([]byte, error) { return nil, nil }
func (s *rotatingSSH) WriteFile(context.Context, string, []byte) error  { return nil }
func (s *rotatingSSH) PublicKey() (string, error)                       { return "", nil }
func (s *rotatingSSH) Run(context.Context, ...string) (string, error)   { return "", nil }
func (s *rotatingSSH) StreamCommand(context.Context, ...string) (io.ReadCloser, func() error, error) {
	return io.NopCloser(strings.NewReader("")), func() error { return nil }, nil
}
func (s *rotatingSSH) RunWithStdin(context.Context, io.Reader, ...string) error { return nil }

// TestSettingsSaveMergesRegistryTokensAgainstTheCurrentRow pins the merge path
// the test above never reaches (its form body names no registryAuths at all,
// so it only exercises the nil "keep the stored blob" branch).
//
// A registry token is write-only: the GET sends tokenSet and a blank token, so
// EVERY tab's baseline carries blanks for the stored hosts, and the SPA PUTs a
// full settings object — meaning this merge runs on every save from every card.
// Resolving those blanks against a snapshot read BEFORE the transaction means
// any save that started before a token rotation silently reverts it.
func TestSettingsSaveMergesRegistryTokensAgainstTheCurrentRow(t *testing.T) {
	h, st, svc := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})

	seed := func(token string) {
		blob, err := svc.EncodeRegistryAuths([]api.RegistryAuth{
			{Host: "ghcr.io", Username: "owner", Token: token},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.MutateSettings(func(s *store.Settings) error {
			s.RegistryAuths = blob
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	seed("old-token")

	// The concurrent save: it rotates the stored token after this request has
	// read its snapshot but before the request writes.
	svc.SetHostSSH(&rotatingSSH{onTest: func() { seed("rotated-token") }})

	// vmsEnabled flips OFF→ON, which is what makes the request run the SSH test.
	// The registry row carries a BLANK token, exactly as the UI submits it.
	body := `{
		"containersPath": "backups/c",
		"vmsPath": "backups/v",
		"flashPath": "backups/f",
		"containersSchedule": "daily 02:30",
		"vmsSchedule": "off",
		"flashSchedule": "off",
		"vmsEnabled": true,
		"registryAuths": [{"host": "ghcr.io", "username": "owner", "tokenSet": true}]
	}`
	w, m := doJSON(t, h, http.MethodPut, "/api/settings", body)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("put status = %d body = %s", w.Code, w.Body.String())
	}

	auths, err := svc.RegistryAuths()
	if err != nil {
		t.Fatal(err)
	}
	if len(auths) != 1 {
		t.Fatalf("registry auths = %+v, want exactly the one seeded host", auths)
	}
	if auths[0].Token != "rotated-token" {
		t.Fatalf("registry token = %q, want %q — the blank token was resolved against a stale snapshot, so an unrelated save reverted the rotation",
			auths[0].Token, "rotated-token")
	}
}

// TestSettingsSaveRejectsAnInvalidRegistryHostVerbatim pins the one merge
// failure whose message must reach the client unscrubbed: the rejected host
// contains "/", which the path scrubber would otherwise eat, leaving the user
// with a complaint that no longer names what it rejected.
func TestSettingsSaveRejectsAnInvalidRegistryHostVerbatim(t *testing.T) {
	h, _, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})

	body := `{
		"containersPath": "backups/c",
		"containersSchedule": "off",
		"registryAuths": [{"host": "ghcr.io/owner/img", "username": "owner", "token": "t"}]
	}`
	w, m := doJSON(t, h, http.MethodPut, "/api/settings", body)
	if w.Code != http.StatusOK || m["ok"] != false {
		t.Fatalf("put status = %d body = %s, want a graceful refusal", w.Code, w.Body.String())
	}
	if msg, _ := m["error"].(string); !strings.Contains(msg, "ghcr.io/owner/img") {
		t.Fatalf("error = %q, want the rejected host spelled out in full", msg)
	}
}
