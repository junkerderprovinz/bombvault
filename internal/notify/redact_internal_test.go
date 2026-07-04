package notify

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

// TestRedactErrStripsSecretURL pins the secret-leak fix: a *url.Error carrying a
// webhook/Healthchecks URL (token in the path) must not survive into the logged
// message, while the underlying cause is kept.
func TestRedactErrStripsSecretURL(t *testing.T) {
	secret := "SUPERSECRET-TOKEN"
	ue := &url.Error{
		Op:  "Post",
		URL: "https://discord.com/api/webhooks/123/" + secret,
		Err: errors.New("dial tcp: connection refused"),
	}
	got := redactErr(ue).Error()
	if strings.Contains(got, secret) {
		t.Fatalf("redacted error still leaks the secret URL: %q", got)
	}
	if !strings.Contains(got, "connection refused") {
		t.Fatalf("redacted error dropped the underlying cause: %q", got)
	}
}

// TestRedactErrPassesThroughPlainError leaves a non-url error untouched.
func TestRedactErrPassesThroughPlainError(t *testing.T) {
	err := errors.New("boom")
	if got := redactErr(err); got.Error() != "boom" {
		t.Fatalf("plain error changed: %q", got.Error())
	}
}

// TestBuildSMTPMessage pins the email composition: Subject is the event title,
// the body is the event message, the From/To headers carry the configured
// addresses, and CRLF line endings separate header from body.
func TestBuildSMTPMessage(t *testing.T) {
	cfg := Config{SMTPFrom: "bombvault@example.com", SMTPTo: "admin@example.com"}
	ev := Event{Title: "BombVault", Message: "Backup of container \"plex\" succeeded.", OK: true}
	msg := string(buildSMTPMessage(cfg, ev))

	for _, want := range []string{
		"From: bombvault@example.com\r\n",
		"To: admin@example.com\r\n",
		"Subject: BombVault\r\n",
		"Content-Type: text/plain; charset=utf-8\r\n",
		"\r\nBackup of container \"plex\" succeeded.\r\n",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q\n--- message ---\n%s", want, msg)
		}
	}
}

// TestSplitRecipients splits comma/semicolon lists and drops blanks.
func TestSplitRecipients(t *testing.T) {
	got := splitRecipients(" a@x.com ,b@x.com; ;c@x.com ")
	want := []string{"a@x.com", "b@x.com", "c@x.com"}
	if len(got) != len(want) {
		t.Fatalf("splitRecipients = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitRecipients[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestHealthchecksURLFor pins the per-domain resolver: a domain with a non-empty
// entry uses that URL; a blank entry, an absent domain, or a nil map all fall back
// to the global HealthchecksURL.
func TestHealthchecksURLFor(t *testing.T) {
	c := Config{
		HealthchecksURL:      "https://hc/global",
		HealthchecksByDomain: map[string]string{"flash": "https://hc/flash", "config": ""},
	}
	if got := c.healthchecksURLFor("flash"); got != "https://hc/flash" {
		t.Fatalf("flash → %q, want the per-domain URL", got)
	}
	if got := c.healthchecksURLFor("config"); got != "https://hc/global" {
		t.Fatalf("blank per-domain entry should fall back to global, got %q", got)
	}
	if got := c.healthchecksURLFor("VM"); got != "https://hc/global" {
		t.Fatalf("absent domain should fall back to global, got %q", got)
	}
	nilMap := Config{HealthchecksURL: "https://hc/global"}
	if got := nilMap.healthchecksURLFor("flash"); got != "https://hc/global" {
		t.Fatalf("nil map should fall back to global, got %q", got)
	}
	// The off-site/tamper notifiers use the plural "containers"/"vms"; they must
	// normalize to the same per-domain check as the "container"/"VM" backup pings.
	norm := Config{HealthchecksByDomain: map[string]string{"container": "https://hc/ct", "VM": "https://hc/vm"}}
	if got := norm.healthchecksURLFor("containers"); got != "https://hc/ct" {
		t.Fatalf("containers should normalize to the container check, got %q", got)
	}
	if got := norm.healthchecksURLFor("vms"); got != "https://hc/vm" {
		t.Fatalf("vms should normalize to the VM check, got %q", got)
	}
}

// TestSMTPReadyGating: smtpReady only fires when enabled AND host/from/to are set.
func TestSMTPReadyGating(t *testing.T) {
	if (Config{SMTPHost: "smtp.x.com", SMTPFrom: "a@x.com", SMTPTo: "b@x.com"}).smtpReady() {
		t.Fatal("smtpReady must be false when SMTPEnabled is false")
	}
	if (Config{SMTPEnabled: true, SMTPFrom: "a@x.com", SMTPTo: "b@x.com"}).smtpReady() {
		t.Fatal("smtpReady must be false when host is empty")
	}
	if !(Config{SMTPEnabled: true, SMTPHost: "smtp.x.com", SMTPFrom: "a@x.com", SMTPTo: "b@x.com"}).smtpReady() {
		t.Fatal("smtpReady must be true when enabled and host/from/to are set")
	}
}
