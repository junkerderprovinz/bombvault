package notify_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/notify"
)

func TestSendRespectsPolicy(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	defer srv.Close()

	cfg := notify.Config{On: "failure", WebhookURL: srv.URL, WebhookFormat: "generic"}
	notify.Send(context.Background(), cfg, "", notify.Event{OK: true}) // success under failure-policy → no send
	if hits != 0 {
		t.Fatalf("success under failure-policy should not send, hits=%d", hits)
	}
	notify.Send(context.Background(), cfg, "", notify.Event{OK: false}) // failure → send
	if hits != 1 {
		t.Fatalf("failure under failure-policy should send once, hits=%d", hits)
	}
}

func TestWebhookDiscordPayload(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
	}))
	defer srv.Close()
	notify.Send(context.Background(),
		notify.Config{On: "always", WebhookURL: srv.URL, WebhookFormat: "discord"},
		"", notify.Event{Title: "BombVault", Message: "hi", OK: true})
	if got["content"] != "BombVault: hi" {
		t.Fatalf("discord content = %v", got["content"])
	}
}

func TestMatrixEndpointAndAuth(t *testing.T) {
	var path, auth string
	var body map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.Path, r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
	}))
	defer srv.Close()
	notify.Send(context.Background(),
		notify.Config{On: "always", MatrixHomeserver: srv.URL, MatrixToken: "tok", MatrixRoom: "!room:hs"},
		"", notify.Event{Title: "BombVault", Message: "done", OK: true})
	if !strings.HasPrefix(path, "/_matrix/client/v3/rooms/") || !strings.Contains(path, "/send/m.room.message/") {
		t.Fatalf("matrix path = %q", path)
	}
	if auth != "Bearer tok" {
		t.Fatalf("matrix auth = %q", auth)
	}
	if body["msgtype"] != "m.text" || body["body"] != "BombVault: done" {
		t.Fatalf("matrix body = %v", body)
	}
}

func TestHealthchecksFailEndpoint(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { path = r.URL.Path }))
	defer srv.Close()
	notify.Send(context.Background(),
		notify.Config{On: "always", HealthchecksURL: srv.URL},
		"", notify.Event{OK: false})
	if !strings.HasSuffix(path, "/fail") {
		t.Fatalf("healthchecks failure should hit /fail, got %q", path)
	}
}

// TestSendHealthchecksSuccessDecoupledFromPolicy: Healthchecks is a monitor, not a
// human message — a successful backup must ping the base URL (keeping the check
// green) even under On=failure, while the webhook message channel stays suppressed.
func TestSendHealthchecksSuccessDecoupledFromPolicy(t *testing.T) {
	var hcPath string
	hc := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { hcPath = r.URL.Path }))
	defer hc.Close()
	var webhookHits int
	wh := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { webhookHits++ }))
	defer wh.Close()

	notify.Send(context.Background(),
		notify.Config{On: "failure", HealthchecksURL: hc.URL, WebhookURL: wh.URL, WebhookFormat: "generic"},
		"", notify.Event{OK: true})

	if hcPath != "/" {
		t.Fatalf("success ping should hit the base path, got %q", hcPath)
	}
	if webhookHits != 0 {
		t.Fatalf("webhook must stay suppressed on success under failure-policy, hits=%d", webhookHits)
	}
}

// TestSendHealthchecksFailPathUnderFailurePolicy: a failed backup pings /fail under
// the failure policy (and the message channels fire too, but here we assert the
// Healthchecks lifecycle path).
func TestSendHealthchecksFailPathUnderFailurePolicy(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { path = r.URL.Path }))
	defer srv.Close()
	notify.Send(context.Background(),
		notify.Config{On: "failure", HealthchecksURL: srv.URL},
		"", notify.Event{OK: false})
	if path != "/fail" {
		t.Fatalf("failure should hit /fail, got %q", path)
	}
}

// TestSendStartPingsStart: SendStart pings the /start endpoint when a URL is set and
// notifications are not "never".
func TestSendStartPingsStart(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { path = r.URL.Path }))
	defer srv.Close()
	notify.SendStart(context.Background(), notify.Config{On: "failure", HealthchecksURL: srv.URL}, "")
	if path != "/start" {
		t.Fatalf("SendStart should hit /start, got %q", path)
	}
}

