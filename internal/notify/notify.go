// Package notify sends best-effort backup notifications to optional channels: a
// webhook (generic JSON, Discord, Slack, Gotify or ntfy), a Matrix room, and a
// Healthchecks.io ping. Every send is best-effort and time-bounded; a failure to
// notify never affects a backup. URLs/tokens are admin-configured, so reaching
// internal endpoints is intentional (no SSRF filtering).
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const sendTimeout = 15 * time.Second

// Config holds the notification channels. An empty field disables that channel.
type Config struct {
	On               string `json:"on"` // "never" | "failure" | "always"
	WebhookURL       string `json:"webhookUrl"`
	WebhookFormat    string `json:"webhookFormat"` // generic|discord|slack|gotify|ntfy
	MatrixHomeserver string `json:"matrixHomeserver"`
	MatrixToken      string `json:"matrixToken"`
	MatrixRoom       string `json:"matrixRoom"`
	HealthchecksURL  string `json:"healthchecksUrl"`
	// Unraid sends each event to Unraid's native notification system (which can
	// itself forward to Pushover/email/Discord/…). It is delivered over SSH by the
	// service layer (the host's notify script), not by this package's HTTP Send.
	Unraid bool `json:"unraid"`
}

// Event is a completed backup, rendered into each channel's message.
type Event struct {
	Title   string
	Message string
	OK      bool
}

func (c Config) shouldSend(ok bool) bool {
	switch c.On {
	case "always":
		return true
	case "failure":
		return !ok
	default: // "never" or unset
		return false
	}
}

// Configured reports whether at least one channel is set.
func (c Config) Configured() bool {
	return c.WebhookURL != "" || c.matrixReady() || c.HealthchecksURL != ""
}

func (c Config) matrixReady() bool {
	return c.MatrixHomeserver != "" && c.MatrixToken != "" && c.MatrixRoom != ""
}

// Send dispatches ev to every configured channel, honouring the On policy. Each
// channel's error is logged, never returned (best-effort).
func Send(ctx context.Context, c Config, ev Event) {
	if !c.shouldSend(ev.OK) {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	client := &http.Client{Timeout: sendTimeout}

	if c.WebhookURL != "" {
		if err := sendWebhook(ctx, client, c, ev); err != nil {
			log.Printf("notify: webhook: %v", err)
		}
	}
	if c.matrixReady() {
		if err := sendMatrix(ctx, client, c, ev); err != nil {
			log.Printf("notify: matrix: %v", err)
		}
	}
	if c.HealthchecksURL != "" {
		if err := pingHealthchecks(ctx, client, c.HealthchecksURL, ev.OK); err != nil {
			log.Printf("notify: healthchecks: %v", err)
		}
	}
}

// SendTest sends a fixed test event to every configured channel (ignoring the On
// policy) and returns the first error so the UI can explain a failed test.
func SendTest(ctx context.Context, c Config) error {
	if !c.Configured() {
		return fmt.Errorf("no notification channel configured")
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	client := &http.Client{Timeout: sendTimeout}
	ev := Event{Title: "BombVault", Message: "Test notification — notifications are working.", OK: true}

	if c.WebhookURL != "" {
		if err := sendWebhook(ctx, client, c, ev); err != nil {
			return fmt.Errorf("webhook: %w", err)
		}
	}
	if c.matrixReady() {
		if err := sendMatrix(ctx, client, c, ev); err != nil {
			return fmt.Errorf("matrix: %w", err)
		}
	}
	if c.HealthchecksURL != "" {
		if err := pingHealthchecks(ctx, client, c.HealthchecksURL, true); err != nil {
			return fmt.Errorf("healthchecks: %w", err)
		}
	}
	return nil
}

func sendWebhook(ctx context.Context, client *http.Client, c Config, ev Event) error {
	text := ev.Title + ": " + ev.Message
	switch c.WebhookFormat {
	case "ntfy":
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.WebhookURL, strings.NewReader(ev.Message))
		if err != nil {
			return err
		}
		req.Header.Set("X-Title", ev.Title)
		if !ev.OK {
			req.Header.Set("Priority", "high")
			req.Header.Set("Tags", "warning")
		}
		return do(client, req)
	case "discord":
		return postJSON(ctx, client, c.WebhookURL, map[string]string{"content": text})
	case "slack":
		return postJSON(ctx, client, c.WebhookURL, map[string]string{"text": text})
	case "gotify":
		prio := 5
		if !ev.OK {
			prio = 8
		}
		return postJSON(ctx, client, c.WebhookURL, map[string]any{"title": ev.Title, "message": ev.Message, "priority": prio})
	default: // generic
		return postJSON(ctx, client, c.WebhookURL, map[string]any{"title": ev.Title, "message": ev.Message, "ok": ev.OK})
	}
}

// sendMatrix posts an m.text message to a room via the client-server API
// (PUT .../send/m.room.message/{txnId}). The token goes in the Authorization
// header, never in the URL.
func sendMatrix(ctx context.Context, client *http.Client, c Config, ev Event) error {
	txn := strconv.FormatInt(time.Now().UnixNano(), 10)
	endpoint := strings.TrimRight(c.MatrixHomeserver, "/") +
		"/_matrix/client/v3/rooms/" + url.PathEscape(c.MatrixRoom) +
		"/send/m.room.message/" + txn
	body, err := json.Marshal(map[string]string{"msgtype": "m.text", "body": ev.Title + ": " + ev.Message})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.MatrixToken)
	return do(client, req)
}

// pingHealthchecks pings the check URL (success) or its /fail endpoint (failure).
func pingHealthchecks(ctx context.Context, client *http.Client, base string, ok bool) error {
	u := strings.TrimRight(base, "/")
	if !ok {
		u += "/fail"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	return do(client, req)
}

func postJSON(ctx context.Context, client *http.Client, endpoint string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return do(client, req)
}

func do(client *http.Client, req *http.Request) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // response body close error is not actionable
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("endpoint returned HTTP %d", resp.StatusCode)
	}
	return nil
}
