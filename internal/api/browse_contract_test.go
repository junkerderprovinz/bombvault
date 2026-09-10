package api_test

// Browse node-listing CONTRACT tests — the additive Phase 1 extension of
// GET /api/browse that the Phase 2 lazy tree will expand against
// (BROWSE-01..04, decisions D-05..D-08).
//
// Deliberate layout: the endpoint tests in this file drive the REAL router via
// the shared newBrowseRouter/doJSON harness and assert the wire envelope
// (m["ok"], never status codes — the HTTP-200 envelope is the contract). The
// read-error classifier is unexported, so its table lives in the sibling
// white-box file browse_contract_internal_test.go (package api) and runs on
// every OS — including Windows dev, where neither a permission-denied fixture
// nor symlink creation is producible.
//
// The four pre-existing browse tests in handlers_test.go
// (TestBrowseListsMountRoot / TestBrowseListsSubpath / TestBrowseRejectsTraversal /
// TestBrowseRejectsAbsolutePath) are the FolderBrowser contract and MUST stay
// green unmodified: the extension is additive only (D-05), and the traversal /
// absolute-path rejection responses gain no new fields.

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// browseNames extracts the entry names from a browse response envelope in
// order, so tests can compare listings without re-decoding the shape.
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

// TestBrowseStatusOkOnEmpty pins the BROWSE-02 empty signal: listing an EMPTY
// directory returns entries:[] WITH status:"ok" — empty + ok is the real-empty
// signal, never an error. The tree needs this to show "nothing in here"
// instead of a failure state, and no hasChildren emptiness probe is involved
// (D-07: every directory is expandable).
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

// TestBrowseStatusMissing pins the missing half of the status trio: a path
// that does not exist returns the SAME generic scrubbed error string as any
// other read failure (the message never carries the path — threat T-01-08)
// with status:"missing" telling the tree the node vanished.
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

// TestBrowseStatusRestricted pins the restricted half of the status trio with
// a real permission-denied fixture (chmod 000). The classifier itself is
// table-tested on every OS in browse_contract_internal_test.go; this endpoint
// fixture proves the classification actually reaches the wire, which only a
// POSIX filesystem can produce — so it skips loudly on Windows dev and proves
// on Linux CI (the house POSIX-skip convention).
func TestBrowseStatusRestricted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0o000 does not restrict access on Windows — this fixture proves on Linux CI (house POSIX-skip convention)")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	if err := os.Mkdir(locked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	// Restore before TempDir cleanup so the removal can succeed (cleanup runs
	// LIFO: this one first, then the TempDir removal).
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) }) //nolint:gosec // G302: 0o700 is the restrictive DIRECTORY mode for this fixture (gosec assumes a file); the cleanup must restore it so TempDir removal succeeds

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

// TestBrowseSymlinkEscapeRejected pins the os.Root containment half of
// BROWSE-03 (threat T-01-05): a symlink planted INSIDE the mount root that
// points OUTSIDE it must be rejected by the listing instead of followed.
// paths.Resolve alone cannot see this (it is purely lexical) — this is the
// exact direct-address escape the pre-Root handler allowed. Symlink creation
// needs privileges on Windows dev, so the fixture skips loudly there and
// proves on Linux CI (RESEARCH Pitfall 5).
func TestBrowseSymlinkEscapeRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on Windows — this fixture proves on Linux CI (house POSIX-skip convention)")
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
	// The rejection must be indistinguishable from any other read failure:
	// the generic scrubbed message and the opaque status KIND — an escape
	// attempt must never announce itself (or its destination) on the wire.
	if m["error"] != "could not read directory" {
		t.Fatalf("error string must stay generic, got %v", m["error"])
	}
	if m["status"] != "error" {
		t.Fatalf("expected opaque status:\"error\" for the escape rejection, got %v", m)
	}
}

