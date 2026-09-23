package notify_test

import (
	"context"
	"encoding/json"
	"io"
	"log"
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

	cfg := notify.Config{On: "failure", WebhookEnabled: true, WebhookURL: srv.URL, WebhookFormat: "generic"}
	notify.Send(context.Background(), cfg, "", notify.Event{OK: true})
	if hits != 0 {
		t.Fatalf("success under failure-policy should not send, hits=%d", hits)
	}
	notify.Send(context.Background(), cfg, "", notify.Event{OK: false})
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
		notify.Config{On: "always", WebhookEnabled: true, WebhookURL: srv.URL, WebhookFormat: "discord"},
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
		notify.Config{On: "always", MatrixEnabled: true, MatrixHomeserver: srv.URL, MatrixToken: "tok", MatrixRoom: "!room:hs"},
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

func TestSendStartPingsStart(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { path = r.URL.Path }))
	defer srv.Close()
	notify.SendStart(context.Background(), notify.Config{On: "failure", HealthchecksURL: srv.URL}, "")
	if path != "/start" {
		t.Fatalf("SendStart should hit /start, got %q", path)
	}
}

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

// TestSendUnknownPolicySuppressed checks that an unrecognized On value sends
// nothing, not even the Healthchecks ping.
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
			"container": global.URL, // same as the global URL
		},
	}
	if err := notify.SendTest(context.Background(), cfg); err != nil {
		t.Fatalf("SendTest: %v", err)
	}
	if globalHits != 1 || flashHits != 1 || configHits != 1 {
		t.Fatalf("each distinct URL should be pinged once: global=%d flash=%d config=%d", globalHits, flashHits, configHits)
	}
}

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

func TestSendTestSMTPDisabledSkips(t *testing.T) {
	cfg := notify.Config{SMTPHost: "smtp.example.com", SMTPFrom: "a@x.com", SMTPTo: "b@x.com"} // SMTPEnabled false
	if err := notify.SendTest(context.Background(), cfg); err == nil {
		t.Fatal("SendTest with SMTPEnabled=false should report no channel configured")
	}
}

func TestSendTestSMTPMissingHostErrors(t *testing.T) {
	cfg := notify.Config{SMTPEnabled: true, SMTPFrom: "a@x.com", SMTPTo: "b@x.com"} // no host
	if err := notify.SendTest(context.Background(), cfg); err == nil {
		t.Fatal("SendTest with SMTP enabled but no host should error")
	}
}

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

func TestSendTestWebhookDisabledSkips(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	defer srv.Close()
	cfg := notify.Config{WebhookURL: srv.URL, WebhookFormat: "generic"} // WebhookEnabled false
	if cfg.Configured() {
		t.Fatal("a webhook URL with WebhookEnabled=false must not count as configured")
	}
	if err := notify.SendTest(context.Background(), cfg); err == nil {
		t.Fatal("SendTest with WebhookEnabled=false should report no channel configured")
	}
	if hits != 0 {
		t.Fatalf("a disabled webhook must never be dialed, hits=%d", hits)
	}
}

func TestSendMatrixDisabledSkips(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	defer srv.Close()
	cfg := notify.Config{MatrixHomeserver: srv.URL, MatrixToken: "tok", MatrixRoom: "!room:hs"} // MatrixEnabled false
	if cfg.Configured() {
		t.Fatal("a Matrix block with MatrixEnabled=false must not count as configured")
	}
	notify.Send(context.Background(), notify.Config{On: "always", MatrixHomeserver: srv.URL, MatrixToken: "tok", MatrixRoom: "!room:hs"},
		"", notify.Event{OK: true})
	if hits != 0 {
		t.Fatalf("a disabled Matrix channel must never be posted to, hits=%d", hits)
	}
}

func TestSendTestAppriseDisabledSkips(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	defer srv.Close()
	cfg := notify.Config{AppriseURL: srv.URL} // AppriseEnabled false
	if cfg.Configured() {
		t.Fatal("an Apprise URL with AppriseEnabled=false must not count as configured")
	}
	if err := notify.SendTest(context.Background(), cfg); err == nil {
		t.Fatal("SendTest with AppriseEnabled=false should report no channel configured")
	}
	if hits != 0 {
		t.Fatalf("a disabled Apprise channel must never be posted to, hits=%d", hits)
	}
}

