package restic

import (
	"strings"
	"testing"
)

// TestLastReasonScrubsURLCredentials pins the fix for the finding that
// reasonPathRe alone stops at the first ":" and so never reaches into a
// "user:pass@host" URL's userinfo — meaning a repo location like restic's
// rest:/s3: backends use (the generated deploy recipe documents this exact
// "rest:https://user:pass@host:port/path" shape as valid) leaked its password
// verbatim into the UI, run history and outbound notifications. The example
// URL and password below are the ones from the finding.
func TestLastReasonScrubsURLCredentials(t *testing.T) {
	stderr := `Fatal: unable to open repository at rest:https://backupuser:Tr0ub4dor&3@storage.example.com:8000/containers: repository does not exist`
	got := lastReason(stderr)
	if strings.Contains(got, "Tr0ub4dor") {
		t.Fatalf("lastReason leaked the repo password, got %q", got)
	}
	if strings.Contains(got, "backupuser") {
		t.Fatalf("lastReason leaked the repo username, got %q", got)
	}
	if !strings.Contains(got, "unable to open repository") {
		t.Fatalf("lastReason should keep the actual cause, got %q", got)
	}
}

// TestLastReasonScrubsURLCredentialsInItemCause exercises the OTHER call site
// that applied the path scrubber directly (itemErrorCauses, used when a
// restore's per-item JSON errors reference a remote repo location) to make
// sure the credential scrub covers it too, not just lastReason's own final
// pass.
func TestLastReasonScrubsURLCredentialsInItemCause(t *testing.T) {
	stderr := strings.Join([]string{
		`{"message_type":"error","error":{"message":"open rest:https://backupuser:Tr0ub4dor&3@storage.example.com:8000/containers: connection refused"},"during":"restore","item":"/x"}`,
		"Fatal: There were 1 errors",
	}, "\n")
	got := lastReason(stderr)
	if strings.Contains(got, "Tr0ub4dor") {
		t.Fatalf("itemErrorCauses leaked the repo password, got %q", got)
	}
}

// TestCredentialReDoesNotEatHostPort pins that the credential scrub is scoped
// to a real "user:pass@" userinfo segment and doesn't mistake an ordinary
// "host:port" (no "@") for one, so it can't gratuitously eat legitimate
// non-secret content the existing path-scrub tests already rely on.
func TestCredentialReDoesNotEatHostPort(t *testing.T) {
	got := lastReason("Fatal: unable to reach storage.example.com:8000: connection refused")
	if strings.Contains(got, "[redacted]") {
		t.Fatalf("credential scrub must not fire on a plain host:port, got %q", got)
	}
	if !strings.Contains(got, "storage.example.com") {
		t.Fatalf("host:port with no userinfo must survive untouched, got %q", got)
	}
}

// TestLastReasonScrubsNumericUsername pins the fix for the finding that
// credentialRe's leading-character class used to require a LETTER, so a
// fully-numeric username (a legal restic/rclone userinfo username) slipped
// through completely unscrubbed — the password right after it too, since the
// whole "user:pass@" match never fired at all.
func TestLastReasonScrubsNumericUsername(t *testing.T) {
	stderr := `Fatal: unable to open repository at rest:https://123456:SuperSecret@storage.example.com:8000/containers: repository does not exist`
	got := lastReason(stderr)
	if strings.Contains(got, "SuperSecret") {
		t.Fatalf("lastReason leaked the repo password for a numeric username, got %q", got)
	}
	if strings.Contains(got, "123456") {
		t.Fatalf("lastReason leaked the numeric repo username, got %q", got)
	}
	if !strings.Contains(got, "unable to open repository") {
		t.Fatalf("lastReason should keep the actual cause, got %q", got)
	}
}

// TestScrubSecretsPreservesHostname pins the fix for the finding that
// scrubbing credentials BEFORE paths (the original order) destroyed the
// hostname along with the password: once credentialRe replaced "user:pass@"
// with "[redacted]@", the leftover "scheme://[redacted]@host" was exactly the
// path-like shape reasonPathRe matches, so reasonPathRe's later pass ate the
// hostname too — leaving an operator with multiple off-site targets unable to
// tell which one a failure came from. Scrubbing paths first, then
// credentials, redacts the password exactly as completely while keeping the
// hostname.
func TestScrubSecretsPreservesHostname(t *testing.T) {
	got := scrubSecrets(`rest:https://backupuser:Tr0ub4dor&3@storage.example.com:8000/containers`)
	if strings.Contains(got, "Tr0ub4dor") || strings.Contains(got, "backupuser") {
		t.Fatalf("scrubSecrets leaked the credentials, got %q", got)
	}
	if !strings.Contains(got, "storage.example.com") {
		t.Fatalf("scrubSecrets destroyed the hostname an operator needs to diagnose which off-site target failed, got %q", got)
	}
}
