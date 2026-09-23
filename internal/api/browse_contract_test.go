package api_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// browseNames returns the entry names of a browse response in order.
func browseNames(m map[string]any) []string {
	dirsRaw, _ := m["dirs"].([]any)
	names := make([]string, 0, len(dirsRaw))
	for _, d := range dirsRaw {
		dm, _ := d.(map[string]any)
		if n, ok := dm["name"].(string); ok {
			names = append(names, n)
		}
	}
	return names
}

// TestBrowseStatusOkOnEmpty: an empty directory is ok with no entries rather
// than an error, so the tree can show that there is nothing in it.
func TestBrowseStatusOkOnEmpty(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "emptydir"), 0o700); err != nil {
		t.Fatal(err)
	}

	h := newBrowseRouter(t, root)
	w, m := doJSON(t, h, http.MethodGet, "/api/browse?path=emptydir", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != true {
		t.Fatalf("expected ok:true for an empty dir, got %v", m)
	}
	if m["status"] != "ok" {
		t.Fatalf("expected status:\"ok\" for an empty dir, got %v", m)
	}
	if got := browseNames(m); len(got) != 0 {
		t.Fatalf("expected zero children, got %v", got)
	}
}

// TestBrowseStatusMissing: a missing path gets the same generic error as any
// other read failure, without the path in it, and status "missing".
func TestBrowseStatusMissing(t *testing.T) {
	root := t.TempDir()

	h := newBrowseRouter(t, root)
	w, m := doJSON(t, h, http.MethodGet, "/api/browse?path=does-not-exist", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != false {
		t.Fatalf("expected ok:false for a missing path, got %v", m)
	}
	if m["error"] != "could not read directory" {
		t.Fatalf("error string must stay verbatim, got %v", m["error"])
	}
	if m["status"] != "missing" {
		t.Fatalf("expected status:\"missing\", got %v", m)
	}
}

// TestBrowseStatusRestricted checks that a permission error reaches the wire as
// "restricted". It needs POSIX permissions and a non-root user; the classifier
// itself is tested on every OS in browse_contract_internal_test.go.
func TestBrowseStatusRestricted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0o000 does not restrict access on Windows; this runs on Linux CI")
	}
	// Root ignores permission bits (CAP_DAC_OVERRIDE).
	if os.Geteuid() == 0 {
		t.Skip("root ignores permission bits; the restricted status needs a non-root runner")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	if err := os.Mkdir(locked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	// Cleanups run last in, first out, so the mode is back before TempDir
	// removes the directory.
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) }) //nolint:gosec // G302: 0o700 is a directory mode; gosec assumes a file

	h := newBrowseRouter(t, root)
	w, m := doJSON(t, h, http.MethodGet, "/api/browse?path=locked", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != false {
		t.Fatalf("expected ok:false for a permission-denied dir, got %v", m)
	}
	if m["error"] != "could not read directory" {
		t.Fatalf("error string must stay verbatim, got %v", m["error"])
	}
	if m["status"] != "restricted" {
		t.Fatalf("expected status:\"restricted\", got %v", m)
	}
}

// TestBrowseSymlinkEscapeRejected: a symlink inside the mount root that points
// outside it is rejected, not followed. paths.Resolve is purely lexical and
// cannot catch this; os.Root does.
func TestBrowseSymlinkEscapeRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating a symlink needs privileges on Windows; this runs on Linux CI")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "appdata"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc", filepath.Join(root, "appdata", "escape")); err != nil {
		t.Fatal(err)
	}

	h := newBrowseRouter(t, root)
	w, m := doJSON(t, h, http.MethodGet, "/api/browse?path=appdata/escape", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != false {
		t.Fatalf("symlink escape must be rejected, got ok:true %v", m)
	}
	// An escape has to look like any other read failure and reveal nothing
	// about its target.
	if m["error"] != "could not read directory" {
		t.Fatalf("error string must stay generic, got %v", m["error"])
	}
	if m["status"] != "error" {
		t.Fatalf("expected opaque status:\"error\" for the escape rejection, got %v", m)
	}
}

