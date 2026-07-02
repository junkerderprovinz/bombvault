package api

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCleanupDrillSandboxMarkerGuard pins the safety interlock: cleanup deletes a
// sandbox ONLY when the .bombvault-drill marker is present in that exact dir. An
// unmarked dir is NEVER removed and yields an error — a guard against ever
// os.RemoveAll-ing a path that is not a drill sandbox we created.
func TestCleanupDrillSandboxMarkerGuard(t *testing.T) {
	t.Run("refuses an unmarked dir", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "not-a-drill")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		// A real file inside, to prove nothing is deleted.
		payload := filepath.Join(dir, "important.txt")
		if err := os.WriteFile(payload, []byte("keep me"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := cleanupDrillSandbox(dir); err == nil {
			t.Fatal("cleanup must refuse a dir without the .bombvault-drill marker")
		}
		if _, err := os.Stat(payload); err != nil {
			t.Fatalf("an unmarked dir must NOT be removed, stat=%v", err)
		}
	})
	t.Run("removes a marked sandbox", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "bombvault-drill-containers-123")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, drillMarkerName), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := cleanupDrillSandbox(dir); err != nil {
			t.Fatalf("cleanup of a marked sandbox must succeed: %v", err)
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("a marked sandbox must be removed, stat=%v", err)
		}
	})
}
