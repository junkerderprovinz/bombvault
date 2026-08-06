package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// TestForeignOpenCloseRoutes pins the Task 9 route wiring end-to-end: POST
// /api/foreign/open answers {ok, session, inventory} (the exact envelope the
// Recovery card consumes in Task 11), a bad key fails gracefully without
// echoing the key, and POST /api/foreign/close always succeeds.
func TestForeignOpenCloseRoutes(t *testing.T) {
	enc := true
	eng := &fakeResticEngine{
		existingMode: &enc, // the foreign repo "exists" and opens with the encrypted probe
		snaps: []restic.Snapshot{
			{ID: "aaaaaaaa11111111", Time: "2026-07-01T10:00:00Z", Tags: []string{"container:web"}},
			{ID: "bbbbbbbb22222222", Time: "2026-07-02T10:00:00Z", Tags: []string{"fileset:docs"}},
		},
	}
	h, _ := newTestRouter(t, &fakeServiceDocker{}, eng)

	key := strings.Repeat("ab", 32) // 64 lowercase hex — a valid-shaped foreign APP_KEY
	w, m := doJSON(t, h, http.MethodPost, "/api/foreign/open", `{"location":"backups/other","key":"`+key+`"}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("open: status=%d body=%v", w.Code, m)
	}
	session, _ := m["session"].(string)
	if session == "" {
		t.Fatalf("open must return a session id, got %v", m)
	}
	inv, _ := m["inventory"].(map[string]any)
	if inv == nil {
		t.Fatalf("open must return the inventory, got %v", m)
	}
	if containers, _ := inv["containers"].([]any); len(containers) != 1 {
		t.Fatalf("inventory containers = %v, want the one tagged item", inv["containers"])
	}
	if fileSets, _ := inv["fileSets"].([]any); len(fileSets) != 1 {
		t.Fatalf("inventory fileSets = %v, want the one tagged item", inv["fileSets"])
	}
	if vms, _ := inv["vms"].([]any); vms == nil || len(vms) != 0 {
		t.Fatalf("inventory vms = %v, want []", inv["vms"])
	}

	// A malformed key fails gracefully — and the response never echoes the key.
	w, m = doJSON(t, h, http.MethodPost, "/api/foreign/open", `{"location":"backups/other","key":"SECRETBUTWRONG"}`)
	if w.Code != http.StatusOK || m["ok"] != false {
		t.Fatalf("bad-key open: status=%d body=%v", w.Code, m)
	}
	if msg, _ := m["error"].(string); strings.Contains(msg, "SECRETBUTWRONG") {
		t.Fatalf("error must not echo the key: %q", msg)
	}

	// Close succeeds for the live session and is a no-op for unknown ids.
	w, m = doJSON(t, h, http.MethodPost, "/api/foreign/close", `{"session":"`+session+`"}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("close: status=%d body=%v", w.Code, m)
	}
	w, m = doJSON(t, h, http.MethodPost, "/api/foreign/close", `{"session":"unknown"}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("close unknown: status=%d body=%v", w.Code, m)
	}
}

// TestForeignRestoreRoute pins the Task 10 route wiring end-to-end (the exact
// contract the Recovery card consumes in Task 11): POST /api/foreign/restore
// with {session, domain, item, snapshot, confirm, target} answers 200
// {ok:true, started:true} and the detached restic work reads the SESSION repo;
// an unconfirmed restore and an unknown session answer 400 (nothing started);
// a second restore while one is running answers 409.
func TestForeignRestoreRoute(t *testing.T) {
	enc := true
	location := "backups/other" // a LOCAL mounted share (the only kind OpenForeign accepts)
	eng := &fakeResticEngine{
		existingMode: &enc,
		snaps: []restic.Snapshot{
			{ID: "eeeeeeee55555555", Time: "2026-07-05T10:00:00Z", Tags: []string{"fileset:docs"}},
		},
	}
	h, _, svc, dir := newTestRouterSvcDir(t, &fakeServiceDocker{}, eng)

	// Seed the foreign repo's config marker so the session's snapshot listing
	// reaches the engine (a local repo with no config marker reads as "missing").
	sessionRepo, err := paths.Resolve(dir, location)
	if err != nil {
		t.Fatalf("resolve session repo: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "backups", "other"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "backups", "other", "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	key := strings.Repeat("ab", 32)
	w, m := doJSON(t, h, http.MethodPost, "/api/foreign/open", `{"location":"`+location+`","key":"`+key+`"}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("open: status=%d body=%v", w.Code, m)
	}
	session, _ := m["session"].(string)
	if session == "" {
		t.Fatalf("open must return a session id, got %v", m)
	}

	// Unconfirmed → 400 with the familiar sentinel text; nothing starts.
	w, m = doJSON(t, h, http.MethodPost, "/api/foreign/restore",
		`{"session":"`+session+`","domain":"files","item":"docs","snapshot":"latest","confirm":false,"target":"restore-here/docs"}`)
	if w.Code != http.StatusBadRequest || m["ok"] != false {
		t.Fatalf("unconfirmed: status=%d body=%v, want 400 ok:false", w.Code, m)
	}
	if msg, _ := m["error"].(string); !strings.Contains(msg, "not confirmed") {
		t.Fatalf("unconfirmed: want the not-confirmed sentinel, got %q", msg)
	}

	// Unknown session → 4xx; nothing starts.
	w, m = doJSON(t, h, http.MethodPost, "/api/foreign/restore",
		`{"session":"unknown","domain":"files","item":"docs","snapshot":"latest","confirm":true,"target":"restore-here/docs"}`)
	if w.Code != http.StatusBadRequest || m["ok"] != false {
		t.Fatalf("unknown session: status=%d body=%v, want 400 ok:false", w.Code, m)
	}

	// Busy → 409: hold the first restore inside the engine, then fire a second.
	eng.blockRestore = make(chan struct{})
	eng.restoreEntered = make(chan struct{}, 1)
	w, m = doJSON(t, h, http.MethodPost, "/api/foreign/restore",
		`{"session":"`+session+`","domain":"files","item":"docs","snapshot":"latest","confirm":true,"target":"restore-here/docs"}`)
	if w.Code != http.StatusOK || m["ok"] != true || m["started"] != true {
		t.Fatalf("restore: status=%d body=%v, want 200 {ok:true, started:true}", w.Code, m)
	}
	<-eng.restoreEntered // the detached restore holds the single-flight guard now
	w, m = doJSON(t, h, http.MethodPost, "/api/foreign/restore",
		`{"session":"`+session+`","domain":"files","item":"docs","snapshot":"latest","confirm":true,"target":"restore-here/docs"}`)
	if w.Code != http.StatusConflict || m["ok"] != false {
		t.Fatalf("busy: status=%d body=%v, want 409 ok:false", w.Code, m)
	}
	close(eng.blockRestore)
	waitForBackupDone(t, svc)

	// The detached work restored from the SESSION repo (never a settings repo).
	if len(eng.restored) != 1 || !strings.HasPrefix(eng.restored[0], sessionRepo+":eeeeeeee55555555:/->") {
		t.Fatalf("restored = %v, want one whole-tree restore from the session repo %q", eng.restored, sessionRepo)
	}
}

