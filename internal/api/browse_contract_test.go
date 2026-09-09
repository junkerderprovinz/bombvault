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
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

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
