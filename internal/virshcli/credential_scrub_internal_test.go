package virshcli

import (
	"strings"
	"testing"
)

// TestLastReasonScrubsURLCredentials pins this package's copy of the
// credential-scrub fix already applied to internal/restic/restic.go and
// internal/api/handlers.go: this file's own doc comment claims lastReason
// "mirrors restic's lastReason scrubbing", so it must catch a "user:pass@"
// userinfo segment too, not just absolute paths, to actually keep that
// parity rather than silently diverging from it.
func TestLastReasonScrubsURLCredentials(t *testing.T) {
	stderr := `error: Failed to connect to qemu+ssh://backupuser:Tr0ub4dor&3@storage.example.com/system: Cannot recv data`
	got := lastReason(stderr)
	if strings.Contains(got, "Tr0ub4dor") {
		t.Fatalf("lastReason leaked the password, got %q", got)
	}
	if strings.Contains(got, "backupuser") {
		t.Fatalf("lastReason leaked the username, got %q", got)
	}
	if !strings.Contains(got, "Cannot recv data") {
		t.Fatalf("lastReason should keep the actual cause, got %q", got)
	}
}

// TestLastReasonScrubsNumericUsername mirrors the twin fix in
// internal/restic and internal/api: credentialRe's leading-character class
// must not require a letter, or a fully-numeric username lets its password
// through unscrubbed.
func TestLastReasonScrubsNumericUsername(t *testing.T) {
	stderr := `error: Failed to connect to qemu+ssh://123456:SuperSecret@storage.example.com/system: Cannot recv data`
	got := lastReason(stderr)
	if strings.Contains(got, "SuperSecret") {
		t.Fatalf("lastReason leaked the password for a numeric username, got %q", got)
	}
	if strings.Contains(got, "123456") {
		t.Fatalf("lastReason leaked the numeric username, got %q", got)
	}
}

// TestLastReasonDoesNotEatHostPort pins that the credential scrub is scoped to
// a real "user:pass@" userinfo segment and doesn't fire on ordinary text that
// merely contains a colon.
func TestLastReasonDoesNotEatHostPort(t *testing.T) {
	got := lastReason("error: unable to connect to server at 'storage.example.com:16509': Connection refused")
	if strings.Contains(got, "[redacted]") {
		t.Fatalf("credential scrub must not fire on a plain host:port, got %q", got)
	}
	if !strings.Contains(got, "storage.example.com") {
		t.Fatalf("host:port with no userinfo must survive untouched, got %q", got)
	}
}

// TestLastReasonScrubsAbsolutePath is the pre-existing behavior this fix must
// not regress: a bare host path with no credentials is still scrubbed to
// "[path]".
func TestLastReasonScrubsAbsolutePath(t *testing.T) {
	got := lastReason("error: failed to open /mnt/user/domains/Windows10/vdisk1.img: Permission denied")
	if strings.Contains(got, "/mnt/user") {
		t.Fatalf("lastReason leaked an absolute host path, got %q", got)
	}
	if !strings.Contains(got, "[path]") {
		t.Fatalf("lastReason should still scrub the absolute path, got %q", got)
	}
}
