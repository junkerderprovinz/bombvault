package api

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
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

// TestTruncateRunErrScrubsCredentials pins that truncateRunErr's bypass-first
// rework (scrubBypassMessage) did not weaken the tamper-leak fix it sits
// alongside: an ordinary, non-sentinel error carrying a raw URL credential
// must still come out scrubbed, exactly like scrubError.
func TestTruncateRunErrScrubsCredentials(t *testing.T) {
	err := errors.New(`unable to open repository at rest:https://backupuser:Tr0ub4dor&3@storage.example.com:8000/containers: repository does not exist`)
	got := truncateRunErr(err)
	if strings.Contains(got, "Tr0ub4dor") {
		t.Fatalf("truncateRunErr leaked the repo password, got %q", got)
	}
	if strings.Contains(got, "backupuser") {
		t.Fatalf("truncateRunErr leaked the repo username, got %q", got)
	}
	if !strings.Contains(got, "unable to open repository") {
		t.Fatalf("truncateRunErr should keep the actual cause, got %q", got)
	}
}

// TestTruncateRunErrBypassesRestoreConflict is the regression test for the
// finding that truncateRunErr used to run EVERY error through scrubSecrets
// unconditionally, on the false theory that scrubbing already-clean text is a
// harmless no-op. backup.ErrRestoreConflict's message is a perfect
// counterexample: scrubError has always bypassed it (see scrubBypassMessage)
// specifically because its host:port conflict list contains "/" (as in
// "8080/tcp") that absPathRe mistakes for a filesystem path. truncateRunErr
// had no equivalent bypass, so the SAME error that survives scrubError intact
// used to reach runs.error with its port numbers mangled.
func TestTruncateRunErrBypassesRestoreConflict(t *testing.T) {
	err := fmt.Errorf("%w — free these and retry: %s", backup.ErrRestoreConflict,
		`host port 8080/tcp is already used by container "other-app"`)

	// Confirm scrubError already treats this as safe-verbatim (the baseline
	// truncateRunErr must now match).
	if got := scrubError(err); strings.Contains(got, "[path]") {
		t.Fatalf("test setup: scrubError itself mangled the conflict text, got %q — fix the fixture", got)
	}

	got := truncateRunErr(err)
	if strings.Contains(got, "[path]") {
		t.Fatalf("truncateRunErr mangled the restore-conflict text into [path], got %q", got)
	}
	if !strings.Contains(got, "8080/tcp") {
		t.Fatalf("truncateRunErr must preserve the literal host:port conflict text, got %q", got)
	}
}

// TestTruncateRunErrBypassesZvolRebaseFailed is the same regression, for the
// other sentinel this exact review round is about: a zvol-rebase failure
// names a ZFS dataset ("<pool>/<rest>"), which necessarily contains "/". This
// mirrors TestPrepareRestoreVMCrossInstanceZvolRebaseFailureBypassesScrubber's
// scrubError assertion (foreign_vm_restore_internal_test.go), but for
// truncateRunErr — the run-bookkeeping path that error also travels through
// via recordContainerFailure/finishRestoreRun-style callers.
func TestTruncateRunErrBypassesZvolRebaseFailed(t *testing.T) {
	err := fmt.Errorf("rebase dataset %q onto pool %q: %w", "tank/vms/zvolvm/disk1", "-badpool", errZvolRebaseFailed)

	if got := scrubError(err); strings.Contains(got, "[path]") {
		t.Fatalf("test setup: scrubError itself mangled the dataset name, got %q — fix the fixture", got)
	}

	got := truncateRunErr(err)
	if strings.Contains(got, "[path]") {
		t.Fatalf("truncateRunErr mangled the ZFS dataset name into [path], got %q", got)
	}
	if !strings.Contains(got, "tank/vms/zvolvm/disk1") {
		t.Fatalf("truncateRunErr must preserve the literal ZFS dataset name, got %q", got)
	}
}

// TestTruncateRunErrDockerDaemonUnreachableStillScrubbed documents a THIRD
// slash-heavy operator-facing shape this same review round raised (a Docker
// connectivity failure naming its socket path, e.g. "Cannot connect to the
// Docker daemon at unix:///var/run/docker.sock" — the exact fixture
// TestBackupRecordsFailedRunOnPreflightFault, backup_failure_recorded_test.go,
// uses for its "inspect fault" case, reaching truncateRunErr via
// recordContainerFailure). Unlike the 8080/tcp and ZFS-dataset cases above,
// this text is NOT wrapped in any of scrubError's 5 sentinel-bypass types —
// nothing in this codebase tags a raw Docker-daemon-unreachable error that
// way — so scrubBypassMessage correctly does NOT exempt it, and it is still
// scrubbed post-fix exactly as it was before and exactly as scrubError itself
// would scrub the identical text. This is intentional parity, not a residual
// bug: extending the bypass to arbitrary slash-heavy text (rather than only
// the named sentinels) would reopen exactly the kind of unreviewed bypass
// surface this fix was careful to avoid. Pinned here so a future change to
// scrubBypassMessage can't silently start (or stop) exempting this shape
// without a test noticing.
func TestTruncateRunErrDockerDaemonUnreachableStillScrubbed(t *testing.T) {
	err := fmt.Errorf("inspect container: %w", errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock"))

	got := truncateRunErr(err)
	if strings.Contains(got, "unix:///var/run/docker.sock") {
		t.Fatalf("expected the socket path to still be scrubbed (parity with scrubError), got %q", got)
	}
	if !strings.Contains(got, "Cannot connect to the Docker daemon") {
		t.Fatalf("truncateRunErr should keep the actual cause, got %q", got)
	}
	// Parity check: scrubError must treat the identical input identically —
	// truncateRunErr must never diverge from scrubError in EITHER direction.
	if want, gotScrub := scrubError(err), got; want != gotScrub {
		t.Fatalf("truncateRunErr and scrubError diverged on a non-bypassed error: scrubError=%q truncateRunErr=%q", want, gotScrub)
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