// TestBrowseHidden pins the BROWSE-04 hidden-entry contract as a pinned
// additive opt-in (D-05): the default call keeps today's byte-identical
// behavior (dot-prefixed entries excluded), ?hidden=1 includes them at the
// root and at every depth, and the two responses are otherwise identical and
// identically sorted — the flag filters, it never reorders or fabricates.
// A name that merely CONTAINS a dot mid-name must never be hidden: the rule
// is the dot PREFIX, nothing else.
func TestBrowseHidden(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{".hidden", ".cache", "my.dir", "appdata", "media"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	// A dot-FILE and a normal file: files are never listed, flag or no flag.
	if err := os.WriteFile(filepath.Join(root, ".dotfile"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A dot-prefixed child one level down: hidden=1 must include it at every
	// depth, not just the root listing.
	if err := os.MkdirAll(filepath.Join(root, "appdata", ".staging"), 0o700); err != nil {
		t.Fatal(err)
	}
	// An empty directory: [] with and without the flag.
	if err := os.Mkdir(filepath.Join(root, "emptydir"), 0o700); err != nil {
		t.Fatal(err)
	}

	h := newBrowseRouter(t, root)

	// Default (no flag): dot-prefixed entries excluded — today's behavior.
	_, def := doJSON(t, h, http.MethodGet, "/api/browse", "")
	if def["ok"] != true {
		t.Fatalf("expected ok:true, got %v", def)
	}
	wantDefault := []string{"appdata", "emptydir", "media", "my.dir"}
	if got := browseNames(def); !equalStrings(got, wantDefault) {
		t.Fatalf("default listing = %v, want %v", got, wantDefault)
	}

	// hidden=1 at the root: dot-dirs included, everything else identical and
	// identically sorted (lexical order includes the dot-dirs first).
	_, hid := doJSON(t, h, http.MethodGet, "/api/browse?hidden=1", "")
	if hid["ok"] != true {
		t.Fatalf("expected ok:true, got %v", hid)
	}
	wantHidden := []string{".cache", ".hidden", "appdata", "emptydir", "media", "my.dir"}
	if got := browseNames(hid); !equalStrings(got, wantHidden) {
		t.Fatalf("hidden=1 listing = %v, want %v", got, wantHidden)
	}

	// hidden=0 is NOT the opt-in: only the literal "1" turns it on.
	_, zero := doJSON(t, h, http.MethodGet, "/api/browse?hidden=0", "")
	if got := browseNames(zero); !equalStrings(got, wantDefault) {
		t.Fatalf("hidden=0 must behave as the default, got %v", got)
	}

	// hidden=1 at a subpath: dot-children included at every depth.
	_, sub := doJSON(t, h, http.MethodGet, "/api/browse?path=appdata&hidden=1", "")
	if sub["ok"] != true {
		t.Fatalf("expected ok:true, got %v", sub)
	}
	wantSub := []string{".staging"}
	if got := browseNames(sub); !equalStrings(got, wantSub) {
		t.Fatalf("hidden=1 subpath listing = %v, want %v", got, wantSub)
	}
	// ...and the same subpath without the flag stays empty (dot-child hidden).
	_, subDef := doJSON(t, h, http.MethodGet, "/api/browse?path=appdata", "")
	if got := browseNames(subDef); len(got) != 0 {
		t.Fatalf("default subpath listing must hide the dot-child, got %v", got)
	}

	// An empty directory returns [] with and without the flag — the flag never
	// fabricates entries.
	_, e1 := doJSON(t, h, http.MethodGet, "/api/browse?path=emptydir", "")
	if e1["ok"] != true || e1["status"] != "ok" || len(browseNames(e1)) != 0 {
		t.Fatalf("empty dir default = %v", e1)
	}
	_, e2 := doJSON(t, h, http.MethodGet, "/api/browse?path=emptydir&hidden=1", "")
	if e2["ok"] != true || e2["status"] != "ok" || len(browseNames(e2)) != 0 {
		t.Fatalf("empty dir with hidden=1 = %v (the flag must not fabricate entries)", e2)
	}
}

// equalStrings reports whether two string slices are element-wise equal.
// slices.Equal (not an index loop) — gosec G602 flags the constant-bounds
// pattern of a range-indexed compare, and the stdlib helper is the idiomatic
// equivalent with the bounds check built in.
func equalStrings(got, want []string) bool {
	return slices.Equal(got, want)
}

// TestBrowseCap pins the DoS cap (threat T-01-07, D-08 / RESEARCH R1): a
// directory with more than maxBrowseEntries entries returns exactly the
// lexically-FIRST maxBrowseEntries with truncated:true; exactly-capacity
// directories are NOT truncated. Fixtures use deterministic zero-padded names
// so "lexically first" is assertable.
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
	// The lexically-first 500: dir0000 .. dir0499 (dir0500 is dropped).
	if got[0] != "dir0000" || got[499] != "dir0499" {
		t.Fatalf("expected the lexically-first 500 (dir0000..dir0499), got first=%q last=%q", got[0], got[499])
	}

	// Exactly 500 entries: NOT truncated (the flag fires only on overflow).
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