// TestBrowseHidden: ?hidden=1 adds dot-prefixed directories at every depth and
// changes nothing else. Only a leading dot hides a name.
func TestBrowseHidden(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{".hidden", ".cache", "my.dir", "appdata", "media"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	// Files are never listed, with or without the flag.
	if err := os.WriteFile(filepath.Join(root, ".dotfile"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "appdata", ".staging"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "emptydir"), 0o700); err != nil {
		t.Fatal(err)
	}

	h := newBrowseRouter(t, root)

	_, def := doJSON(t, h, http.MethodGet, "/api/browse", "")
	if def["ok"] != true {
		t.Fatalf("expected ok:true, got %v", def)
	}
	wantDefault := []string{"appdata", "emptydir", "media", "my.dir"}
	if got := browseNames(def); !slices.Equal(got, wantDefault) {
		t.Fatalf("default listing = %v, want %v", got, wantDefault)
	}

	// Dot directories sort last. Sorted first, they would take the front of a
	// capped 500-entry page and push ordinary folders off the end, where they
	// could not be ticked in the selection tree.
	_, hid := doJSON(t, h, http.MethodGet, "/api/browse?hidden=1", "")
	if hid["ok"] != true {
		t.Fatalf("expected ok:true, got %v", hid)
	}
	wantHidden := []string{"appdata", "emptydir", "media", "my.dir", ".cache", ".hidden"}
	if got := browseNames(hid); !slices.Equal(got, wantHidden) {
		t.Fatalf("hidden=1 listing = %v, want %v", got, wantHidden)
	}

	// Only the literal "1" turns it on.
	_, zero := doJSON(t, h, http.MethodGet, "/api/browse?hidden=0", "")
	if got := browseNames(zero); !slices.Equal(got, wantDefault) {
		t.Fatalf("hidden=0 must behave as the default, got %v", got)
	}

	_, sub := doJSON(t, h, http.MethodGet, "/api/browse?path=appdata&hidden=1", "")
	if sub["ok"] != true {
		t.Fatalf("expected ok:true, got %v", sub)
	}
	wantSub := []string{".staging"}
	if got := browseNames(sub); !slices.Equal(got, wantSub) {
		t.Fatalf("hidden=1 subpath listing = %v, want %v", got, wantSub)
	}
	_, subDef := doJSON(t, h, http.MethodGet, "/api/browse?path=appdata", "")
	if got := browseNames(subDef); len(got) != 0 {
		t.Fatalf("default subpath listing must hide the dot-child, got %v", got)
	}

	_, e1 := doJSON(t, h, http.MethodGet, "/api/browse?path=emptydir", "")
	if e1["ok"] != true || e1["status"] != "ok" || len(browseNames(e1)) != 0 {
		t.Fatalf("empty dir default = %v", e1)
	}
	_, e2 := doJSON(t, h, http.MethodGet, "/api/browse?path=emptydir&hidden=1", "")
	if e2["ok"] != true || e2["status"] != "ok" || len(browseNames(e2)) != 0 {
		t.Fatalf("empty dir with hidden=1 = %v (the flag must not fabricate entries)", e2)
	}
}

// TestBrowseCap: a directory with more than maxBrowseEntries entries returns
// the first maxBrowseEntries by name with truncated:true, and one with exactly
// that many is not truncated.
func TestBrowseCap(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 501; i++ {
		if err := os.MkdirAll(filepath.Join(root, fmt.Sprintf("dir%04d", i)), 0o700); err != nil {
			t.Fatal(err)
		}
	}

	h := newBrowseRouter(t, root)
	w, m := doJSON(t, h, http.MethodGet, "/api/browse", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != true {
		t.Fatalf("expected ok:true, got %v", m)
	}
	got := browseNames(m)
	if len(got) != 500 {
		t.Fatalf("expected exactly 500 entries, got %d", len(got))
	}
	if m["truncated"] != true {
		t.Fatalf("expected truncated:true for 501 entries, got %v", m)
	}
	// dir0500 is the one dropped.
	if got[0] != "dir0000" || got[499] != "dir0499" {
		t.Fatalf("expected the lexically-first 500 (dir0000..dir0499), got first=%q last=%q", got[0], got[499])
	}

	full := t.TempDir()
	for i := 0; i < 500; i++ {
		if err := os.MkdirAll(filepath.Join(full, fmt.Sprintf("dir%04d", i)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	h2 := newBrowseRouter(t, full)
	_, m2 := doJSON(t, h2, http.MethodGet, "/api/browse", "")
	if m2["ok"] != true {
		t.Fatalf("expected ok:true, got %v", m2)
	}
	if got2 := browseNames(m2); len(got2) != 500 {
		t.Fatalf("expected 500 entries, got %d", len(got2))
	}
	if m2["truncated"] != false {
		t.Fatalf("expected truncated:false for exactly 500 entries, got %v", m2)
	}
}
