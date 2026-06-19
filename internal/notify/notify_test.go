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
