package restic

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestDerivedExcludePatternsAreLiteral shows why internal/api's
// escapeGlobLiteral exists: restic reads an --exclude value as a glob, and the
// tree builds those values from folder names the user unticked. Unescaped,
// "Inception (2010) [1080p]" stays in the snapshot because "[1080p]" is a
// one-character class, "Season [01]" drops its siblings "Season 0" and
// "Season 1" but keeps itself, and "Movies [2024" is an invalid pattern that
// fails the whole backup. The escaped forms exclude exactly the named folder.
func TestDerivedExcludePatternsAreLiteral(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic not on PATH")
	}
	if runtime.GOOS == "windows" {
		t.Skip("these folder names are not creatable on Windows (? and * are reserved)")
	}
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	src := filepath.Join(tmp, "src")
	names := []string{"Inception (2010) [1080p]", "Season [01]", "Season 0", "Season 1", "star*name", "starXname", "plain"}
	for _, n := range names {
		if err := os.MkdirAll(filepath.Join(src, n), 0o755); err != nil { //nolint:gosec // G301: test temp dir
			t.Fatalf("mkdir %q: %v", n, err)
		}
		if err := os.WriteFile(filepath.Join(src, n, "f.txt"), []byte(n), 0o644); err != nil { //nolint:gosec // G306: test file
			t.Fatalf("write %q: %v", n, err)
		}
	}

	env := append(os.Environ(), "RESTIC_PASSWORD=contract")
	run := func(args ...string) (string, error) {
		cmd := exec.CommandContext(context.Background(), "restic", args...) //nolint:gosec // fixed args, test-local paths
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run("-r", repo, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}

	// backupWith runs one backup with a single --exclude and returns the set of
	// top-level folder names that made it into the snapshot.
	backupWith := func(t *testing.T, pattern string) map[string]bool {
		t.Helper()
		if out, err := run("-r", repo, "backup", "--no-scan", "--exclude", pattern, src); err != nil {
			t.Fatalf("backup with --exclude %q: %v\n%s", pattern, err, out)
		}
		out, err := run("-r", repo, "ls", "latest")
		if err != nil {
			t.Fatalf("ls: %v\n%s", err, out)
		}
		got := map[string]bool{}
		for _, line := range strings.Split(out, "\n") {
			rest, ok := strings.CutPrefix(strings.TrimSpace(line), src+"/")
			if !ok {
				continue
			}
			if name, _, found := strings.Cut(rest, "/"); found {
				got[name] = true
			}
		}
		return got
	}

	// A copy of internal/api's escaping, kept separate so the test measures
	// restic independently of the code it backs.
	escape := func(p string) string {
		var b strings.Builder
		for _, r := range p {
			if strings.ContainsRune(`\*?[]`, r) {
				b.WriteByte('\\')
			}
			b.WriteRune(r)
		}
		return b.String()
	}

	t.Run("a raw bracketed name does not exclude its own folder", func(t *testing.T) {
		in := backupWith(t, filepath.Join(src, "Inception (2010) [1080p]"))
		if !in["Inception (2010) [1080p]"] {
			t.Fatal("the raw pattern DID exclude its folder: restic no longer reads --exclude as a glob, and the escaping is now unnecessary")
		}
	})

	t.Run("the escaped name excludes exactly its own folder", func(t *testing.T) {
		in := backupWith(t, escape(filepath.Join(src, "Inception (2010) [1080p]")))
		if in["Inception (2010) [1080p]"] {
			t.Fatal("the escaped pattern did not exclude the folder it names")
		}
		if !in["plain"] || !in["Season 0"] {
			t.Fatalf("the escaped pattern took unrelated folders with it: %v", in)
		}
	})

	t.Run("a raw class hits the siblings and spares itself", func(t *testing.T) {
		in := backupWith(t, filepath.Join(src, "Season [01]"))
		if in["Season 0"] || in["Season 1"] {
			t.Fatalf("expected the raw class to swallow the siblings, got %v", in)
		}
		if !in["Season [01]"] {
			t.Fatal("expected the raw class to spare the folder it came from")
		}
	})

	t.Run("the escaped class spares the siblings", func(t *testing.T) {
		in := backupWith(t, escape(filepath.Join(src, "Season [01]")))
		if !in["Season 0"] || !in["Season 1"] {
			t.Fatalf("the escaped pattern must leave the siblings alone, got %v", in)
		}
		if in["Season [01]"] {
			t.Fatal("the escaped pattern did not exclude the folder it names")
		}
	})

	t.Run("a raw star takes a similarly named folder with it", func(t *testing.T) {
		in := backupWith(t, filepath.Join(src, "star*name"))
		if in["starXname"] {
			t.Fatal("expected the raw star to match starXname too")
		}
		escaped := backupWith(t, escape(filepath.Join(src, "star*name")))
		if !escaped["starXname"] {
			t.Fatalf("the escaped star must leave starXname alone, got %v", escaped)
		}
		if escaped["star*name"] {
			t.Fatal("the escaped star did not exclude the folder it names")
		}
	})

	t.Run("an unmatched bracket fails the whole run, escaped it does not", func(t *testing.T) {
		out, err := run("-r", repo, "backup", "--no-scan", "--exclude", filepath.Join(src, "Movies [2024"), src)
		if err == nil {
			t.Fatal("expected restic to refuse an invalid pattern outright")
		}
		if !strings.Contains(out, "invalid pattern") {
			t.Fatalf("expected the invalid-pattern refusal, got: %s", out)
		}
		if out, err := run("-r", repo, "backup", "--no-scan", "--exclude", escape(filepath.Join(src, "Movies [2024")), src); err != nil {
			t.Fatalf("the escaped form must be a valid pattern: %v\n%s", err, out)
		}
	})
}
