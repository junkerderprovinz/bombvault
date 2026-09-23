package api

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestEnsureDefsDirHealsTo0755: the defs directory lives on the backup share,
// which non-root SMB users copy off-box. ensureDefsDir creates it at 0755 and
// relaxes an existing 0700 one.
func TestEnsureDefsDirHealsTo0755(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not modelled on windows")
	}
	dir := filepath.Join(t.TempDir(), "bombvault-defs")

	if err := ensureDefsDir(dir); err != nil {
		t.Fatalf("ensureDefsDir (fresh): %v", err)
	}
	if perm := statPerm(t, dir); perm != 0o755 {
		t.Fatalf("fresh defs dir perm = %o, want 0755", perm)
	}

	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // G302: simulates a defs dir locked down to 0700
		t.Fatal(err)
	}
	if err := ensureDefsDir(dir); err != nil {
		t.Fatalf("ensureDefsDir (heal): %v", err)
	}
	if perm := statPerm(t, dir); perm != 0o755 {
		t.Fatalf("healed defs dir perm = %o, want 0755 (must relax a 0700 dir so the off-server sync tool can read it)", perm)
	}
}

// TestWriteDefHeals0600FileTo0644: os.WriteFile keeps an existing file's mode,
// so writeDef has to relax a 0600 .def to 0644 itself.
func TestWriteDefHeals0600FileTo0644(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not modelled on windows")
	}
	dir := t.TempDir()
	const fn = "plex.def"
	if err := os.WriteFile(filepath.Join(dir, fn), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeDef(dir, fn, []byte("new-encrypted")); err != nil {
		t.Fatalf("writeDef: %v", err)
	}
	if perm := statPerm(t, filepath.Join(dir, fn)); perm != 0o644 {
		t.Fatalf("def file perm = %o, want 0644 (must heal a pre-existing 0600 file)", perm)
	}
	if b, err := os.ReadFile(filepath.Join(dir, fn)); err != nil || string(b) != "new-encrypted" { //nolint:gosec // G304: fn is a test constant under a t.TempDir()
		t.Fatalf("content = %q (err %v), want the new bytes", b, err)
	}
}

// TestMakeRepoReadableRelaxesTree: makeRepoReadable gives group and other read
// access to a root-written restic tree, so an off-box sync tool can copy it.
func TestMakeRepoReadableRelaxesTree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not modelled on windows")
	}
	repo := t.TempDir()
	sub := filepath.Join(repo, "data", "00")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	pack := filepath.Join(sub, "deadbeef")
	if err := os.WriteFile(pack, []byte("packfile"), 0o600); err != nil {
		t.Fatal(err)
	}

	makeRepoReadable(repo, t.TempDir())

	if perm := statPerm(t, sub); perm&0o055 != 0o055 {
		t.Fatalf("dir perm %o must gain group+other traverse (rx)", perm)
	}
	if perm := statPerm(t, pack); perm&0o044 != 0o044 {
		t.Fatalf("file perm %o must gain group+other read", perm)
	}
	if b, err := os.ReadFile(pack); err != nil || string(b) != "packfile" { //nolint:gosec // G304: test path under t.TempDir()
		t.Fatalf("content must be untouched, got %q err %v", b, err)
	}
}

// TestMakeRepoReadablePreservesSetgid: a setgid bit, used for group inheritance
// on a shared NAS, survives while the read bits are added.
func TestMakeRepoReadablePreservesSetgid(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix mode bits are not modelled on windows")
	}
	repo := t.TempDir()
	sub := filepath.Join(repo, "data")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, os.ModeSetgid|0o700); err != nil { //nolint:gosec // G302: a setgid dir that makeRepoReadable must preserve
		t.Fatal(err)
	}

	makeRepoReadable(repo, t.TempDir())

	fi, err := os.Stat(sub)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSetgid == 0 {
		t.Fatalf("setgid bit must survive the relax, mode=%v", fi.Mode())
	}
	if fi.Mode().Perm()&0o055 != 0o055 {
		t.Fatalf("dir must still gain group+other rx, perm=%o", fi.Mode().Perm())
	}
}