// TestSendStartSuppressed: SendStart is a no-op when notifications are "never" or no
// Healthchecks URL is configured.
func TestSendStartSuppressed(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	defer srv.Close()

	notify.SendStart(context.Background(), notify.Config{On: "never", HealthchecksURL: srv.URL}, "")
	if hits != 0 {
		t.Fatalf("SendStart under On=never should not ping, hits=%d", hits)
	}
	notify.SendStart(context.Background(), notify.Config{On: "always"}, "") // no URL
	if hits != 0 {
		t.Fatalf("SendStart with no URL should not ping, hits=%d", hits)
	}
}

// TestSendUnknownPolicySuppressed: an unrecognized On value must be treated like
// "never" (positive allowlist matching shouldSend), so neither Send nor SendStart
// contacts any endpoint — including the Healthchecks monitor.
func TestSendUnknownPolicySuppressed(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	defer srv.Close()

	cfg := notify.Config{On: "bogus", HealthchecksURL: srv.URL, WebhookURL: srv.URL, WebhookFormat: "generic"}
	notify.Send(context.Background(), cfg, "", notify.Event{OK: true})
	notify.Send(context.Background(), cfg, "", notify.Event{OK: false})
	notify.SendStart(context.Background(), cfg, "")
	if hits != 0 {
		t.Fatalf("unknown On policy must ping nothing, hits=%d", hits)
	}
}

// TestHealthchecksPhasePaths exercises the phase→path mapping through the exported
// API: /start (SendStart), base (Send success) and /fail (Send failure).
func TestHealthchecksPhasePaths(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { path = r.URL.Path }))
	defer srv.Close()
	base := notify.Config{On: "always", HealthchecksURL: srv.URL}

	notify.SendStart(context.Background(), base, "")
	if path != "/start" {
		t.Fatalf("start phase → %q, want /start", path)
	}
	notify.Send(context.Background(), base, "", notify.Event{OK: true})
	if path != "/" {
		t.Fatalf("success phase → %q, want /", path)
	}
	notify.Send(context.Background(), base, "", notify.Event{OK: false})
	if path != "/fail" {
		t.Fatalf("fail phase → %q, want /fail", path)
	}
}

// TestSendPerDomainURLRouted: a domain with its own Healthchecks URL pings ONLY that
// check, never the global one — the per-domain URL replaces the global for that domain.
func TestSendPerDomainURLRouted(t *testing.T) {
	var flashHits, globalHits int
	flash := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { flashHits++ }))
	defer flash.Close()
	global := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { globalHits++ }))
	defer global.Close()

	cfg := notify.Config{
		On:                   "always",
		HealthchecksURL:      global.URL,
		HealthchecksByDomain: map[string]string{"flash": flash.URL},
	}
	notify.Send(context.Background(), cfg, "flash", notify.Event{OK: true})
	if flashHits != 1 {
		t.Fatalf("flash per-domain URL should be pinged once, hits=%d", flashHits)
	}
	if globalHits != 0 {
		t.Fatalf("global URL must not be pinged for a per-domain-routed domain, hits=%d", globalHits)
	}
}

// TestSendUnmappedDomainFallsBackToGlobal: a domain with no per-domain entry pings the
// global Healthchecks URL.
func TestSendUnmappedDomainFallsBackToGlobal(t *testing.T) {
	var flashHits, globalHits int
	flash := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { flashHits++ }))
	defer flash.Close()
	global := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { globalHits++ }))
	defer global.Close()

	cfg := notify.Config{
		On:                   "always",
		HealthchecksURL:      global.URL,
		HealthchecksByDomain: map[string]string{"flash": flash.URL},
	}
	notify.Send(context.Background(), cfg, "config", notify.Event{OK: true})
	if globalHits != 1 {
		t.Fatalf("unmapped domain should ping the global URL once, hits=%d", globalHits)
	}
	if flashHits != 0 {
		t.Fatalf("the flash per-domain URL must not be pinged for the config domain, hits=%d", flashHits)
	}
}

