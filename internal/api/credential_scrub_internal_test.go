package api

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

// TestScrubErrorScrubsURLCredentials: absPathRe stops at the first ":", so on
// its own it never reaches the userinfo of a repo URL such as
// rest:https://user:pass@host:port/path, the shape the deploy recipe produces.
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

// TestCredentialReDoesNotEatHostPort: the credential scrub needs a "user:pass@"
// segment, so a plain host:port survives.
func TestCredentialReDoesNotEatHostPort(t *testing.T) {
	got := scrubError(errors.New("unable to reach storage.example.com:8000: connection refused"))
	if strings.Contains(got, "[redacted]") {
		t.Fatalf("credential scrub must not fire on a plain host:port, got %q", got)
	}
	if !strings.Contains(got, "storage.example.com") {
		t.Fatalf("host:port with no userinfo must survive untouched, got %q", got)
	}
}

// TestScrubErrorScrubsNumericUsername: a numeric username is legal in a restic
// or rclone URL, and it and its password must be scrubbed like any other.
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

// TestTruncateRunErrScrubsCredentials: an error outside the sentinel bypass has
// its URL credentials scrubbed, as in scrubError.
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

// TestTruncateRunErrBypassesRestoreConflict: backup.ErrRestoreConflict lists
// host ports such as "8080/tcp", which absPathRe takes for a path. scrubError
// passes it through verbatim via scrubBypassMessage, and truncateRunErr has to
// do the same.
func TestTruncateRunErrBypassesRestoreConflict(t *testing.T) {
	err := fmt.Errorf("%w. Free these and retry: %s", backup.ErrRestoreConflict,
		`host port 8080/tcp is already used by container "other-app"`)

	// scrubError is the baseline.
	if got := scrubError(err); strings.Contains(got, "[path]") {
		t.Fatalf("test setup: scrubError itself mangled the conflict text, got %q; fix the fixture", got)
	}

	got := truncateRunErr(err)
	if strings.Contains(got, "[path]") {
		t.Fatalf("truncateRunErr mangled the restore-conflict text into [path], got %q", got)
	}
	if !strings.Contains(got, "8080/tcp") {
		t.Fatalf("truncateRunErr must preserve the literal host:port conflict text, got %q", got)
	}
}

// TestTruncateRunErrBypassesZvolRebaseFailed: a zvol rebase failure names a ZFS
// dataset ("<pool>/<rest>"), which truncateRunErr must keep verbatim, as
// scrubError does.
func TestTruncateRunErrBypassesZvolRebaseFailed(t *testing.T) {
	err := fmt.Errorf("rebase dataset %q onto pool %q: %w", "tank/vms/zvolvm/disk1", "-badpool", errZvolRebaseFailed)

	if got := scrubError(err); strings.Contains(got, "[path]") {
		t.Fatalf("test setup: scrubError itself mangled the dataset name, got %q; fix the fixture", got)
	}

	got := truncateRunErr(err)
	if strings.Contains(got, "[path]") {
		t.Fatalf("truncateRunErr mangled the ZFS dataset name into [path], got %q", got)
	}
	if !strings.Contains(got, "tank/vms/zvolvm/disk1") {
		t.Fatalf("truncateRunErr must preserve the literal ZFS dataset name, got %q", got)
	}
}

// TestTruncateRunErrScrubsDockerSocketPath: a Docker connection error naming
// its socket path is not one of the bypassed sentinels, so it is scrubbed
// exactly as scrubError scrubs it. Bypassing any text with slashes in it would
// let unreviewed messages through unscrubbed.
func TestTruncateRunErrScrubsDockerSocketPath(t *testing.T) {
	err := fmt.Errorf("inspect container: %w", errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock"))

	got := truncateRunErr(err)
	if strings.Contains(got, "unix:///var/run/docker.sock") {
		t.Fatalf("expected the socket path to still be scrubbed (parity with scrubError), got %q", got)
	}
	if !strings.Contains(got, "Cannot connect to the Docker daemon") {
		t.Fatalf("truncateRunErr should keep the actual cause, got %q", got)
	}
	if want, gotScrub := scrubError(err), got; want != gotScrub {
		t.Fatalf("truncateRunErr and scrubError diverged on a non-bypassed error: scrubError=%q truncateRunErr=%q", want, gotScrub)
	}
}

// TestScrubSecretsPreservesHostname: paths are scrubbed before credentials. The
// other way round, credentialRe leaves "scheme://[redacted]@host", which
// absPathRe then removes along with the hostname, and nobody can tell which
// off-site target failed.
func TestScrubSecretsPreservesHostname(t *testing.T) {
	got := scrubSecrets(`rest:https://backupuser:Tr0ub4dor&3@storage.example.com:8000/containers`)
	if strings.Contains(got, "Tr0ub4dor") || strings.Contains(got, "backupuser") {
		t.Fatalf("scrubSecrets leaked the credentials, got %q", got)
	}
	if !strings.Contains(got, "storage.example.com") {
		t.Fatalf("scrubSecrets destroyed the hostname an operator needs to diagnose which off-site target failed, got %q", got)
	}
}
