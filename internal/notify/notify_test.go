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
	notify.Send(context.Background(), cfg, notify.Event{OK: true}) // success under failure-policy → no send
	if hits != 0 {
		t.Fatalf("success under failure-policy should not send, hits=%d", hits)
	}
	notify.Send(context.Background(), cfg, notify.Event{OK: false}) // failure → send
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
		notify.Event{Title: "BombVault", Message: "hi", OK: true})
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
		notify.Event{Title: "BombVault", Message: "done", OK: true})
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
		notify.Event{OK: false})
	if !strings.HasSuffix(path, "/fail") {
		t.Fatalf("healthchecks failure should hit /fail, got %q", path)
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