// TestForeignFilesAndSelectiveRestoreRoutes pins the #123 route wiring: POST
// /api/foreign/files lists the session snapshot's file tree so the Recovery card
// can offer a subfolder picker, and POST /api/foreign/restore with a non-empty
// "paths" selection restores ONLY those subfolders — from the SESSION repo, via a
// subtree-relative include (the contents land in the target, not nested under
// /host/user/…).
func TestForeignFilesAndSelectiveRestoreRoutes(t *testing.T) {
	enc := true
	location := "backups/other"
	eng := &fakeResticEngine{
		existingMode: &enc,
		snaps: []restic.Snapshot{
			{ID: "aaaaaaaa11111111", Time: "2026-07-05T10:00:00Z", Paths: []string{"/host/user/appdata"}, Tags: []string{"fileset:appdata"}},
		},
		lsEntries: []restic.FileEntry{
			{Path: "/host/user/appdata", Type: "dir"},
			{Path: "/host/user/appdata/vaultwarden", Type: "dir"},
			{Path: "/host/user/appdata/vaultwarden/db.sqlite3", Type: "file", Size: 4096},
		},
	}
	h, _, svc, dir := newTestRouterSvcDir(t, &fakeServiceDocker{}, eng)

	sessionRepo, err := paths.Resolve(dir, location)
	if err != nil {
		t.Fatalf("resolve session repo: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "backups", "other"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "backups", "other", "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	key := strings.Repeat("ab", 32)
	w, m := doJSON(t, h, http.MethodPost, "/api/foreign/open", `{"location":"`+location+`","key":"`+key+`"}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("open: status=%d body=%v", w.Code, m)
	}
	session, _ := m["session"].(string)
	if session == "" {
		t.Fatalf("open must return a session id, got %v", m)
	}

	// POST /api/foreign/files → 200 {ok:true, files:[...]} (the picker's tree).
	w, m = doJSON(t, h, http.MethodPost, "/api/foreign/files",
		`{"session":"`+session+`","domain":"files","item":"appdata","snapshot":"latest"}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("files: status=%d body=%v, want 200 ok:true", w.Code, m)
	}
	files, _ := m["files"].([]any)
	if len(files) != 3 {
		t.Fatalf("files: want 3 entries, got %v", m["files"])
	}

	// POST /api/foreign/restore with a "paths" selection → 200 {ok,started}, and
	// the detached work restores only that subfolder from the SESSION repo.
	w, m = doJSON(t, h, http.MethodPost, "/api/foreign/restore",
		`{"session":"`+session+`","domain":"files","item":"appdata","snapshot":"latest","confirm":true,"target":"restore-here/subset","paths":["/host/user/appdata/vaultwarden"]}`)
	if w.Code != http.StatusOK || m["ok"] != true || m["started"] != true {
		t.Fatalf("selective restore: status=%d body=%v, want 200 {ok:true, started:true}", w.Code, m)
	}
	waitForBackupDone(t, svc)

	wantTarget, err := paths.Resolve(dir, "restore-here/subset")
	if err != nil {
		t.Fatalf("resolve want target: %v", err)
	}
	want := sessionRepo + ":aaaaaaaa11111111:/host/user/appdata|/vaultwarden->" + wantTarget
	if len(eng.restored) != 1 || eng.restored[0] != want {
		t.Fatalf("restored = %v, want exactly [%s]", eng.restored, want)
	}
}
