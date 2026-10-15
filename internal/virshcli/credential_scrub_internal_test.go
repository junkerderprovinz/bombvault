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

func TestLastReasonScrubsAPasswordWithoutAUserName(t *testing.T) {
	got := lastReason(`error: failed to connect to the hypervisor "//:hunter2@tower/system"`)
	if strings.Contains(got, "hunter2") {
		t.Fatalf("lastReason leaked a password with no user name, got %q", got)
	}
}

func TestLastReasonKeepsThePunctuationAfterAPath(t *testing.T) {
	got := lastReason("error: cannot access storage file (/mnt/user/domains/vm/vdisk1.img).")
	if want := "error: cannot access storage file ([path])."; got != want {
		t.Fatalf("lastReason = %q, want %q", got, want)
	}
}

func TestLastReasonScrubsPathsButNotDatasetNames(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"zvol dataset", "error: cannot use zvol tank/vms/testvm", "error: cannot use zvol tank/vms/testvm"},
		{"quoted zvol device", "error: Cannot access storage file '/dev/zvol/tank/vms/testvm': No such file", "error: Cannot access storage file '[path]': No such file"},
		{"zvol device in parentheses", "error: cannot open disk (/dev/zvol/tank/vms/testvm).", "error: cannot open disk ([path])."},
		{"path after an equals sign", "error: unsupported configuration: path=/mnt/user/domains/vm", "error: unsupported configuration: path=[path]"},
		{"libvirt uri", "error: failed to connect to qemu+ssh://root@tower/system", "error: failed to connect to qemu+ssh:[path]"},
		{"password with a slash", "error: failed to connect to qemu+ssh://root:pa/ss@tower/system", "error: failed to connect to [redacted]@tower/system"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := lastReason(tc.in); got != tc.want {
				t.Fatalf("lastReason(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
