package api

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/fortressmcp"
)

// fortressCall runs body through the action the other apps of the fortress
// call BombVault's tools with, as KnightLoader on Tower.
func fortressCall(t *testing.T, a fortress.Action, body string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := a.Run(ctx, fortress.Caller{ID: "kl1", App: "knightloader", Device: "Tower"}, []byte(body))
	if err != nil {
		t.Fatalf("%s: %v", body, err)
	}
	return out
}

func TestTheFortressSeesOnlyTheToolsThatStartAndCancelBackups(t *testing.T) {
	h, _, _ := newMCPStartHandler(t)
	a := h.fortressToolsAction()
	if a.Name != fortressmcp.ActAction {
		t.Fatalf("the action is %q, want %q", a.Name, fortressmcp.ActAction)
	}
	var list struct{ Tools []mcp.Tool }
	if err := json.Unmarshal(fortressCall(t, a, ""), &list); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range list.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	want := []string{"cancel_backup", "start_backup", "start_backup_everything", "start_domain_backup"}
	if !slices.Equal(names, want) {
		t.Fatalf("the fortress sees %v, want %v", names, want)
	}
}

func TestAnotherAppStartsABackupWithinBombVaultsOwnBudget(t *testing.T) {
	h, repo, _ := newMCPStartHandler(t, "docs", "media", "music")
	h.mcp.starts = newSlidingWindow(time.Hour, 2)
	a := h.fortressToolsAction()

	for _, name := range []string{"docs", "media"} {
		var res mcp.CallToolResult
		if err := json.Unmarshal(fortressCall(t, a, `{"tool":"start_backup","arguments":{"domain":"files","item":"`+name+`"}}`), &res); err != nil {
			t.Fatal(err)
		}
		if res.IsError {
			t.Fatalf("start of %s through the fortress: %v", name, res.StructuredContent)
		}
		waitForFilesIdle(t, h)
	}
	var res mcp.CallToolResult
	if err := json.Unmarshal(fortressCall(t, a, `{"tool":"start_backup","arguments":{"domain":"files","item":"music"}}`), &res); err != nil {
		t.Fatal(err)
	}
	if code := mcpErrorCode(t, &res); code != "rate_limited" {
		t.Fatalf("the third start through the fortress gives %q, want rate_limited", code)
	}

	runs, err := repo.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) == 0 {
		t.Fatal("no run was recorded")
	}
	for _, r := range runs {
		if r.StartedVia != viaFortress || r.StartedViaKey != "" {
			t.Fatalf("run %s was started via %q with key %q, want the fortress and no key", r.ID, r.StartedVia, r.StartedViaKey)
		}
	}
	if res := startFileSet(mcpStartCaller("c41d", true), h, "music"); res.IsError {
		t.Fatalf("a key was held to the fortress's budget: %v", res.StructuredContent)
	}
	waitForFilesIdle(t, h)
}

func TestOnlyAKeyThatMayStartBackupsCallsTheToolsOfOtherApps(t *testing.T) {
	h, _, _ := newMCPStartHandler(t)
	cfg := h.fortressMCP()
	if cfg.MayAct(mcpStartCaller("0b7e", false)) {
		t.Error("a key that may only read may call the tools of other apps")
	}
	if !cfg.MayAct(mcpStartCaller("0b7e", true)) {
		t.Error("a key that may start backups may not call the tools of other apps")
	}
	if cfg.Fortress() != nil {
		t.Error("a fortress shows up without a membership")
	}
}

func TestBombVaultOffersItsToolsAndUsesThoseOfOtherApps(t *testing.T) {
	svc, _ := newFortressSvc(t)
	offer := fortress.Action{Name: fortressmcp.ActAction, Run: func(context.Context, fortress.Caller, []byte) ([]byte, error) { return nil, nil }}
	cfg := svc.fortressConfig(fortress.Config{}, nil, offer)
	if !slices.ContainsFunc(cfg.Actions, func(a fortress.Action) bool { return a.Name == fortressmcp.ActAction }) {
		t.Errorf("actions %v lack %s", cfg.Actions, fortressmcp.ActAction)
	}
	if !slices.Contains(cfg.Uses, fortressmcp.ActAction) {
		t.Errorf("uses %v lack %s", cfg.Uses, fortressmcp.ActAction)
	}
}
