// Package notify sends backup notifications to a webhook (generic JSON,
// Discord, Slack, Gotify or ntfy), a Matrix room, SMTP, a user-run Apprise API
// server and Healthchecks.io. Every send is best-effort and time-bounded, so a
// failed notification never affects a backup. The admin configures the URLs,
// which is why internal endpoints are allowed (no SSRF filtering).
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

// redactErr drops the request URL from a *url.Error before it is logged,
// because webhook and Healthchecks URLs carry their secret token in the path.
// The cause (timeout, connection refused) is kept.
func redactErr(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("%s: %w", ue.Op, ue.Err)
	}
	return err
}

const sendTimeout = 15 * time.Second

// hcSuppressKey marks a context in which Send and SendStart skip the
// Healthchecks ping. A scheduled per-domain run sets it on every item and pings
// once for the whole run instead (PingDomainStart, PingDomainResult).
type hcSuppressKey struct{}

// WithHealthchecksSuppressed returns a context in which Send and SendStart skip
// the Healthchecks ping. The other channels still fire.
func WithHealthchecksSuppressed(ctx context.Context) context.Context {
	return context.WithValue(ctx, hcSuppressKey{}, true)
}

func healthchecksSuppressed(ctx context.Context) bool {
	v, _ := ctx.Value(hcSuppressKey{}).(bool)
	return v
}

// msgSuppressKey marks a per-item message of a scheduled run. Send drops it
// when Config.ScheduledSummary is on, so the run ends in one "N of M" summary.
// It is separate from hcSuppressKey because a container update notice skips
// only the Healthchecks ping and must still deliver its message.
type msgSuppressKey struct{}

// WithMessagesSuppressed marks ctx as a per-item scheduled-backup message,
// which Send drops when ScheduledSummary is on.
func WithMessagesSuppressed(ctx context.Context) context.Context {
	return context.WithValue(ctx, msgSuppressKey{}, true)
}

// MessagesSuppressed reports whether ctx is a per-item scheduled-backup message.
// The service layer uses it to drop its own Unraid push in summary mode.
func MessagesSuppressed(ctx context.Context) bool {
	v, _ := ctx.Value(msgSuppressKey{}).(bool)
	return v
}

// Config holds the notification channels. An empty field disables that channel.
type Config struct {
	On string `json:"on"` // "never" | "failure" | "always"
	// The Enabled flags switch a channel off without clearing its settings.
	WebhookEnabled   bool   `json:"webhookEnabled"`
	WebhookURL       string `json:"webhookUrl"`
	WebhookFormat    string `json:"webhookFormat"` // generic|discord|slack|gotify|ntfy
	MatrixEnabled    bool   `json:"matrixEnabled"`
	MatrixHomeserver string `json:"matrixHomeserver"`
	MatrixToken      string `json:"matrixToken"`
	MatrixRoom       string `json:"matrixRoom"`
	HealthchecksURL  string `json:"healthchecksUrl"`
	// HealthchecksByDomain maps a backup domain ("container", "VM", "flash",
	// "config", "files", "zfs") to its own check URL, which replaces
	// HealthchecksURL for that domain. An empty entry falls back to
	// HealthchecksURL.
	HealthchecksByDomain map[string]string `json:"healthchecksByDomain"`
	// Unraid sends each event to Unraid's own notification system. The service
	// layer delivers it over SSH through the host's notify script; Send does not.
	Unraid bool `json:"unraid"`
	// SMTP sends each event as a plain-text email.
	SMTPEnabled    bool   `json:"smtpEnabled"`
	SMTPHost       string `json:"smtpHost"`
	SMTPPort       int    `json:"smtpPort"`
	SMTPUsername   string `json:"smtpUsername"`
	SMTPPassword   string `json:"smtpPassword"`
	SMTPFrom       string `json:"smtpFrom"`
	SMTPTo         string `json:"smtpTo"`
	SMTPTLS        string `json:"smtpTls"` // "starttls" (default) | "tls" | "none"
	AppriseEnabled bool   `json:"appriseEnabled"`
	// AppriseURL is the notify endpoint of a user-run Apprise API server
	// (github.com/caronc/apprise-api), typically http://host:8000/notify/<key>.
	// The <key> is a secret, so errors go through redactErr.
	AppriseURL string `json:"appriseUrl"`
	// AppriseTags is an optional comma-separated tag filter, so a shared
	// Apprise key can route BombVault events to some of its targets.
	AppriseTags string `json:"appriseTags"`
	// ScheduledSummary replaces the per-item messages of a scheduled run with
	// one "N of M" summary on webhook, Matrix, SMTP and Unraid. Healthchecks is
	// aggregated either way, and manual backups always report per item.
	ScheduledSummary bool `json:"scheduledSummary"`
	// NotifyOnUpdate sends "Updated <name> to a newer image" when the
	// post-backup image update replaces a container, so the user can check it
	// still works. It goes out per container, never folded into the summary.
	NotifyOnUpdate bool `json:"notifyOnUpdate"`
}

