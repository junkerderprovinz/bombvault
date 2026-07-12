// Package notify sends best-effort backup notifications to optional channels: a
// webhook (generic JSON, Discord, Slack, Gotify or ntfy), a Matrix room, an
// email (SMTP), and a Healthchecks.io ping. Every send is best-effort and
// time-bounded; a failure to notify never affects a backup. URLs/tokens are
// admin-configured, so reaching internal endpoints is intentional (no SSRF
// filtering).
package notify

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// redactErr strips the request URL from a *url.Error before it is logged: a
// webhook (Discord/Slack/Gotify/ntfy) or Healthchecks URL carries its secret
// token in the path, and the default *url.Error string prints the full URL. The
// underlying cause (timeout, connection refused, …) is preserved.
func redactErr(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("%s: %w", ue.Op, ue.Err)
	}
	return err
}

const sendTimeout = 15 * time.Second

// hcSuppressKey is the context key that suppresses the per-call Healthchecks ping
// in Send and SendStart while leaving every message channel (webhook/Matrix/SMTP)
// untouched. A SCHEDULED per-domain run sets it on each item's context so the many
// per-item Healthchecks pings collapse into ONE aggregate start/success/fail ping
// for the whole domain job (see PingDomainStart / PingDomainResult and #49).
type hcSuppressKey struct{}

// WithHealthchecksSuppressed returns a child context that suppresses the
// Healthchecks ping in Send and SendStart. ONLY the Healthchecks ping is affected;
// every other channel still fires per call. Used by scheduled per-domain runs,
// which ping Healthchecks once for the whole run instead of once per item.
func WithHealthchecksSuppressed(ctx context.Context) context.Context {
	return context.WithValue(ctx, hcSuppressKey{}, true)
}

// healthchecksSuppressed reports whether ctx carries the suppress flag.
func healthchecksSuppressed(ctx context.Context) bool {
	v, _ := ctx.Value(hcSuppressKey{}).(bool)
	return v
}

// msgSuppressKey is the context key that marks a per-item MESSAGE send (webhook/
// Matrix/SMTP) as a candidate for domain-level summarisation. A scheduled per-domain
// run sets it on each backup item; Send then skips the per-item message ONLY when the
// config also asks for a summary (Config.ScheduledSummary), so the many per-item
// messages collapse into ONE "N of M" summary for the whole run (#56). It is separate
// from hcSuppressKey so a per-container update notice (which suppresses only the HC
// ping) still delivers its message. See WithMessagesSuppressed.
type msgSuppressKey struct{}

// WithMessagesSuppressed marks ctx as a per-item scheduled-backup message: Send drops
// the per-item webhook/Matrix/SMTP send when the config's ScheduledSummary is on.
func WithMessagesSuppressed(ctx context.Context) context.Context {
	return context.WithValue(ctx, msgSuppressKey{}, true)
}

// MessagesSuppressed reports whether ctx is a per-item scheduled-backup message. The
// service layer reads it to drop its own per-item Unraid push in summary mode.
func MessagesSuppressed(ctx context.Context) bool {
	v, _ := ctx.Value(msgSuppressKey{}).(bool)
	return v
}

