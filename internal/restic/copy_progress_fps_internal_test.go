package restic

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

// TestCopySetsProgressFPSWhenSinkPresent checks that Copy sets
// RESTIC_PROGRESS_FPS when a CopySink is installed. restic prints periodic
// progress only to a TTY or with that variable set, and BombVault's stdout is
// a pipe. The fake restic writes the value it sees to a file.
func TestCopySetsProgressFPSWhenSinkPresent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell to exec a shebang script as the fake restic binary")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "fake-restic.sh")
	outFile := filepath.Join(dir, "fps.out")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$RESTIC_PROGRESS_FPS\" > \"$FPS_OUT_FILE\"\n"), 0o700); err != nil { //nolint:gosec // G306: test-only helper script, needs the exec bit
		t.Fatalf("write fake restic script: %v", err)
	}
	t.Setenv("FPS_OUT_FILE", outFile)

	r := Restic{Bin: script}

	t.Run("with sink sets RESTIC_PROGRESS_FPS=3", func(t *testing.T) {
		ctx := progress.WithCopySink(context.Background(), func(progress.CopyProgress) {})
		if err := r.Copy(ctx, "dest-repo", "src-repo", nil, Limits{}, Mode{}); err != nil {
			t.Fatalf("Copy: %v", err)
		}
		got, err := os.ReadFile(outFile) //nolint:gosec // G304: outFile is a path this test builds under its own t.TempDir(), not user input
		if err != nil {
			t.Fatalf("read fps output: %v", err)
		}
		if fps := strings.TrimSpace(string(got)); fps != "3" {
			t.Fatalf("RESTIC_PROGRESS_FPS = %q, want %q when a CopySink is present", fps, "3")
		}
	})

	t.Run("without sink leaves RESTIC_PROGRESS_FPS unset", func(t *testing.T) {
		if err := os.Remove(outFile); err != nil && !os.IsNotExist(err) {
			t.Fatalf("reset fps output: %v", err)
		}
		if err := r.Copy(context.Background(), "dest-repo", "src-repo", nil, Limits{}, Mode{}); err != nil {
			t.Fatalf("Copy: %v", err)
		}
		got, err := os.ReadFile(outFile) //nolint:gosec // G304: outFile is a path this test builds under its own t.TempDir(), not user input
		if err != nil {
			t.Fatalf("read fps output: %v", err)
		}
		if fps := strings.TrimSpace(string(got)); fps != "" {
			t.Fatalf("RESTIC_PROGRESS_FPS = %q, want unset (empty) when no CopySink is present", fps)
		}
	})
}
