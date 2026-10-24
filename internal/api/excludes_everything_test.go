package api_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func TestBackupIgnoresAnExcludeThatWouldLeaveABackupFolderOut(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, root, appdata := appdataService(t, eng)
	if err := os.WriteFile(filepath.Join(appdata, "db.sqlite3"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	lines := []string{root, root + "/disks", appdata, appdata + "/*", "/data", "/data/", "/data/*", "appdata", "/data/db.sqlite3"}
	if err := svc.SetExcludes(context.Background(), "vw", lines); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Backup(context.Background(), "vw"); err != nil {
		t.Fatalf("backup: %v", err)
	}
	if len(eng.lastExcludes) != 1 || !strings.HasSuffix(eng.lastExcludes[0], "/vw/db.sqlite3") {
		t.Fatalf("excludes = %q, want only the file inside the backed-up folder", eng.lastExcludes)
	}
}
