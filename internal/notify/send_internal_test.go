package notify

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func shortSendTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	prev := sendTimeout
	sendTimeout = d
	t.Cleanup(func() { sendTimeout = prev })
}

// stalledServer accepts requests and never answers them.
func stalledServer(t *testing.T) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-release
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	return srv
}

func countingServer(t *testing.T, status int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestAStalledChannelLeavesTheOthersTheirTime(t *testing.T) {
	shortSendTimeout(t, 300*time.Millisecond)
	stalled := stalledServer(t)
	apprise, hits := countingServer(t, http.StatusOK)
	cfg := Config{
		On:              "failure",
		HealthchecksURL: stalled.URL,
		WebhookEnabled:  true, WebhookURL: stalled.URL, WebhookFormat: "generic",
		AppriseEnabled: true, AppriseURL: apprise.URL,
	}

	if !Send(context.Background(), cfg, "container", Event{Title: "BombVault", Message: "Backup of plex FAILED", OK: false}) {
		t.Fatal("Send reports nothing delivered although Apprise answered")
	}
	if hits.Load() != 1 {
		t.Fatalf("apprise got %d messages after a stalled Healthchecks and webhook, want 1", hits.Load())
	}
}

func TestSendReportsWhetherAMessageChannelTookTheEvent(t *testing.T) {
	ok, _ := countingServer(t, http.StatusOK)
	broken, _ := countingServer(t, http.StatusBadGateway)
	alert := Event{Title: "BombVault", Message: "Backups for containers are overdue", OK: false}

	for _, c := range []struct {
		name string
		cfg  Config
		want bool
	}{
		{"one channel answered", Config{On: "failure", WebhookEnabled: true, WebhookURL: broken.URL, AppriseEnabled: true, AppriseURL: ok.URL}, true},
		{"every channel failed", Config{On: "failure", WebhookEnabled: true, WebhookURL: broken.URL, AppriseEnabled: true, AppriseURL: broken.URL}, false},
		{"only a health check", Config{On: "failure", HealthchecksURL: ok.URL}, false},
		{"switched off", Config{On: "never", WebhookEnabled: true, WebhookURL: ok.URL}, false},
	} {
		if got := Send(context.Background(), c.cfg, "containers", alert); got != c.want {
			t.Errorf("%s: Send = %v, want %v", c.name, got, c.want)
		}
	}
}

// A server that takes the connection and never greets, such as an
// implicit-TLS server on port 465 reached in plaintext.
func TestSMTPGivesUpOnAServerThatNeverGreets(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop); _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		<-stop
		_ = conn.Close()
	}()
	cfg := Config{SMTPEnabled: true, SMTPHost: "127.0.0.1", SMTPPort: ln.Addr().(*net.TCPAddr).Port, SMTPFrom: "a@x.com", SMTPTo: "b@x.com"}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- sendSMTP(ctx, cfg, Event{Title: "BombVault", Message: "x"}) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a server that never greets cannot have taken the mail")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("sendSMTP is still waiting for a greeting past its deadline")
	}
}
