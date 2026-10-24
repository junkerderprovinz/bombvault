package dockercli

import (
	"errors"
	"fmt"
	"testing"
)

func TestAHookExitCodeCanBeReadThroughAWrappedError(t *testing.T) {
	err := fmt.Errorf("backup: pre-hook: %w", &ExitError{Code: 75, Reason: "busy"})
	var ec interface{ ExitCode() int }
	if !errors.As(err, &ec) || ec.ExitCode() != 75 {
		t.Fatalf("err = %v does not carry its exit code", err)
	}
	if err.Error() != "backup: pre-hook: hook exited 75: busy" {
		t.Fatalf("message = %q", err.Error())
	}
}
