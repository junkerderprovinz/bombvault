package store_test

// Convention guard: production code must not call Repo.UpdateSettings.
//
// UpdateSettings writes the WHOLE settings row from the struct it is handed.
// Paired with GetSettings that is a lost-update bug wearing a patch's clothes:
// every column another writer changed between the read and the write is
// reverted, and the writer that lost is told it succeeded. That is not a
// hypothetical — it is what made a minutes-long encryption probe undo the
// backup paths a user had just saved, and what made a settings save wipe the
// named cloud-credential sets. Both were one instance each of the same shape.
//
// MutateSettings closes the window structurally (read, mutate and write in one
// serialized transaction), so the fix only stays fixed if nothing reintroduces
// the pairing. Tests keep using UpdateSettings freely — seeding a whole row is
// exactly what it is for — so the guard is scoped to non-test sources.
//
// This is a source scan, deliberately: the pairing is a shape, not a type
// error, so no compiler or linter available here can refuse it.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// moduleRoot walks up from the test's working directory to the directory
// holding go.mod, so the scan below covers the whole module rather than this
// one package.
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
			// The declaration itself lives here; it is the thing being guarded,
			// not a caller of it.
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
