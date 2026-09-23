package store_test

// Production code must not call Repo.UpdateSettings: a read-modify-write around
// it reverts every column another writer changed in between. No compiler or
// linter catches that, so this scans the non-test sources; tests may still use
// it to seed a row.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// moduleRoot walks up from the working directory to the one holding go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test's working directory")
		}
		dir = parent
	}
}

func TestNoProductionCallerUsesUpdateSettings(t *testing.T) {
	root := moduleRoot(t)
	var offenders []string

	for _, sub := range []string{"cmd", "internal"} {
		base := filepath.Join(root, sub)
		err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			name := info.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			// settings.go declares the method.
			if path == filepath.Join(root, "internal", "store", "settings.go") {
				return nil
			}
			src, rErr := os.ReadFile(path) //nolint:gosec // G304: paths come from walking the module's own source tree
			if rErr != nil {
				return rErr
			}
			for i, line := range strings.Split(string(src), "\n") {
				if strings.Contains(line, ".UpdateSettings(") {
					rel, _ := filepath.Rel(root, path)
					offenders = append(offenders, rel+":"+strconv.Itoa(i+1)+": "+strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", sub, err)
		}
	}

	if len(offenders) > 0 {
		t.Fatalf("production code must use store.MutateSettings, not the full-row UpdateSettings "+
			"(a read-modify-write around it reverts every column another writer changed in between):\n  %s",
			strings.Join(offenders, "\n  "))
	}
}
