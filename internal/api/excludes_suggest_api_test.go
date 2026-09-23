package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The suggest endpoint walks the container's backed-up folder, suggests a
// well-known junk dir by name and skips a dir the stored excludes already
// cover. The returned line can be stored through the excludes PATCH as is.
func TestExcludesSuggest(t *testing.T) {
	d := &fakeServiceDocker{}
	h, _, _, dir := newTestRouterSvcDir(t, d, &fakeResticEngine{})

	// appdata/plex is the auto-detected backup root.
	appdata := filepath.Join(dir, "appdata", "plex")
	if err := os.MkdirAll(filepath.Join(appdata, "Cache"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appdata, "Cache", "chunk.bin"), []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(appdata, "logs"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appdata, "logs", "app.log"), []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}

	// With "logs" excluded, the scan must not suggest it.
	w, m := doJSON(t, h, http.MethodPatch, "/api/containers/plex", `{"excludes":["logs"]}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("patch status=%d body=%s", w.Code, w.Body.String())
	}

	w, m = doJSON(t, h, http.MethodGet, "/api/containers/plex/excludes/suggest", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("suggest status=%d body=%s", w.Code, w.Body.String())
	}
	if m["truncated"] != false {
		t.Fatalf("truncated = %v, want false", m["truncated"])
	}
	suggestions, ok := m["suggestions"].([]any)
	if !ok || len(suggestions) != 1 {
		t.Fatalf("expected exactly 1 suggestion (Cache; logs excluded), got %v", m["suggestions"])
	}
	sg, ok := suggestions[0].(map[string]any)
	if !ok {
		t.Fatalf("suggestion shape: %v", suggestions[0])
	}
	if sg["path"] != "Cache" || sg["reason"] != "known-cache" || sg["sizeBytes"] != float64(10) {
		t.Fatalf("suggestion = %v, want Cache/known-cache/10", sg)
	}
	// No mount covers the harness dir, so the line is the scanned path itself.
	wantLine := filepath.ToSlash(filepath.Join(appdata, "Cache"))
	line, _ := sg["line"].(string)
	if line != wantLine {
		t.Fatalf("line = %q, want %q", line, wantLine)
	}
	if strings.TrimSpace(line) == "" {
		t.Fatal("line must be storable")
	}
}
