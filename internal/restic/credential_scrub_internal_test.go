package restic

import (
	"strings"
	"testing"
)

// TestLastReasonScrubsURLCredentials covers the userinfo of a
// "rest:https://user:pass@host:port/path" repository, which reasonPathRe alone
// never reaches because it stops at the first ":".
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

// TestLastReasonScrubsURLCredentialsInItemCause covers itemErrorCauses, which
// scrubs per-item restore errors on its own, apart from lastReason's final
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

func TestCredentialReDoesNotEatHostPort(t *testing.T) {
	got := lastReason("Fatal: unable to reach storage.example.com:8000: connection refused")
	if strings.Contains(got, "[redacted]") {
		t.Fatalf("credential scrub must not fire on a plain host:port, got %q", got)
	}
	if !strings.Contains(got, "storage.example.com") {
		t.Fatalf("host:port with no userinfo must survive untouched, got %q", got)
	}
}

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

// TestScrubSecretsPreservesHostname checks that the hostname survives, which
// needs credentials scrubbed after paths. In the other order the leftover
// "scheme://[redacted]@host" looks like a path to reasonPathRe, which then
// removes the hostname an operator needs to tell off-site targets apart.
func TestScrubSecretsPreservesHostname(t *testing.T) {
	got := scrubSecrets(`rest:https://backupuser:Tr0ub4dor&3@storage.example.com:8000/containers`)
	if strings.Contains(got, "Tr0ub4dor") || strings.Contains(got, "backupuser") {
		t.Fatalf("scrubSecrets leaked the credentials, got %q", got)
	}
	if !strings.Contains(got, "storage.example.com") {
		t.Fatalf("scrubSecrets destroyed the hostname an operator needs to diagnose which off-site target failed, got %q", got)
	}
}