func TestSuppressedHealthchecksSkipsPingNotChannels(t *testing.T) {
	var hcHits, webhookHits int
	hc := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hcHits++ }))
	defer hc.Close()
	wh := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { webhookHits++ }))
	defer wh.Close()

	cfg := notify.Config{On: "always", HealthchecksURL: hc.URL, WebhookEnabled: true, WebhookURL: wh.URL, WebhookFormat: "generic"}
	ctx := notify.WithHealthchecksSuppressed(context.Background())

	notify.SendStart(ctx, cfg, "container")
	notify.Send(ctx, cfg, "container", notify.Event{Title: "BombVault", Message: "ok", OK: true})

	if hcHits != 0 {
		t.Fatalf("suppressed context must not ping Healthchecks, hits=%d", hcHits)
	}
	if webhookHits != 1 {
		t.Fatalf("message channels must still fire under suppression, webhook hits=%d", webhookHits)
	}
}

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

func TestApprisePayloadAndTypeMapping(t *testing.T) {
	var got map[string]any
	var path, contentType string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		path, contentType = r.URL.Path, r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
	}))
	defer srv.Close()
	cfg := notify.Config{On: "always", AppriseEnabled: true, AppriseURL: srv.URL + "/notify/mykey"}

	notify.Send(context.Background(), cfg, "", notify.Event{Title: "BombVault", Message: "backup done", OK: true})
	if path != "/notify/mykey" || contentType != "application/json" {
		t.Fatalf("apprise POST path=%q content-type=%q", path, contentType)
	}
	if got["title"] != "BombVault" || got["body"] != "backup done" || got["type"] != "success" {
		t.Fatalf("success payload = %v", got)
	}
	if _, ok := got["tag"]; ok {
		t.Fatalf("no tag configured must mean no tag key, payload = %v", got)
	}

	notify.Send(context.Background(), cfg, "", notify.Event{Title: "BombVault", Message: "backup failed", OK: false})
	if got["type"] != "failure" {
		t.Fatalf("failure event should map to type=failure, payload = %v", got)
	}
}

func TestAppriseTagPassedThrough(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
	}))
	defer srv.Close()
	notify.Send(context.Background(),
		notify.Config{On: "always", AppriseEnabled: true, AppriseURL: srv.URL, AppriseTags: "backups,homelab"},
		"", notify.Event{Title: "BombVault", Message: "hi", OK: true})
	if got["tag"] != "backups,homelab" {
		t.Fatalf("tag = %v, want backups,homelab", got["tag"])
	}
}

func TestAppriseStartPostsInfo(t *testing.T) {
	var got map[string]any
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		hits++
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
	}))
	defer srv.Close()

	notify.SendStart(context.Background(), notify.Config{On: "always", AppriseEnabled: true, AppriseURL: srv.URL}, "containers")
	if hits != 1 {
		t.Fatalf("On=always should post an apprise start, hits=%d", hits)
	}
	if got["type"] != "info" || got["body"] != "Backup started (containers)" {
		t.Fatalf("start payload = %v", got)
	}

	notify.SendStart(notify.WithHealthchecksSuppressed(context.Background()),
		notify.Config{On: "always", AppriseEnabled: true, AppriseURL: srv.URL}, "containers")
	if hits != 2 {
		t.Fatalf("HC suppression must not silence the apprise start, hits=%d", hits)
	}

	notify.SendStart(context.Background(), notify.Config{On: "failure", AppriseEnabled: true, AppriseURL: srv.URL}, "containers")
	if hits != 2 {
		t.Fatalf("On=failure must not post a start notice, hits=%d", hits)
	}
}

