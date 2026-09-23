package virshcli

import (
	"strings"
	"testing"
)

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

// credentialRe must not require a leading letter, or a numeric username lets
// its password through.
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

func TestLastReasonKeepsHostPort(t *testing.T) {
	got := lastReason("error: unable to connect to server at 'storage.example.com:16509': Connection refused")
	if strings.Contains(got, "[redacted]") {
		t.Fatalf("credential scrub must not fire on a plain host:port, got %q", got)
	}
	if !strings.Contains(got, "storage.example.com") {
		t.Fatalf("host:port with no userinfo must survive untouched, got %q", got)
	}
}

func TestLastReasonScrubsAbsolutePath(t *testing.T) {
	got := lastReason("error: failed to open /mnt/user/domains/Windows10/vdisk1.img: Permission denied")
	if strings.Contains(got, "/mnt/user") {
		t.Fatalf("lastReason leaked an absolute host path, got %q", got)
	}
	if !strings.Contains(got, "[path]") {
		t.Fatalf("lastReason should still scrub the absolute path, got %q", got)
	}
}