// Config holds the notification channels. An empty field disables that channel.
type Config struct {
	On               string `json:"on"` // "never" | "failure" | "always"
	WebhookURL       string `json:"webhookUrl"`
	WebhookFormat    string `json:"webhookFormat"` // generic|discord|slack|gotify|ntfy
	MatrixHomeserver string `json:"matrixHomeserver"`
	MatrixToken      string `json:"matrixToken"`
	MatrixRoom       string `json:"matrixRoom"`
	HealthchecksURL  string `json:"healthchecksUrl"`
	// HealthchecksByDomain maps a backup domain ("container"|"VM"|"flash"|"config")
	// to its own Healthchecks check URL. A per-domain URL replaces (does not add to)
	// the global HealthchecksURL for that domain; a blank/absent entry falls back to
	// the global URL.
	HealthchecksByDomain map[string]string `json:"healthchecksByDomain"`
	// Unraid sends each event to Unraid's native notification system (which can
	// itself forward to Pushover/email/Discord/…). It is delivered over SSH by the
	// service layer (the host's notify script), not by this package's HTTP Send.
	Unraid bool `json:"unraid"`
	// SMTP sends each event as a plain-text email via an SMTP server.
	SMTPEnabled  bool   `json:"smtpEnabled"`
	SMTPHost     string `json:"smtpHost"`
	SMTPPort     int    `json:"smtpPort"`
	SMTPUsername string `json:"smtpUsername"`
	SMTPPassword string `json:"smtpPassword"`
	SMTPFrom     string `json:"smtpFrom"`
	SMTPTo       string `json:"smtpTo"`
	SMTPTLS      string `json:"smtpTls"` // "starttls" (default) | "tls" | "none"
	// ScheduledSummary collapses a scheduled per-domain run's per-item messages into
	// ONE "N of M succeeded/failed" summary on the message channels (webhook/Matrix/
	// SMTP/Unraid), instead of one message per container/VM (#56). Off by default, so
	// existing setups keep their per-item notifications. Healthchecks is already
	// aggregated regardless. Manual multi-select backups stay per-item.
	ScheduledSummary bool `json:"scheduledSummary"`
	// NotifyOnUpdate sends a message when a container is updated by the post-backup
	// image update (#52/#56): "Updated <name> to a newer image", so the user can
	// verify it still works. Off by default; fires per updated container (updates are
	// rare) and is NOT folded into the scheduled summary.
	NotifyOnUpdate bool `json:"notifyOnUpdate"`
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

// Configured reports whether at least one channel is set. Healthchecks counts when
// the global URL or any per-domain URL is set.
func (c Config) Configured() bool {
	return c.WebhookURL != "" || c.matrixReady() || len(c.healthchecksURLs()) > 0 || c.smtpReady()
}

// healthchecksURLFor returns the per-domain Healthchecks URL when one is set for
// domain, otherwise the global HealthchecksURL. A per-domain URL replaces (does
// not add to) the global for that domain.
func (c Config) healthchecksURLFor(domain string) string {
	if u := c.HealthchecksByDomain[normalizeHCDomain(domain)]; u != "" {
		return u
	}
	return c.HealthchecksURL
}

// normalizeHCDomain maps the domain spellings used across the codebase to the
// canonical HealthchecksByDomain keys ("container"|"VM"|"flash"|"config"). Backups
// use "container"/"VM"; the off-site and tamper failure notifiers use the plural
// "containers"/"vms". Normalizing here means a per-domain check catches ALL of a
// domain's events (backup + replication/drill/tamper failures), not just backups.
func normalizeHCDomain(domain string) string {
	switch domain {
	case "containers":
		return "container"
	case "vms", "VMs", "vm":
		return "VM"
	default:
		return domain
	}
}

// healthchecksURLs returns every distinct configured Healthchecks URL — the global
// HealthchecksURL plus each non-empty per-domain value — de-duplicated. Used by
// SendTest to ping every check exactly once and by Configured.
func (c Config) healthchecksURLs() []string {
	seen := map[string]bool{}
	var urls []string
	for _, u := range append([]string{c.HealthchecksURL}, mapValues(c.HealthchecksByDomain)...) {
		if u != "" && !seen[u] {
			seen[u] = true
			urls = append(urls, u)
		}
	}
	return urls
}

// mapValues returns m's values in an unspecified order.
func mapValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func (c Config) matrixReady() bool {
	return c.MatrixHomeserver != "" && c.MatrixToken != "" && c.MatrixRoom != ""
}

func (c Config) smtpReady() bool {
	return c.SMTPEnabled && c.SMTPHost != "" && c.SMTPFrom != "" && c.SMTPTo != ""
}

// Send dispatches ev to the configured channels. Healthchecks is a monitor, not a
// human message: it must get the success ping to stay green, so it fires on both
// outcomes whenever configured (except when notifications are "never"). The On
// policy governs only the message channels (webhook/matrix/smtp). Each channel's
// error is logged, never returned (best-effort).
func Send(ctx context.Context, c Config, domain string, ev Event) {
	if c.On != "always" && c.On != "failure" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	client := &http.Client{Timeout: sendTimeout}

	// Healthchecks is a monitor, not a human message: it must get the success ping
	// to stay green, so it fires on both outcomes whenever configured — the On
	// policy governs only the message channels below. The domain selects its own
	// check when one is configured, else the global URL. A scheduled per-domain run
	// suppresses this per-item ping (context flag) so its ONE aggregate ping speaks
	// for the whole run; the message channels below still fire per item.
	if hcURL := c.healthchecksURLFor(domain); hcURL != "" && !healthchecksSuppressed(ctx) {
		phase := "success"
		if !ev.OK {
			phase = "fail"
		}
		if err := pingHealthchecks(ctx, client, hcURL, phase); err != nil {
			log.Printf("notify: healthchecks: %v", redactErr(err))
		}
	}

	if !c.shouldSend(ev.OK) {
		return
	}

	// Scheduled per-domain run with summary mode on: drop this per-item message so the
	// single "N of M" summary (Service.ScheduledNotifyResult) speaks for the whole run.
	if MessagesSuppressed(ctx) && c.ScheduledSummary {
		return
	}

	if c.WebhookURL != "" {
		if err := sendWebhook(ctx, client, c, ev); err != nil {
			log.Printf("notify: webhook: %v", redactErr(err))
		}
	}
	if c.matrixReady() {
		if err := sendMatrix(ctx, client, c, ev); err != nil {
			log.Printf("notify: matrix: %v", redactErr(err))
		}
	}
	if c.smtpReady() {
		if err := sendSMTP(ctx, c, ev); err != nil {
			log.Printf("notify: smtp: %v", err)
		}
	}
}

// SendStart pings the Healthchecks check's /start endpoint at the beginning of a
// backup, so the check can measure duration and detect a hung/never-finished run.
// Healthchecks-only (message channels have no "start" concept); best-effort.
func SendStart(ctx context.Context, c Config, domain string) {
	hcURL := c.healthchecksURLFor(domain)
	// A scheduled per-domain run suppresses this per-item /start (context flag) so the
	// run's ONE aggregate /start (PingDomainStart) speaks for the whole domain job.
	if (c.On != "always" && c.On != "failure") || hcURL == "" || healthchecksSuppressed(ctx) {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	client := &http.Client{Timeout: sendTimeout}
	if err := pingHealthchecks(ctx, client, hcURL, "start"); err != nil {
		log.Printf("notify: healthchecks start: %v", redactErr(err))
	}
}

// PingDomainStart pings a SCHEDULED per-domain run's Healthchecks check /start once,
// at the start of the whole run — the aggregate counterpart to the per-item SendStart
// (which the run suppresses). No-op when the domain has no check configured or
// notifications are off; best-effort. The suppress flag does NOT apply here: this IS
// the aggregate ping. domain may be the plural scheduler spelling ("containers"|"vms");
// healthchecksURLFor normalises it to the per-domain check key.
func PingDomainStart(ctx context.Context, c Config, domain string) {
	hcURL := c.healthchecksURLFor(domain)
	if (c.On != "always" && c.On != "failure") || hcURL == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	client := &http.Client{Timeout: sendTimeout}
	if err := pingHealthchecks(ctx, client, hcURL, "start"); err != nil {
		log.Printf("notify: healthchecks domain start: %v", redactErr(err))
	}
}

// PingDomainResult pings a SCHEDULED per-domain run's Healthchecks check once at the
// end of the whole run: the success endpoint when ok, else <base>/fail. summary (e.g.
// "3 of 3 items succeeded" or "1 of 3 items failed") is POSTed as the request body so
// it shows in the check's event log. It is the aggregate counterpart to the per-item
// success/fail ping inside Send (which the run suppresses). No-op when the domain has
// no check configured or notifications are off; best-effort.
func PingDomainResult(ctx context.Context, c Config, domain string, ok bool, summary string) {
	hcURL := c.healthchecksURLFor(domain)
	if (c.On != "always" && c.On != "failure") || hcURL == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	client := &http.Client{Timeout: sendTimeout}
	phase := "success"
	if !ok {
		phase = "fail"
	}
	if err := pingHealthchecksBody(ctx, client, hcURL, phase, summary); err != nil {
		log.Printf("notify: healthchecks domain result: %v", redactErr(err))
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
	for _, u := range c.healthchecksURLs() {
		if err := pingHealthchecks(ctx, client, u, "success"); err != nil {
			return fmt.Errorf("healthchecks: %w", err)
		}
	}
	if c.smtpReady() {
		if err := sendSMTP(ctx, c, ev); err != nil {
			return fmt.Errorf("smtp: %w", err)
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

// pingHealthchecks pings the check for a lifecycle phase: "start" (<base>/start),
// "success" (<base>) or "fail" (<base>/fail). Bodyless (GET) — the per-item pings.
func pingHealthchecks(ctx context.Context, client *http.Client, base, phase string) error {
	return pingHealthchecksBody(ctx, client, base, phase, "")
}

// pingHealthchecksBody pings the check for a lifecycle phase, optionally attaching a
// body that Healthchecks records in the check's event feed. An empty body sends a
// plain GET (the per-item lifecycle pings); a non-empty body is POSTed (the aggregate
// per-domain-run summary — see PingDomainResult).
func pingHealthchecksBody(ctx context.Context, client *http.Client, base, phase, body string) error {
	u := strings.TrimRight(base, "/")
	switch phase {
	case "start":
		u += "/start"
	case "fail":
		u += "/fail"
	}
	method := http.MethodGet
	var reqBody io.Reader
	if body != "" {
		method = http.MethodPost
		reqBody = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
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

// buildSMTPMessage renders ev into an RFC 5322 message (CRLF line endings):
// From/To/Subject/Date headers plus a plain-text body. Subject is the event
// title, the body its message. Kept pure so it can be unit-tested without a
// server.
func buildSMTPMessage(c Config, ev Event) []byte {
	var b strings.Builder
	b.WriteString("From: " + c.SMTPFrom + "\r\n")
	b.WriteString("To: " + c.SMTPTo + "\r\n")
	b.WriteString("Subject: " + ev.Title + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(ev.Message + "\r\n")
	return []byte(b.String())
}

// sendSMTP delivers ev as a plain-text email. TLS mode selects the transport:
//   - "tls": dial an implicit-TLS connection (port 465 style);
//   - "starttls" (default): plain dial, then upgrade with STARTTLS;
//   - "none": plain dial, no encryption.
//
// PLAIN auth is used only when a username is set. The dial is bounded by ctx's
// deadline (falling back to sendTimeout) so an unreachable server fails fast
// rather than hanging the (best-effort) notification.
func sendSMTP(ctx context.Context, c Config, ev Event) error {
	port := c.SMTPPort
	if port == 0 {
		port = 587
	}
	addr := net.JoinHostPort(c.SMTPHost, strconv.Itoa(port))

	deadline := time.Now().Add(sendTimeout)
	if d, ok := ctx.Deadline(); ok {
		deadline = d
	}
	dialer := &net.Dialer{Deadline: deadline}

	var (
		client *smtp.Client
		err    error
	)
	switch strings.ToLower(c.SMTPTLS) {
	case "tls":
		conn, dErr := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: c.SMTPHost, MinVersion: tls.VersionTLS12})
		if dErr != nil {
			return fmt.Errorf("dial %s: %w", addr, dErr)
		}
		client, err = smtp.NewClient(conn, c.SMTPHost)
	default: // "starttls" (default) and "none" dial plaintext first
		conn, dErr := dialer.DialContext(ctx, "tcp", addr)
		if dErr != nil {
			return fmt.Errorf("dial %s: %w", addr, dErr)
		}
		client, err = smtp.NewClient(conn, c.SMTPHost)
	}
	if err != nil {
		return err
	}
	defer client.Close() //nolint:errcheck // close error after Quit is not actionable

	if strings.EqualFold(c.SMTPTLS, "starttls") || c.SMTPTLS == "" {
		// Require STARTTLS when it was requested: if the server does not advertise
		// it, fail loudly instead of silently sending credentials/mail in cleartext
		// (a STARTTLS-stripping MITM must not be able to downgrade us). Users who
		// genuinely want plaintext can pick the "none" encryption mode explicitly.
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("starttls: server does not advertise STARTTLS — set Encryption to TLS (implicit) or None")
		}
		if err := client.StartTLS(&tls.Config{ServerName: c.SMTPHost, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}

	if c.SMTPUsername != "" {
		auth := smtp.PlainAuth("", c.SMTPUsername, c.SMTPPassword, c.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}

	if err := client.Mail(c.SMTPFrom); err != nil {
		return fmt.Errorf("from: %w", err)
	}
	for _, rcpt := range splitRecipients(c.SMTPTo) {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("rcpt %s: %w", rcpt, err)
		}
	}
	wc, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := wc.Write(buildSMTPMessage(c, ev)); err != nil {
		return err
	}
	if err := wc.Close(); err != nil {
		return err
	}
	return client.Quit()
}

// splitRecipients splits a comma/semicolon-separated recipient list into trimmed,
// non-empty addresses (the To header keeps the raw string for display).
func splitRecipients(to string) []string {
	fields := strings.FieldsFunc(to, func(r rune) bool { return r == ',' || r == ';' })
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if t := strings.TrimSpace(f); t != "" {
			out = append(out, t)
		}
	}
	return out
}
