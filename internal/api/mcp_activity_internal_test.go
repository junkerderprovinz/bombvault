package api

import (
	"net/http"
	"testing"
	"time"
)

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
