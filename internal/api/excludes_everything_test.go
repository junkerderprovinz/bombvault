package api_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestBackupIgnoresAnExcludeThatWouldLeaveEverythingOut(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, _, appdata := appdataService(t, eng)
	if err := os.WriteFile(filepath.Join(appdata, "db.sqlite3"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	lines := []string{"/", "//", "/data/..", "*", "**", "/*", "/**/*", "*.log"}
	if err := svc.SetExcludes(context.Background(), "vw", lines); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Backup(context.Background(), "vw"); err != nil {
		t.Fatalf("backup: %v", err)
	}
	if !slices.Contains(eng.lastExcludes, "*.log") {
		t.Fatalf("excludes = %q, want the ordinary line passed on", eng.lastExcludes)
	}
	for _, line := range lines[:len(lines)-1] {
		if slices.Contains(eng.lastExcludes, line) {
			t.Fatalf("excludes = %q, want %q kept away from restic", eng.lastExcludes, line)
		}
	}
}
