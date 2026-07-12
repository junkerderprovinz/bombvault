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

// TestSuppressedHealthchecksSkipsPingNotChannels: a context flagged with
// WithHealthchecksSuppressed folds out the per-call Healthchecks ping in Send and
// SendStart, while the message channels (here a webhook) still fire. This is what a
// scheduled per-domain run sets on each item so the run's ONE aggregate ping
// represents the whole domain (#49).
func TestSuppressedHealthchecksSkipsPingNotChannels(t *testing.T) {
	var hcHits, webhookHits int
	hc := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hcHits++ }))
	defer hc.Close()
	wh := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { webhookHits++ }))
	defer wh.Close()

	cfg := notify.Config{On: "always", HealthchecksURL: hc.URL, WebhookURL: wh.URL, WebhookFormat: "generic"}
	ctx := notify.WithHealthchecksSuppressed(context.Background())

	notify.SendStart(ctx, cfg, "container") // suppressed → no /start ping
	notify.Send(ctx, cfg, "container", notify.Event{Title: "BombVault", Message: "ok", OK: true})

	if hcHits != 0 {
		t.Fatalf("suppressed context must not ping Healthchecks, hits=%d", hcHits)
	}
	if webhookHits != 1 {
		t.Fatalf("message channels must still fire under suppression, webhook hits=%d", webhookHits)
	}
}

// TestPingDomainStartAndResultEndpoints: the aggregate per-domain-run pings hit the
// right lifecycle endpoints — /start for the run start, the base URL (success) when
// every item passed, and /fail when any failed — and carry the summary as the body so
// it shows in the check's event feed.
func TestPingDomainStartAndResultEndpoints(t *testing.T) {
	var path, body string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
	}))
	defer srv.Close()
	cfg := notify.Config{On: "always", HealthchecksURL: srv.URL}

	notify.PingDomainStart(context.Background(), cfg, "containers")
	if path != "/start" {
		t.Fatalf("PingDomainStart should hit /start, got %q", path)
	}

	notify.PingDomainResult(context.Background(), cfg, "containers", true, "3 of 3 items succeeded")
	if path != "/" {
		t.Fatalf("PingDomainResult(ok) should hit the success base path, got %q", path)
	}
	if body != "3 of 3 items succeeded" {
		t.Fatalf("success summary should be sent as the body, got %q", body)
	}

	notify.PingDomainResult(context.Background(), cfg, "containers", false, "1 of 3 items failed")
	if path != "/fail" {
		t.Fatalf("PingDomainResult(!ok) should hit /fail, got %q", path)
	}
	if body != "1 of 3 items failed" {
		t.Fatalf("fail summary should be sent as the body, got %q", body)
	}
}

// TestPingDomainSuppressedWhenNeverOrNoURL: the aggregate pings honour the same gates
// as the rest of the package — a no-op under On=never and when the domain resolves to
// no Healthchecks URL.
func TestPingDomainSuppressedWhenNeverOrNoURL(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	defer srv.Close()

	notify.PingDomainStart(context.Background(), notify.Config{On: "never", HealthchecksURL: srv.URL}, "containers")
	notify.PingDomainResult(context.Background(), notify.Config{On: "never", HealthchecksURL: srv.URL}, "containers", true, "x")
	notify.PingDomainStart(context.Background(), notify.Config{On: "always"}, "containers") // no URL
	notify.PingDomainResult(context.Background(), notify.Config{On: "always"}, "containers", true, "x")
	if hits != 0 {
		t.Fatalf("aggregate pings must be a no-op under never/no-URL, hits=%d", hits)
	}
}

// TestScheduledSummarySuppressesPerItemMessages: with ScheduledSummary on, a
// message marked WithMessagesSuppressed (a scheduled per-item backup) is dropped so
// the ONE domain summary speaks for the run; a non-marked message (the summary
// itself, or a per-updated-container notice) still fires; and with the toggle OFF a
// marked message fires as before (#56).
func TestScheduledSummarySuppressesPerItemMessages(t *testing.T) {
	var hits int
	wh := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	defer wh.Close()
	cfg := notify.Config{On: "always", WebhookURL: wh.URL, WebhookFormat: "generic", ScheduledSummary: true}

	// per-item scheduled message → suppressed in summary mode
	notify.Send(notify.WithMessagesSuppressed(context.Background()), cfg, "container", notify.Event{OK: true})
	if hits != 0 {
		t.Fatalf("summary mode must suppress a per-item message, hits=%d", hits)
	}
	// non-marked message (the summary / an update notice) → still fires
	notify.Send(context.Background(), cfg, "containers", notify.Event{OK: true})
	if hits != 1 {
		t.Fatalf("a non-marked message must still send, hits=%d", hits)
	}
	// toggle off → a marked per-item message fires as before
	cfg.ScheduledSummary = false
	notify.Send(notify.WithMessagesSuppressed(context.Background()), cfg, "container", notify.Event{OK: true})
	if hits != 2 {
		t.Fatalf("with summary off a marked message must send, hits=%d", hits)
	}
}