// UnmarshalJSON decodes a stored config. Older configs have no
// WebhookEnabled, MatrixEnabled or AppriseEnabled key, and reading those as
// false would turn off their failure alerts, so an absent key means the channel
// is on when its fields are filled in. A present key, false included, is kept.
//
// This is a decode rule rather than a migration because the stored blob is
// encrypted with APP_KEY, which the SQL migrations cannot read, and because an
// imported settings export needs the same rule.
func (c *Config) UnmarshalJSON(data []byte) error {
	return c.decode(data, false)
}

// DecodeStrict is UnmarshalJSON that also rejects unknown keys.
// json.Decoder.DisallowUnknownFields does not reach inside a custom
// UnmarshalJSON, and a misspelled "webhookEnable" would otherwise be dropped
// and the absent key would switch that channel on. The stored blob stays
// lenient because it may carry keys of removed fields.
func (c *Config) DecodeStrict(data []byte) error {
	return c.decode(data, true)
}

func (c *Config) decode(data []byte, strict bool) error {
	// storedConfig has Config's fields but none of its methods, so decoding
	// into it neither recurses into UnmarshalJSON nor hides the fields from
	// DisallowUnknownFields.
	type storedConfig Config
	var out storedConfig
	if strict {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&out); err != nil {
			return err
		}
	} else if err := json.Unmarshal(data, &out); err != nil {
		return err
	}
	// Read the three switches again as pointers: nil means the key was absent.
	var gates struct {
		Webhook *bool `json:"webhookEnabled"`
		Matrix  *bool `json:"matrixEnabled"`
		Apprise *bool `json:"appriseEnabled"`
	}
	if err := json.Unmarshal(data, &gates); err != nil {
		return err
	}
	if gates.Webhook == nil {
		out.WebhookEnabled = out.WebhookURL != ""
	}
	if gates.Matrix == nil {
		out.MatrixEnabled = out.MatrixHomeserver != "" && out.MatrixToken != "" && out.MatrixRoom != ""
	}
	if gates.Apprise == nil {
		out.AppriseEnabled = out.AppriseURL != ""
	}
	*c = Config(out)
	return nil
}

// Event is a completed backup, rendered into each channel's message.
type Event struct {
	Title   string
	Message string
	OK      bool
}

// Active reports whether notifications are switched on at all. It is an
// allowlist, so a corrupted or hand-edited value sends nothing. An empty On
// means nobody has chosen yet and counts as "failure", because whoever fills
// in a channel wants to hear when a backup breaks.
func (c Config) Active() bool {
	switch c.On {
	case "", "always", "failure":
		return true
	default:
		return false
	}
}

// shouldSend reports whether an event with this outcome gets a message.
func (c Config) shouldSend(ok bool) bool {
	switch c.On {
	case "always":
		return true
	case "failure", "":
		return !ok
	default:
		return false
	}
}

// Configured reports whether at least one channel is set. Healthchecks counts when
// the global URL or any per-domain URL is set.
func (c Config) Configured() bool {
	return c.webhookReady() || c.matrixReady() || len(c.healthchecksURLs()) > 0 || c.smtpReady() || c.appriseReady()
}

// healthchecksURLFor returns the domain's own Healthchecks URL, or the global
// one when the domain has none.
func (c Config) healthchecksURLFor(domain string) string {
	if u := c.HealthchecksByDomain[normalizeHCDomain(domain)]; u != "" {
		return u
	}
	return c.HealthchecksURL
}

// normalizeHCDomain maps the domain spellings used across the codebase to the
// HealthchecksByDomain keys. Backups say "container" and "VM", the off-site and
// tamper notifiers "containers" and "vms"; without this a per-domain check
// would miss the replication, drill and tamper failures.
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

// healthchecksURLs returns the global and per-domain Healthchecks URLs without
// duplicates.
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

func (c Config) webhookReady() bool {
	return c.WebhookEnabled && c.WebhookURL != ""
}

func (c Config) matrixReady() bool {
	return c.MatrixEnabled && c.MatrixHomeserver != "" && c.MatrixToken != "" && c.MatrixRoom != ""
}

func (c Config) smtpReady() bool {
	return c.SMTPEnabled && c.SMTPHost != "" && c.SMTPFrom != "" && c.SMTPTo != ""
}

func (c Config) appriseReady() bool {
	return c.AppriseEnabled && c.AppriseURL != ""
}

