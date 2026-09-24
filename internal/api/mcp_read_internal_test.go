package api

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// newMCPRestorePointHandler wires a handler whose flash repository exists and
// whose every listing blocks in eng.
func newMCPRestorePointHandler(t *testing.T) (*Handler, *blockingSnapshotsEngine) {
	t.Helper()
	h, _, repo, _ := newMCPGateHandler(t)
	eng := newBlockingSnapshotsEngine()
	h.svc = NewService(h.cfg, repo, nil, nil, eng)

	settings, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.FlashEnabled = true
	settings.FlashPath = "backups/flash"
	if err := repo.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(h.cfg.HostMountRoot, "backups", "flash")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return h, eng
}

// listFlashRestorePoints calls the tool the way the transport would, on the
// context the transport would hand it.
func listFlashRestorePoints(ctx context.Context, h *Handler) *mcp.CallToolResult {
	ctx = withMCPCaller(ctx, mcpCaller{KeyID: "0b7e", Hint: "x9Qa", CanStartBackups: true})
	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name:      "list_restore_points",
		Arguments: json.RawMessage(`{"domain":"flash"}`),
	}}
	res, _ := h.toolListRestorePoints(ctx, req)
	return res
}

// A legacy-era handler context is detached from its client, so restic would
// keep reading a remote repository long after the assistant gave up.
func TestMCPRestorePointsTimeoutCancelsRestic(t *testing.T) {
	h, eng := newMCPRestorePointHandler(t)
	mcpResticTimeout = 50 * time.Millisecond
	t.Cleanup(func() { mcpResticTimeout = time.Minute })

	done := make(chan *mcp.CallToolResult, 1)
	go func() { done <- listFlashRestorePoints(context.Background(), h) }()

	select {
	case res := <-done:
		if got := mcpErrorCode(t, res); got != "timeout" {
			t.Fatalf("code = %q, want timeout", got)
		}
	case <-time.After(time.Second):
		t.Fatal("the listing is still running a second after its deadline")
	}
	if !eng.sawCancellation() {
		t.Fatal("restic was left running after the tool gave up")
	}
}

func TestMCPRestorePointsSemaphoreBusy(t *testing.T) {
	h, eng := newMCPRestorePointHandler(t)

	first := make(chan *mcp.CallToolResult, 1)
	go func() { first <- listFlashRestorePoints(context.Background(), h) }()
	select {
	case <-eng.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the first listing never reached the engine")
	}

	busy := listFlashRestorePoints(context.Background(), h)
	if got := mcpErrorCode(t, busy); got != "busy" {
		t.Fatalf("a second listing gives %q, want busy", got)
	}

	close(eng.release)
	select {
	case res := <-first:
		if res.IsError {
			t.Fatalf("the released listing failed: %v", res.StructuredContent)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the released listing never finished")
	}

	if third := listFlashRestorePoints(context.Background(), h); third.IsError {
		t.Fatalf("the slot was never given back: %v", third.StructuredContent)
	}
	if got := eng.callCount(); got != 2 {
		t.Fatalf("restic ran %d times, want the two listings that held the slot", got)
	}
}

// A client that hangs up mid-call cancels the tool context. Reporting that as a
// slow repository sends an operator after a fault that is not there and drops
// what restic really said on the way.
func TestMCPRestorePointsCancelledClientIsNotATimeout(t *testing.T) {
	h, eng := newMCPRestorePointHandler(t)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan *mcp.CallToolResult, 1)
	go func() { done <- listFlashRestorePoints(ctx, h) }()
	select {
	case <-eng.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the listing never reached the engine")
	}
	cancel()

	select {
	case res := <-done:
		if got := mcpErrorCode(t, res); got != "failed" {
			t.Fatalf("code = %q, want failed", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the cancelled listing never came back")
	}
}
