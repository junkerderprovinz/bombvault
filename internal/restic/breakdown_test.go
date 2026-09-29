package restic

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLsNoLockArgsTakeNoLock(t *testing.T) {
	got := LsNoLockArgs("/repo", "abc123", Mode{Encrypted: true})
	want := []string{"-r", "/repo", "ls", "--no-lock", "--json", "--", "abc123"}
	if !slices.Equal(got, want) {
		t.Fatalf("args %q, want %q", got, want)
	}
}

// TestDiffStreamAndLsAgreeOnPaths runs the real binary: the size breakdown
// matches the paths diff reports against the ones ls lists, so both must
// spell a path the same way.
func TestDiffStreamAndLsAgreeOnPaths(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic not on PATH")
	}
	tmp := t.TempDir()
	repo, src := filepath.Join(tmp, "repo"), filepath.Join(tmp, "src")
	if err := os.MkdirAll(filepath.Join(src, "db"), 0o750); err != nil {
		t.Fatal(err)
	}
	put := func(name, body string) {
		if err := os.WriteFile(filepath.Join(src, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	put("keep.txt", "same")
	put("db/data.db", "one")
	r := Restic{Bin: "restic", CacheDir: filepath.Join(tmp, "cache")}
	m := Mode{Encrypted: true, Password: "contract"}
	ctx := context.Background()
	if err := r.Init(ctx, repo, m); err != nil {
		t.Fatal(err)
	}
	backup := func() string {
		out, err := exec.CommandContext(ctx, "restic", "-r", repo, "backup", "--json", "--no-scan", src).Output() //nolint:gosec // test-local paths
		if err != nil {
			t.Fatalf("backup: %v", err)
		}
		sum, err := ParseBackupSummary(out)
		if err != nil {
			t.Fatal(err)
		}
		return sum.SnapshotID
	}
	t.Setenv("RESTIC_PASSWORD", "contract")
	first := backup()
	put("db/data.db", "two, longer")
	put("new.txt", "new")
	second := backup()

	var changes []DiffChange
	if err := r.DiffStream(ctx, repo, first, second, m, func(c DiffChange) { changes = append(changes, c) }); err != nil {
		t.Fatal(err)
	}
	listed := map[string]FileEntry{}
	if err := r.LsStreamNoLock(ctx, repo, second, m, func(e FileEntry) { listed[e.Path] = e }); err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, c := range changes {
		if strings.HasSuffix(c.Path, "/") {
			continue
		}
		e, ok := listed[c.Path]
		if !ok {
			b, _ := json.Marshal(listed)
			t.Fatalf("diff path %q is not in the listing %s", c.Path, b)
		}
		found[filepath.Base(e.Path)] = c.Modifier
	}
	if found["new.txt"] != "+" || !strings.Contains(found["data.db"], "M") {
		t.Fatalf("changes %+v", changes)
	}
	if _, ok := found["keep.txt"]; ok {
		t.Fatalf("an unchanged file is reported: %+v", changes)
	}
}
