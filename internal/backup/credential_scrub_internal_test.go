package backup

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestTruncateErrScrubsURLCredentials checks that truncateErr scrubs
// credentials itself rather than trusting every caller to pass a clean error.
func TestTruncateErrScrubsURLCredentials(t *testing.T) {
	err := errors.New(`unable to open repository at rest:https://backupuser:Tr0ub4dor&3@storage.example.com:8000/containers: repository does not exist`)
	got := truncateErr(err)
	if strings.Contains(got, "Tr0ub4dor") {
		t.Fatalf("truncateErr leaked the repo password, got %q", got)
	}
	if strings.Contains(got, "backupuser") {
		t.Fatalf("truncateErr leaked the repo username, got %q", got)
	}
	if !strings.Contains(got, "unable to open repository") {
		t.Fatalf("truncateErr should keep the actual cause, got %q", got)
	}
}

// TestTruncateErrScrubsNumericUsername checks that a username of digits alone
// is scrubbed together with its password.
func TestTruncateErrScrubsNumericUsername(t *testing.T) {
	err := errors.New(`unable to open repository at rest:https://123456:SuperSecret@storage.example.com:8000/containers: repository does not exist`)
	got := truncateErr(err)
	if strings.Contains(got, "SuperSecret") {
		t.Fatalf("truncateErr leaked the repo password for a numeric username, got %q", got)
	}
	if strings.Contains(got, "123456") {
		t.Fatalf("truncateErr leaked the numeric repo username, got %q", got)
	}
}

// TestTruncateErrDoesNotEatHostPort checks that the credential scrub needs a
// user:pass@ segment and leaves a plain host:port alone.
func TestTruncateErrDoesNotEatHostPort(t *testing.T) {
	got := truncateErr(errors.New("unable to reach storage.example.com:8000: connection refused"))
	if strings.Contains(got, "[redacted]") {
		t.Fatalf("credential scrub must not fire on a plain host:port, got %q", got)
	}
	if !strings.Contains(got, "storage.example.com") {
		t.Fatalf("host:port with no userinfo must survive untouched, got %q", got)
	}
}

// TestTruncateErrPreservesHostname checks that the hostname survives the
// scrub. Scrubbing credentials before paths would take it with the password.
func TestTruncateErrPreservesHostname(t *testing.T) {
	got := truncateErr(errors.New(`rest:https://backupuser:Tr0ub4dor&3@storage.example.com:8000/containers`))
	if strings.Contains(got, "Tr0ub4dor") || strings.Contains(got, "backupuser") {
		t.Fatalf("truncateErr leaked the credentials, got %q", got)
	}
	if !strings.Contains(got, "storage.example.com") {
		t.Fatalf("truncateErr destroyed the hostname an operator needs to diagnose which target failed, got %q", got)
	}
}

// TestTruncateErrBypassesRestoreConflict checks that a restore conflict keeps
// its port list. The message is already safe to show, and runErrPathRe would
// read "8080/tcp" as a path.
func TestTruncateErrBypassesRestoreConflict(t *testing.T) {
	err := fmt.Errorf("%w. Free these and retry: %s", ErrRestoreConflict,
		`host port 8080/tcp is already used by container "other-app"`)

	got := truncateErr(err)
	if strings.Contains(got, "[path]") {
		t.Fatalf("truncateErr mangled the restore-conflict text into [path], got %q", got)
	}
	if !strings.Contains(got, "8080/tcp") {
		t.Fatalf("truncateErr must preserve the literal host:port conflict text, got %q", got)
	}
}

func TestTruncateErrNilAndTruncation(t *testing.T) {
	if got := truncateErr(nil); got != "" {
		t.Fatalf("truncateErr(nil) = %q, want empty", got)
	}
	long := strings.Repeat("x", 600)
	got := truncateErr(errors.New(long))
	if len(got) != 500 {
		t.Fatalf("truncateErr should cap at 500 chars, got %d", len(got))
	}
}