func TestAppriseScheduledSummarySuppression(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	defer srv.Close()
	cfg := notify.Config{On: "always", AppriseEnabled: true, AppriseURL: srv.URL, ScheduledSummary: true}
	ictx := notify.WithMessagesSuppressed(context.Background())

	notify.Send(ictx, cfg, "container", notify.Event{OK: true})
	notify.SendStart(ictx, cfg, "container")
	if hits != 0 {
		t.Fatalf("summary mode must suppress per-item apprise sends, hits=%d", hits)
	}
	// The summary itself is not marked.
	notify.Send(context.Background(), cfg, "containers", notify.Event{OK: true})
	if hits != 1 {
		t.Fatalf("a non-marked apprise message must still send, hits=%d", hits)
	}
	cfg.ScheduledSummary = false
	notify.Send(ictx, cfg, "container", notify.Event{OK: true})
	if hits != 2 {
		t.Fatalf("with summary off a marked apprise message must send, hits=%d", hits)
	}
}

func TestAppriseSendErrorRedactsURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadURL := srv.URL + "/notify/secretkey"
	srv.Close() // makes the send fail with a *url.Error that carries the URL

	var buf strings.Builder
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	notify.Send(context.Background(),
		notify.Config{On: "always", AppriseEnabled: true, AppriseURL: deadURL},
		"", notify.Event{Title: "BombVault", Message: "x", OK: true})

	if !strings.Contains(buf.String(), "notify: apprise:") {
		t.Fatalf("failed apprise send should be logged, log=%q", buf.String())
	}
	if strings.Contains(buf.String(), "secretkey") {
		t.Fatalf("logged apprise error must not leak the notify key, log=%q", buf.String())
	}
}

func TestConfiguredWithOnlyAppriseURL(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
	}))
	defer srv.Close()
	cfg := notify.Config{AppriseEnabled: true, AppriseURL: srv.URL}
	if !cfg.Configured() {
		t.Fatal("Configured() should be true with an enabled Apprise URL")
	}
	if err := notify.SendTest(context.Background(), cfg); err != nil {
		t.Fatalf("SendTest: %v", err)
	}
	if got["type"] != "success" || got["title"] == "" {
		t.Fatalf("test payload = %v", got)
	}
}

func TestScheduledSummarySuppressesPerItemMessages(t *testing.T) {
	var hits int
	wh := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	defer wh.Close()
	cfg := notify.Config{On: "always", WebhookEnabled: true, WebhookURL: wh.URL, WebhookFormat: "generic", ScheduledSummary: true}

	notify.Send(notify.WithMessagesSuppressed(context.Background()), cfg, "container", notify.Event{OK: true})
	if hits != 0 {
		t.Fatalf("summary mode must suppress a per-item message, hits=%d", hits)
	}
	// The summary and container update notices are not marked.
	notify.Send(context.Background(), cfg, "containers", notify.Event{OK: true})
	if hits != 1 {
		t.Fatalf("a non-marked message must still send, hits=%d", hits)
	}
	cfg.ScheduledSummary = false
	notify.Send(notify.WithMessagesSuppressed(context.Background()), cfg, "container", notify.Event{OK: true})
	if hits != 2 {
		t.Fatalf("with summary off a marked message must send, hits=%d", hits)
	}
}

// TestSendOnUnsetNotifiesFailures covers the empty On a fresh install stores,
// which must behave like "failure".
func TestSendOnUnsetNotifiesFailures(t *testing.T) {
	cases := []struct {
		name     string
		on       string
		ok       bool
		wantSent bool
	}{
		{"unset sends a failure", "", false, true},
		{"unset stays quiet on success", "", true, false},
		{"explicit never stays silent on failure", "never", false, false},
		{"explicit never stays silent on success", "never", true, false},
		{"failure sends a failure", "failure", false, true},
		{"always sends a success too", "always", true, true},
		{"an unknown value stays silent", "wat", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var hits int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits++
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()
			notify.Send(context.Background(),
				notify.Config{On: c.on, WebhookEnabled: true, WebhookURL: srv.URL, WebhookFormat: "generic"},
				"", notify.Event{Title: "t", Message: "m", OK: c.ok})
			if got := hits > 0; got != c.wantSent {
				t.Fatalf("On=%q ok=%v: sent=%v, want %v", c.on, c.ok, got, c.wantSent)
			}
		})
	}
}