// TestSendStartPerDomainURL: SendStart routes the /start ping to the domain's own
// Healthchecks URL when one is configured.
func TestSendStartPerDomainURL(t *testing.T) {
	var flashPath string
	var globalHits int
	flash := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { flashPath = r.URL.Path }))
	defer flash.Close()
	global := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { globalHits++ }))
	defer global.Close()

	cfg := notify.Config{
		On:                   "always",
		HealthchecksURL:      global.URL,
		HealthchecksByDomain: map[string]string{"flash": flash.URL},
	}
	notify.SendStart(context.Background(), cfg, "flash")
	if flashPath != "/start" {
		t.Fatalf("SendStart should hit the flash /start, got %q", flashPath)
	}
	if globalHits != 0 {
		t.Fatalf("global URL must not be pinged when the domain has its own URL, hits=%d", globalHits)
	}
}

// TestSendTestPingsEveryDistinctURL: SendTest pings the global URL plus each distinct
// per-domain URL exactly once (de-duplicated across recorders).
func TestSendTestPingsEveryDistinctURL(t *testing.T) {
	var globalHits, flashHits, configHits int
	global := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { globalHits++ }))
	defer global.Close()
	flash := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { flashHits++ }))
	defer flash.Close()
	config := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { configHits++ }))
	defer config.Close()

	cfg := notify.Config{
		HealthchecksURL: global.URL,
		HealthchecksByDomain: map[string]string{
			"flash":     flash.URL,
			"config":    config.URL,
			"container": global.URL, // duplicate of the global URL → must not double-ping
		},
	}
	if err := notify.SendTest(context.Background(), cfg); err != nil {
		t.Fatalf("SendTest: %v", err)
	}
	if globalHits != 1 || flashHits != 1 || configHits != 1 {
		t.Fatalf("each distinct URL should be pinged once: global=%d flash=%d config=%d", globalHits, flashHits, configHits)
	}
}

// TestConfiguredWithOnlyPerDomainURL: a config whose only Healthchecks setting is a
// per-domain URL still counts as configured.
func TestConfiguredWithOnlyPerDomainURL(t *testing.T) {
	cfg := notify.Config{HealthchecksByDomain: map[string]string{"flash": "https://hc/flash"}}
	if !cfg.Configured() {
		t.Fatal("Configured() should be true when only a per-domain Healthchecks URL is set")
	}
}

func TestSendTestNoChannel(t *testing.T) {
	if err := notify.SendTest(context.Background(), notify.Config{}); err == nil {
		t.Fatal("SendTest with no channel should error")
	}
}

// TestSendTestSMTPDisabledSkips: an SMTP block with SMTPEnabled=false is not a
// configured channel, so SendTest reports "no channel" rather than dialing.
func TestSendTestSMTPDisabledSkips(t *testing.T) {
	cfg := notify.Config{SMTPHost: "smtp.example.com", SMTPFrom: "a@x.com", SMTPTo: "b@x.com"} // SMTPEnabled false
	if err := notify.SendTest(context.Background(), cfg); err == nil {
		t.Fatal("SendTest with SMTPEnabled=false should report no channel configured")
	}
}

// TestSendTestSMTPMissingHostErrors: enabling SMTP without a host means the
// channel is not ready, so SendTest still reports nothing configured (no dial).
func TestSendTestSMTPMissingHostErrors(t *testing.T) {
	cfg := notify.Config{SMTPEnabled: true, SMTPFrom: "a@x.com", SMTPTo: "b@x.com"} // no host
	if err := notify.SendTest(context.Background(), cfg); err == nil {
		t.Fatal("SendTest with SMTP enabled but no host should error")
	}
}

// TestSendTestSMTPUnreachableErrorsClearly: a ready SMTP config pointed at a dead
// port surfaces a clear dial error from the SMTP channel.
func TestSendTestSMTPUnreachableErrorsClearly(t *testing.T) {
	cfg := notify.Config{
		SMTPEnabled: true,
		SMTPHost:    "127.0.0.1",
		SMTPPort:    1, // nothing listens here
		SMTPFrom:    "a@x.com",
		SMTPTo:      "b@x.com",
		SMTPTLS:     "none",
	}
	err := notify.SendTest(context.Background(), cfg)
	if err == nil {
		t.Fatal("SendTest against an unreachable SMTP server should error")
	}
	if !strings.Contains(err.Error(), "smtp:") {
		t.Fatalf("expected a clear smtp error, got %q", err)
	}
}
