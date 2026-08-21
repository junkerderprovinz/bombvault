package backup

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestTruncateErrScrubsURLCredentials pins this package's own copy of the
// credential-scrub fix already applied to internal/restic/restic.go and
// internal/api/handlers.go. truncateErr's old doc comment claimed the
// restic/dockercli adapters "already scrub secrets/paths from their own
// errors" and so only truncated — true for THOSE callers today, but not a
// property this function could actually enforce for every future caller, so
// it now scrubs unconditionally instead of trusting every upstream error to
// have been pre-scrubbed.
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

// TestTruncateErrScrubsNumericUsername mirrors the twin fix in
// internal/restic and internal/api: the credential regex's leading-character
// class must not require a letter, or a fully-numeric username lets its
// password through unscrubbed.
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

// TestTruncateErrDoesNotEatHostPort pins that the credential scrub is scoped
// to a real "user:pass@" userinfo segment and doesn't fire on an ordinary
// "host:port" (no "@").
func TestTruncateErrDoesNotEatHostPort(t *testing.T) {
	got := truncateErr(errors.New("unable to reach storage.example.com:8000: connection refused"))
	if strings.Contains(got, "[redacted]") {
		t.Fatalf("credential scrub must not fire on a plain host:port, got %q", got)
	}
	if !strings.Contains(got, "storage.example.com") {
		t.Fatalf("host:port with no userinfo must survive untouched, got %q", got)
	}
}

// TestTruncateErrPreservesHostname pins the same reorder fix as
// internal/restic/restic.go's scrubSecrets: scrubbing credentials BEFORE
// paths destroys the hostname along with the password, so this package's own
// copy runs the path scrub first too.
func TestTruncateErrPreservesHostname(t *testing.T) {
	got := truncateErr(errors.New(`rest:https://backupuser:Tr0ub4dor&3@storage.example.com:8000/containers`))
	if strings.Contains(got, "Tr0ub4dor") || strings.Contains(got, "backupuser") {
		t.Fatalf("truncateErr leaked the credentials, got %q", got)
	}
	if !strings.Contains(got, "storage.example.com") {
		t.Fatalf("truncateErr destroyed the hostname an operator needs to diagnose which target failed, got %q", got)
	}
}

// TestTruncateErrBypassesRestoreConflict is the regression test for the
// finding that truncateErr used to run EVERY error through scrubRunErr
// unconditionally, on the false theory that scrubbing already-clean text is a
// harmless no-op. ErrRestoreConflict's message is a perfect counterexample:
// checkRestoreConflicts' host:port conflict list ("host port 8080/tcp is
// already used by container ...") is already user-safe, but contains "/",
// which runErrPathRe mistakes for a filesystem path. Before
// restoreConflictBypass, this exact error — produced by
// checkRestoreConflicts, in this same package — reached runs.error
// via orchestrator.go's Restore path with its port numbers mangled into
// "[path]", even though the identical error survives intact through the api
// package's scrubError.
func TestTruncateErrBypassesRestoreConflict(t *testing.T) {
	err := fmt.Errorf("%w — free these and retry: %s", ErrRestoreConflict,
		`host port 8080/tcp is already used by container "other-app"`)

	got := truncateErr(err)
	if strings.Contains(got, "[path]") {
		t.Fatalf("truncateErr mangled the restore-conflict text into [path], got %q", got)
	}
	if !strings.Contains(got, "8080/tcp") {
		t.Fatalf("truncateErr must preserve the literal host:port conflict text, got %q", got)
	}
}

// TestTruncateErrNilAndTruncation pins truncateErr's pre-existing nil and
// length-cap behavior, unaffected by the scrub now running first.
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
