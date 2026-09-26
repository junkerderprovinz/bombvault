package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The key's log keeps reads that went through under a cap of their own, so an
// assistant polling get_activity cannot push out the start it is polling for.
func TestAKeysPollsDoNotPushItsStartOutOfTheLog(t *testing.T) {
	h, _, repo, _ := newMCPGateHandler(t)
	_, id := seedMCPKey(t, h, repo, "Laptop")
	ctx := mcpStartCaller(id, true)

	h.logMCPCall(ctx, "start_backup", "ok")
	h.logMCPCall(ctx, "list_runs", "invalid_argument")
	for range store.MCPKeyEventsKept {
		h.logMCPCall(ctx, "get_activity", "ok")
	}

	events, err := repo.MCPKeyEvents(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != store.MCPKeyReadsKept+2 {
		t.Fatalf("%d events kept, want %d reads and the two others", len(events), store.MCPKeyReadsKept)
	}
	if last := events[len(events)-1]; last.Tool != "start_backup" {
		t.Fatalf("the oldest event kept is %+v, want the start", last)
	}
}

// Every tool that is not a read acts on the server, and its calls are what the
// log is for.
func TestOnlyAReadThatWentThroughIsRoutine(t *testing.T) {
	h, _, _, _ := newMCPGateHandler(t)
	for _, def := range h.mcpToolDefs() {
		read := def.tool.Annotations.ReadOnlyHint
		if got := mcpRoutineCall(def.tool.Name, "ok"); got != read {
			t.Errorf("%s ok: routine = %v, want %v", def.tool.Name, got, read)
		}
		if mcpRoutineCall(def.tool.Name, "invalid_argument") {
			t.Errorf("%s: a refused call counts as routine", def.tool.Name)
		}
	}
}

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

// The key's log records the code the assistant received, so a read that ran out
// of time or a start that failed does not read as something else on the card.
func TestAKeysLogRecordsTheCodeTheAssistantGot(t *testing.T) {
	h, _, repo, _ := newMCPGateHandler(t)
	_, id := seedMCPKey(t, h, repo, "Laptop")
	ctx := mcpStartCaller(id, true)
	release := func() {}

	results := []*mcp.CallToolResult{
		h.mcpFailure(ctx, "get_coverage", fmt.Errorf("coverage: %w", context.DeadlineExceeded)),
		h.mcpFailure(ctx, "get_status", errors.New("disk I/O error")),
		h.mcpStartOutcome(ctx, "start_backup", release, false, errors.New("disk I/O error"), "a backup is already running", nil),
		h.mcpStartOutcome(ctx, "start_backup", release, false, domainBusyError{op: "prune", domain: "files"}, "a backup is already running", nil),
		h.mcpStartOutcome(ctx, "start_backup", release, false, nil, "a backup is already running", nil),
		h.mcpStartOutcome(ctx, "start_backup_everything", release, false, errMCPBackupRunning, "a Backup Everything pass is already running", nil),
	}
	want := []string{"timeout", "failed", "failed", "busy", "busy", "busy"}

	events, err := repo.MCPKeyEvents(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != len(want) {
		t.Fatalf("%d events, want %d", len(events), len(want))
	}
	for i, res := range results {
		logged := events[len(events)-1-i].Outcome
		if sent := mcpErrorCodeOf(res); sent != want[i] || logged != want[i] {
			t.Errorf("call %d: sent %q, logged %q, want %q for both", i, sent, logged, want[i])
		}
	}
}
