package api_test

import (
	"net/http"
	"strings"
	"testing"

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
