package api

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// A key's log on the settings card names every outcome in a sentence of its
// own, and a code the card does not know shows up raw. So every code the tools
// and the gate can log has to be in McpKeyLog.tsx.
func TestEveryLoggedMCPOutcomeHasASentenceOnTheCard(t *testing.T) {
	logged := regexp.MustCompile(`(?:logMCP(?:Run)?Call\(ctx, [^,]+, |recordMCPRefusal\([^,]+, |mcpToolError\(|outcome :?= )"([a-z_]+)"`)
	files, err := filepath.Glob("mcp*.go")
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, rErr := os.ReadFile(f) //nolint:gosec // G304: a file of this package found by the glob above
		if rErr != nil {
			t.Fatal(rErr)
		}
		for _, m := range logged.FindAllStringSubmatch(string(src), -1) {
			codes[m[1]] = true
		}
	}
	if len(codes) < 10 {
		t.Fatalf("found only %d outcome codes, the pattern no longer matches the tools", len(codes))
	}

	path := filepath.Join("..", "..", "web", "src", "pages", "settings", "McpKeyLog.tsx")
	card, err := os.ReadFile(path) //nolint:gosec // G304: fixed repo-relative path
	if err != nil {
		t.Fatal(err)
	}
	table := regexp.MustCompile(`(?s)const OUTCOME\b[^{]*\{(.*?)\n\};`).FindStringSubmatch(string(card))
	if table == nil {
		t.Fatal("McpKeyLog.tsx has no OUTCOME table")
	}
	var missing []string
	for code := range codes {
		if !strings.Contains(table[1], "\n  "+code+": ") && !strings.Contains(string(card), `e.outcome === "`+code+`"`) {
			missing = append(missing, code)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("McpKeyLog.tsx shows these outcomes as raw codes: %v", missing)
	}
}

// A client looping on a refusal would otherwise write a row per request, so a
// refusal reaches the key's log once per minute and reason.
func TestMCPGateRefusalsReachTheKeysLogOncePerMinute(t *testing.T) {
	h, router, repo, _ := newMCPGateHandler(t)
	key, id := seedMCPKey(t, h, repo, "Laptop")
	h.mcp.calls = newSlidingWindow(time.Hour, 2)
	base := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	h.mcp.now = func() time.Time { return base }

	if w := mcpInternalPost(router, key, mcpInternalInitialize, ""); w.Code != http.StatusOK {
		t.Fatalf("first request: status = %d", w.Code)
	}
	if w := mcpInternalPost(router, key, `[`+mcpInternalInitialize+`]`, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("batch: status = %d, want 400", w.Code)
	}
	for range 3 {
		if w := mcpInternalPost(router, key, mcpInternalInitialize, ""); w.Code != http.StatusTooManyRequests {
			t.Fatalf("over the budget: status = %d, want 429", w.Code)
		}
	}
	h.mcp.now = func() time.Time { return base.Add(61 * time.Second) }
	mcpInternalPost(router, key, mcpInternalInitialize, "")

	events, err := repo.MCPKeyEvents(id)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range events {
		if e.Tool != "" {
			t.Fatalf("a gate refusal names a tool: %+v", e)
		}
		got = append(got, e.Outcome)
	}
	want := []string{"rate_limited", "rate_limited", "batch_refused"}
	if len(got) != len(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("events = %v, want %v", got, want)
		}
	}
	calls, err := repo.MCPKeyCallsSince(0)
	if err != nil {
		t.Fatal(err)
	}
	if calls[id] != 0 {
		t.Fatalf("refused requests counted as %d calls", calls[id])
	}
}
