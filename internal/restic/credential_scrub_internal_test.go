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
