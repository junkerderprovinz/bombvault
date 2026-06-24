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