// Send delivers ev to the configured channels and logs each channel's error.
// The On policy applies to the message channels. Healthchecks is pinged on both
// outcomes unless notifications are off, because a check needs its success
// pings to stay green.
func Send(ctx context.Context, c Config, domain string, ev Event) {
	if !c.Active() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	client := &http.Client{Timeout: sendTimeout}

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

	// In summary mode Service.ScheduledNotifyResult reports the whole run.
	if MessagesSuppressed(ctx) && c.ScheduledSummary {
		return
	}

	if c.webhookReady() {
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
	if c.appriseReady() {
		if err := sendApprise(ctx, client, c, ev); err != nil {
			log.Printf("notify: apprise: %v", redactErr(err))
		}
	}
}

// SendStart marks the start of a backup. It pings the Healthchecks /start
// endpoint, so the check can measure duration and catch a hung run, and posts
// an "info" message to Apprise, the only message channel with a type for it.
// The Apprise post needs On=always and is skipped for per-item sends in
// summary mode.
func SendStart(ctx context.Context, c Config, domain string) {
	if !c.Active() {
		return
	}
	hcURL := c.healthchecksURLFor(domain)
	pingHC := hcURL != "" && !healthchecksSuppressed(ctx)
	postApprise := c.appriseReady() && c.On == "always" &&
		(!MessagesSuppressed(ctx) || !c.ScheduledSummary)
	if !pingHC && !postApprise {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	client := &http.Client{Timeout: sendTimeout}
	if pingHC {
		if err := pingHealthchecks(ctx, client, hcURL, "start"); err != nil {
			log.Printf("notify: healthchecks start: %v", redactErr(err))
		}
	}
	if postApprise {
		body := "Backup started"
		if domain != "" {
			body = "Backup started (" + domain + ")"
		}
		if err := postAppriseJSON(ctx, client, c, "BombVault", body, "info"); err != nil {
			log.Printf("notify: apprise start: %v", redactErr(err))
		}
	}
}

// PingDomainStart pings /start once for a whole scheduled per-domain run, in
// place of the per-item SendStart pings the run suppresses. domain may use the
// scheduler's plural spelling ("containers", "vms").
func PingDomainStart(ctx context.Context, c Config, domain string) {
	hcURL := c.healthchecksURLFor(domain)
	if !c.Active() || hcURL == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	client := &http.Client{Timeout: sendTimeout}
	if err := pingHealthchecks(ctx, client, hcURL, "start"); err != nil {
		log.Printf("notify: healthchecks domain start: %v", redactErr(err))
	}
}

// PingDomainResult pings the domain's check once at the end of a scheduled
// run, on <base> when ok and <base>/fail otherwise. summary, such as "1 of 3
// items failed", is sent as the body so it shows in the check's event log.
func PingDomainResult(ctx context.Context, c Config, domain string, ok bool, summary string) {
	hcURL := c.healthchecksURLFor(domain)
	if !c.Active() || hcURL == "" {
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
	ev := Event{Title: "BombVault", Message: "Test notification: notifications are working.", OK: true}

	if c.webhookReady() {
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
	if c.appriseReady() {
		if err := sendApprise(ctx, client, c, ev); err != nil {
			return fmt.Errorf("apprise: %w", err)
		}
	}
	return nil
}

// sendApprise posts ev to the Apprise /notify/<key> endpoint with the type
// "success" or "failure".
func sendApprise(ctx context.Context, client *http.Client, c Config, ev Event) error {
	typ := "success"
	if !ev.OK {
		typ = "failure"
	}
	return postAppriseJSON(ctx, client, c, ev.Title, ev.Message, typ)
}

// postAppriseJSON posts {title, body, type}, plus "tag" when AppriseTags is set.
func postAppriseJSON(ctx context.Context, client *http.Client, c Config, title, body, typ string) error {
	payload := map[string]string{"title": title, "body": body, "type": typ}
	if c.AppriseTags != "" {
		payload["tag"] = c.AppriseTags
	}
	return postJSON(ctx, client, c.AppriseURL, payload)
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

// pingHealthchecks sends a GET for the phase "start" (<base>/start), "success"
// (<base>) or "fail" (<base>/fail).
func pingHealthchecks(ctx context.Context, client *http.Client, base, phase string) error {
	return pingHealthchecksBody(ctx, client, base, phase, "")
}

// pingHealthchecksBody is pingHealthchecks with a body, which Healthchecks
// records in the check's event log. A non-empty body turns the GET into a POST.
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

// buildSMTPMessage renders ev as a plain-text RFC 5322 message with the title
// as subject.
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
// deadline, or sendTimeout, so an unreachable server fails fast.
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
		// Fail rather than fall back to cleartext, or a MITM that strips the
		// STARTTLS extension would get the credentials. Plaintext is the explicit
		// "none" mode.
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("starttls: server does not advertise STARTTLS. Set Encryption to TLS (implicit) or None")
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

// splitRecipients splits a comma or semicolon separated recipient list into
// trimmed addresses. The To header keeps the raw string.
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
