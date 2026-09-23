package restic

import (
	"errors"
	"os/exec"
	"runtime"
	"strconv"
	"testing"
)

// TestBackupExit3Warning checks that a backup exiting with code 3, where restic
// could not read some source files but still wrote the snapshot, is the
// ErrBackupSourceUnreadable warning. Any other code, and any other command,
// stays a failure.
func TestBackupExit3Warning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell to produce a real *exec.ExitError with a chosen code")
	}
	exitErr := func(code int) error {
		return exec.Command("sh", "-c", "exit "+strconv.Itoa(code)).Run() //nolint:gosec // G204: test-only; the argument is a hardcoded int from this test
	}

	if got := backupExit3Err([]string{"backup", "/data"}, exitErr(3),
		"Warning: at least one source file could not be read"); !errors.Is(got, ErrBackupSourceUnreadable) {
		t.Fatalf("exit-3 backup: want ErrBackupSourceUnreadable, got %v", got)
	}
	if got := backupExit3Err([]string{"backup", "/data"}, exitErr(1),
		"Fatal: unable to open repository"); got != nil {
		t.Fatalf("exit-1 backup: want nil (hard failure), got %v", got)
	}
	if got := backupExit3Err([]string{"check"}, exitErr(3), ""); got != nil {
		t.Fatalf("exit-3 non-backup: want nil, got %v", got)
	}
	if got := backupExit3Err([]string{"backup", "/data"}, nil, ""); got != nil {
		t.Fatalf("nil err: want nil, got %v", got)
	}
}
