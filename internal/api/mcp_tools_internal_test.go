package api

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Only a request that skipped the gate can reach a handler without a caller, so
// every handler refuses instead of answering with the instance's state.
func TestMCPToolRefusesWithoutCaller(t *testing.T) {
	h, _, _, _ := newMCPGateHandler(t)

	for _, def := range h.mcpToolDefs() {
		res, err := def.run(context.Background(), nil)
		if err != nil {
			t.Fatalf("%s returned a protocol error: %v", def.tool.Name, err)
		}
		if !res.IsError {
			t.Fatalf("%s answered a request that carries no key", def.tool.Name)
		}
		if got := mcpErrorCode(t, res); got != "not_permitted" {
			t.Fatalf("%s code = %q, want not_permitted", def.tool.Name, got)
		}
	}
}

// The web interface may show the operator the path a message is about; a
// language model provider may not. scrubError keeps the sentinels below
// verbatim for exactly that reason, so the MCP layer scrubs on top of it.
func TestMCPToolFailuresAreScrubbedToolErrors(t *testing.T) {
	const (
		hostPath = "/host/user/appdata/plex"
		repoURL  = "rest:https://u:p@nas:8000/containers"
	)

	for _, tc := range []struct {
		name string
		err  error
		keep string
	}{
		{"repository location refused", fmt.Errorf("%w: use %s", errRepoPathGuidance, repoURL), errRepoPathGuidance.Error()},
		{"restore destination refused", fmt.Errorf("%w: %s already holds data", errRestoreDestination, hostPath), errRestoreDestination.Error()},
		{"zvol rebase failed", fmt.Errorf("%w: cache/vm/win11 under %s", errZvolRebaseFailed, hostPath), errZvolRebaseFailed.Error()},
		{"unraid platform mismatch", fmt.Errorf("%w: %s is not mounted", errUnraidPlatformMismatch, hostPath), errUnraidPlatformMismatch.Error()},
		{"rest path user", fmt.Errorf("%w: %s names backup, the credential names restic", errRestPathUser, hostPath), errRestPathUser.Error()},
		{"stored run error", fmt.Errorf("restic backup failed on %s: %s", repoURL, hostPath), "restic backup failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := mcpServiceError(tc.err)
			if !res.IsError {
				t.Fatal("a service failure must answer a tool error, not a plain result")
			}
			if got := mcpErrorCode(t, res); got != "failed" {
				t.Fatalf("code = %q, want failed", got)
			}
			msg := mcpErrorMessage(t, res)
			if !strings.Contains(msg, tc.keep) {
				t.Fatalf("the message lost the sentinel's wording %q: %q", tc.keep, msg)
			}
			for _, leak := range []string{hostPath, "u:p@", "rest:https://nas:8000"} {
				if strings.Contains(msg, leak) {
					t.Fatalf("the message leaks %q: %q", leak, msg)
				}
			}
		})
	}

	res := mcpServiceError(context.DeadlineExceeded)
	if got := mcpErrorCode(t, res); got != "timeout" {
		t.Fatalf("a deadline gives code %q, want timeout", got)
	}
}

// A domain BombVault knows but the MCP vocabulary does not would be invisible
// to every tool, and the tools would still answer as if they had seen
// everything. A domain the service learns later fails here first.
func TestMCPDomainsCoverEveryServiceDomain(t *testing.T) {
	h, _, repo, _ := newMCPGateHandler(t)
	h.svc = NewService(h.cfg, repo, &foreignFakeDocker{}, nil, nil)

	settings, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersEnabled = true
	settings.VMsEnabled = true
	settings.FilesEnabled = true
	settings.FlashEnabled = true
	settings.ConfigEnabled = true
	if err := repo.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	status, err := h.svc.domainStatusFrom(settings)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range status {
		if !slices.Contains(mcpDomains, d.Domain) {
			t.Fatalf("domainStatusFrom reports %q, which mcpDomains does not carry", d.Domain)
		}
	}

	coverage, err := h.svc.Coverage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range coverage.Domains {
		if !slices.Contains(mcpDomains, d.Domain) {
			t.Fatalf("Coverage reports %q, which mcpDomains does not carry", d.Domain)
		}
	}

	for domain := range h.svc.repoMu {
		if !slices.Contains(mcpDomains, domain) {
			t.Fatalf("the service locks a repository for %q, which mcpDomains does not carry", domain)
		}
	}

	for _, def := range h.mcpToolDefs() {
		schema, ok := def.tool.InputSchema.(map[string]any)
		if !ok {
			t.Fatalf("%s has no input schema object", def.tool.Name)
		}
		props, _ := schema["properties"].(map[string]any)
		prop, ok := props["domain"].(map[string]any)
		if !ok {
			continue
		}
		values, _ := prop["enum"].([]string)
		for _, domain := range mcpDomains {
			if !slices.Contains(values, domain) {
				t.Fatalf("%s does not accept the domain %q", def.tool.Name, domain)
			}
		}
	}
}

// An item that is not there is counted and logged under the code the caller
// got, so an alert on not_found sees a client probing names.
func TestMCPItemLookupRecordsTheCodeItAnswers(t *testing.T) {
	h, _, _ := newMCPStartHandler(t)
	ctx := mcpStartCaller("0b7e", true)
	args := json.RawMessage(`{"domain":"files","item":"ghost"}`)

	for _, c := range []struct {
		tool string
		run  func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error)
	}{
		{"start_backup", h.toolStartBackup},
		{"list_runs", h.toolListRuns},
		{"list_restore_points", h.toolListRestorePoints},
	} {
		res, _ := c.run(ctx, &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: c.tool, Arguments: args}})
		if code := mcpErrorCode(t, res); code != "not_found" {
			t.Fatalf("%s answers %q, want not_found", c.tool, code)
		}
		h.mcp.countMu.Lock()
		got := h.mcp.toolCalls[mcpToolOutcome{tool: c.tool, outcome: "not_found"}]
		h.mcp.countMu.Unlock()
		if got != 1 {
			t.Fatalf("%s counted %d calls under not_found, want 1", c.tool, got)
		}
	}
}

func mcpErrorCode(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	return mcpErrorField(t, res, "code")
}

func mcpErrorMessage(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	return mcpErrorField(t, res, "message")
}

func mcpErrorField(t *testing.T, res *mcp.CallToolResult, field string) string {
	t.Helper()
	s, _ := mcpErrorBody(t, res)[field].(string)
	return s
}

// mcpErrorBody is a refusal's error object, which carries the code and the
// message next to whatever numbers the refusal explains itself with.
func mcpErrorBody(t *testing.T, res *mcp.CallToolResult) map[string]any {
	t.Helper()
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return body.Error
}
