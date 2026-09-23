package notify

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

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

func TestRedactErrPassesThroughPlainError(t *testing.T) {
	err := errors.New("boom")
	if got := redactErr(err); got.Error() != "boom" {
		t.Fatalf("plain error changed: %q", got.Error())
	}
}

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
	// The off-site and tamper notifiers use the plural spellings.
	norm := Config{HealthchecksByDomain: map[string]string{"container": "https://hc/ct", "VM": "https://hc/vm"}}
	if got := norm.healthchecksURLFor("containers"); got != "https://hc/ct" {
		t.Fatalf("containers should normalize to the container check, got %q", got)
	}
	if got := norm.healthchecksURLFor("vms"); got != "https://hc/vm" {
		t.Fatalf("vms should normalize to the VM check, got %q", got)
	}
}

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

func TestWebhookReadyGating(t *testing.T) {
	if (Config{WebhookURL: "https://example.com/hook"}).webhookReady() {
		t.Fatal("webhookReady must be false when WebhookEnabled is false")
	}
	if (Config{WebhookEnabled: true}).webhookReady() {
		t.Fatal("webhookReady must be false when the URL is empty")
	}
	if !(Config{WebhookEnabled: true, WebhookURL: "https://example.com/hook"}).webhookReady() {
		t.Fatal("webhookReady must be true when enabled and a URL is set")
	}
}

func TestMatrixReadyGating(t *testing.T) {
	if (Config{MatrixHomeserver: "https://m.example", MatrixToken: "tok", MatrixRoom: "!r:x"}).matrixReady() {
		t.Fatal("matrixReady must be false when MatrixEnabled is false")
	}
	if (Config{MatrixEnabled: true, MatrixToken: "tok", MatrixRoom: "!r:x"}).matrixReady() {
		t.Fatal("matrixReady must be false when the homeserver is empty")
	}
	if !(Config{MatrixEnabled: true, MatrixHomeserver: "https://m.example", MatrixToken: "tok", MatrixRoom: "!r:x"}).matrixReady() {
		t.Fatal("matrixReady must be true when enabled and homeserver/token/room are all set")
	}
}

func TestAppriseReadyGating(t *testing.T) {
	if (Config{AppriseURL: "https://apprise.example/notify/key"}).appriseReady() {
		t.Fatal("appriseReady must be false when AppriseEnabled is false")
	}
	if (Config{AppriseEnabled: true}).appriseReady() {
		t.Fatal("appriseReady must be false when the URL is empty")
	}
	if !(Config{AppriseEnabled: true, AppriseURL: "https://apprise.example/notify/key"}).appriseReady() {
		t.Fatal("appriseReady must be true when enabled and a URL is set")
	}
}