// TestWriteDefIsAtomic: writeDef goes through a temp file and a rename, so
// nobody reads a half-written def, and leaves no .tmp behind.
func TestWriteDefIsAtomic(t *testing.T) {
	dir := t.TempDir()
	if err := writeDef(dir, "plex.def", []byte("ENC")); err != nil {
		t.Fatal(err)
	}
	if b := mustRead(t, filepath.Join(dir, "plex.def")); b != "ENC" {
		t.Fatalf("content = %q, want ENC", b)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

// TestReadStoredDefPrefersNewThenLegacy: readStoredDef falls back to the legacy
// sibling folder, so a restore from an old-layout backup still finds its
// definitions.
func TestReadStoredDefPrefersNewThenLegacy(t *testing.T) {
	base := t.TempDir()
	newDir := filepath.Join(base, "repo", "def")
	legacyDir := filepath.Join(base, "bombvault-defs")
	mustMkdir(t, newDir)
	mustMkdir(t, legacyDir)

	mustWrite(t, filepath.Join(legacyDir, "a.def"), "LEGACY")
	if b, err := readStoredDef(newDir, legacyDir, "a.def"); err != nil || string(b) != "LEGACY" {
		t.Fatalf("legacy fallback: %q %v", b, err)
	}
	mustWrite(t, filepath.Join(newDir, "a.def"), "NEW")
	if b, err := readStoredDef(newDir, legacyDir, "a.def"); err != nil || string(b) != "NEW" {
		t.Fatalf("prefer new: %q %v", b, err)
	}
	if _, err := readStoredDef(newDir, legacyDir, "missing.def"); err == nil {
		t.Fatal("a def missing from both dirs must error")
	}
}

// TestMigrateLegacyDefs: migrateLegacyDefs moves legacy defs into the repo,
// drops a legacy duplicate of a def already there, and removes the legacy dir
// once it is empty.
func TestMigrateLegacyDefs(t *testing.T) {
	base := t.TempDir()
	newDir := filepath.Join(base, "repo", "def")
	legacyDir := filepath.Join(base, "bombvault-defs")
	mustMkdir(t, newDir)
	mustMkdir(t, legacyDir)
	mustWrite(t, filepath.Join(legacyDir, "plex.def"), "P")
	mustWrite(t, filepath.Join(legacyDir, "sonarr.def"), "S")
	mustWrite(t, filepath.Join(newDir, "plex.def"), "P-NEW") // conflict: new wins, legacy dropped

	migrateLegacyDefs(newDir, legacyDir)

	if b := mustRead(t, filepath.Join(newDir, "plex.def")); b != "P-NEW" {
		t.Fatalf("conflict must keep the new def, got %q", b)
	}
	if b := mustRead(t, filepath.Join(newDir, "sonarr.def")); b != "S" {
		t.Fatalf("sonarr must be migrated, got %q", b)
	}
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Fatalf("emptied legacy dir must be removed, stat err=%v", err)
	}
}

func TestMigrateLegacyDefsLeavesForeignFiles(t *testing.T) {
	base := t.TempDir()
	newDir := filepath.Join(base, "repo", "def")
	legacyDir := filepath.Join(base, "bombvault-defs")
	mustMkdir(t, newDir)
	mustMkdir(t, legacyDir)
	mustWrite(t, filepath.Join(legacyDir, "a.def"), "A")
	mustWrite(t, filepath.Join(legacyDir, "keep.txt"), "x") // not a .def

	migrateLegacyDefs(newDir, legacyDir)

	if b := mustRead(t, filepath.Join(newDir, "a.def")); b != "A" {
		t.Fatalf("a.def must migrate, got %q", b)
	}
	if _, err := os.Stat(legacyDir); err != nil {
		t.Fatalf("legacy dir with a non-def file must survive: %v", err)
	}
	if b := mustRead(t, filepath.Join(legacyDir, "keep.txt")); b != "x" {
		t.Fatalf("foreign file must be untouched, got %q", b)
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // G304: test path under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func statPerm(t *testing.T, path string) os.FileMode {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Mode().Perm()
}
