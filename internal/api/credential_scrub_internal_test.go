package api

import (
	"errors"
	"strings"
	"testing"
)

// TestScrubErrorScrubsURLCredentials pins the fix for the finding that
// absPathRe alone stops at the first ":" and so never reaches into a
// "user:pass@host" URL's userinfo — meaning a repo location like restic's
// rest:/s3: backends use (the generated deploy recipe documents this exact
// "rest:https://user:pass@host:port/path" shape as valid) leaked its password
// verbatim through scrubError, the SAME defense-in-depth path
// internal/restic/restic.go's lastReason has its own independent copy of.
// The example URL and password are the ones from the finding.
func TestScrubErrorScrubsURLCredentials(t *testing.T) {
	err := errors.New(`unable to open repository at rest:https://backupuser:Tr0ub4dor&3@storage.example.com:8000/containers: repository does not exist`)
	got := scrubError(err)
	if strings.Contains(got, "Tr0ub4dor") {
		t.Fatalf("scrubError leaked the repo password, got %q", got)
	}
	if strings.Contains(got, "backupuser") {
		t.Fatalf("scrubError leaked the repo username, got %q", got)
	}
	if !strings.Contains(got, "unable to open repository") {
		t.Fatalf("scrubError should keep the actual cause, got %q", got)
	}
}

// TestCredentialReDoesNotEatHostPort pins that the credential scrub is scoped
// to a real "user:pass@" userinfo segment and doesn't fire on an ordinary
// "host:port" (no "@"), so it can't gratuitously eat legitimate non-secret
// content the existing scrubError tests already rely on (e.g.
// backup.ErrRestoreConflict's "8080/tcp" host-port text, which bypasses the
// scrubber entirely, and any other message that merely contains a colon).
func TestCredentialReDoesNotEatHostPort(t *testing.T) {
	got := scrubError(errors.New("unable to reach storage.example.com:8000: connection refused"))
	if strings.Contains(got, "[redacted]") {
		t.Fatalf("credential scrub must not fire on a plain host:port, got %q", got)
	}
	if !strings.Contains(got, "storage.example.com") {
		t.Fatalf("host:port with no userinfo must survive untouched, got %q", got)
	}
}

// TestScrubErrorScrubsNumericUsername pins the fix for the finding that
// credentialRe's leading-character class used to require a LETTER, so a
// fully-numeric username (a legal restic/rclone userinfo username) slipped
// through completely unscrubbed — the password right after it too, since the
// whole "user:pass@" match never fired at all.
func TestScrubErrorScrubsNumericUsername(t *testing.T) {
	err := errors.New(`unable to open repository at rest:https://123456:SuperSecret@storage.example.com:8000/containers: repository does not exist`)
	got := scrubError(err)
	if strings.Contains(got, "SuperSecret") {
		t.Fatalf("scrubError leaked the repo password for a numeric username, got %q", got)
	}
	if strings.Contains(got, "123456") {
		t.Fatalf("scrubError leaked the numeric repo username, got %q", got)
	}
	if !strings.Contains(got, "unable to open repository") {
		t.Fatalf("scrubError should keep the actual cause, got %q", got)
	}
}

// TestScrubSecretsPreservesHostname pins the fix for the finding that
// scrubbing credentials BEFORE paths (the original order) destroyed the
// hostname along with the password: once credentialRe replaced "user:pass@"
// with "[redacted]@", the leftover "scheme://[redacted]@host" was exactly the
// path-like shape absPathRe matches, so absPathRe's later pass ate the
// hostname too — leaving an operator with multiple off-site targets unable to
// tell which one a failure came from. Scrubbing paths first, then credentials,
// redacts the password exactly as completely while keeping the hostname.
func TestScrubSecretsPreservesHostname(t *testing.T) {
	got := scrubSecrets(`rest:https://backupuser:Tr0ub4dor&3@storage.example.com:8000/containers`)
	if strings.Contains(got, "Tr0ub4dor") || strings.Contains(got, "backupuser") {
		t.Fatalf("scrubSecrets leaked the credentials, got %q", got)
	}
	if !strings.Contains(got, "storage.example.com") {
		t.Fatalf("scrubSecrets destroyed the hostname an operator needs to diagnose which off-site target failed, got %q", got)
	}
}
