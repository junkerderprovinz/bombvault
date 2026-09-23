package api_test

import (
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

// mcpReadTools are the tools a key may call without any permission beyond
// holding a key at all.
var mcpReadTools = []string{
	"get_health",
	"get_status",
	"get_coverage",
	"get_activity",
	"get_storage_stats",
	"list_items",
	"list_runs",
	"list_restore_points",
}

// mcpStartTools are the tools a key needs the start permission for. They are in
// every key's tool list all the same.
var mcpStartTools = []string{
	"start_backup",
	"start_domain_backup",
	"start_backup_everything",
	"cancel_backup",
}

// The era Claude Code and mcp-remote negotiate today. A client that speaks it
// must reach the tool list without sending anything newer.
func TestMCPLegacyEraHandshake(t *testing.T) {
	h, _, _, key := newMCPToolRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})

	w := (mcpReq{key: key, body: mcpInitializeBody}).do(t, h)
	if w.Code != http.StatusOK {
		t.Fatalf("initialize: status = %d body = %q", w.Code, w.Body.String())
	}
	var env struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
			Capabilities    struct {
				Tools *struct {
					ListChanged bool `json:"listChanged"`
				} `json:"tools"`
			} `json:"capabilities"`
			Instructions string `json:"instructions"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode initialize response %q: %v", w.Body.String(), err)
	}
	if env.Result.ProtocolVersion != "2025-11-25" {
		t.Fatalf("protocolVersion = %q, want 2025-11-25", env.Result.ProtocolVersion)
	}
	if env.Result.Capabilities.Tools == nil {
		t.Fatal("the server does not advertise tools")
	}
	for _, want := range []string{"web interface", "data"} {
		if !strings.Contains(env.Result.Instructions, want) {
			t.Fatalf("instructions do not mention %q:\n%s", want, env.Result.Instructions)
		}
	}

	w = (mcpReq{
		key:     key,
		body:    `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		headers: [][2]string{{"MCP-Protocol-Version", "2025-11-25"}},
	}).do(t, h)
	if w.Code != http.StatusOK {
		t.Fatalf("tools/list in the legacy era: status = %d body = %q", w.Code, w.Body.String())
	}
}

func TestMCPToolAllowlist(t *testing.T) {
	h, _, _, key := newMCPToolRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})

	want := append(append([]string(nil), mcpReadTools...), mcpStartTools...)
	sort.Strings(want)
	if got := mcpToolNames(t, mcpListTools(t, h, key)); !reflect.DeepEqual(got, want) {
		t.Fatalf("tools = %v, want %v", got, want)
	}

	readOnly, _ := createMCPKey(t, h, "Desktop", false)
	if got := mcpToolNames(t, mcpListTools(t, h, readOnly)); !reflect.DeepEqual(got, want) {
		t.Fatalf("a read-only key sees %v, want the same list %v", got, want)
	}
}

func TestMCPToolAnnotationsAndSchemas(t *testing.T) {
	if v := os.Getenv("MCPGODEBUG"); v != "" {
		t.Fatalf("MCPGODEBUG is set to %q, which changes how the hints are marshalled; unset it and run again", v)
	}
	h, _, _, key := newMCPToolRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})

	for _, tool := range mcpListTools(t, h, key) {
		name, _ := tool["name"].(string)
		if title, _ := tool["title"].(string); title == "" {
			t.Fatalf("%s has no title", name)
		}
		if desc, _ := tool["description"].(string); desc == "" {
			t.Fatalf("%s has no description", name)
		}
		if _, ok := tool["outputSchema"]; ok {
			t.Fatalf("%s carries an output schema, which the SDK would then validate every result against", name)
		}

		schema, ok := tool["inputSchema"].(map[string]any)
		if !ok {
			t.Fatalf("%s has no input schema object", name)
		}
		if schema["type"] != "object" {
			t.Fatalf("%s input schema type = %v, want object", name, schema["type"])
		}
		if schema["additionalProperties"] != false {
			t.Fatalf("%s input schema allows additional properties", name)
		}

		ann, ok := tool["annotations"].(map[string]any)
		if !ok {
			t.Fatalf("%s carries no annotations", name)
		}
		// Listing restore points is the one read that leaves the machine: a
		// primary repository can be an S3 bucket or a REST server. A start
		// leaves it too, and a second call makes a second backup, but nothing
		// it does destroys a restore point.
		// Cancelling writes too, and a second call finds nothing left to stop,
		// which is why it keeps the idempotent hint the starts give up.
		starts := slices.Contains(mcpStartTools, name)
		for hint, want := range map[string]any{
			"readOnlyHint":    !starts,
			"destructiveHint": false,
			"idempotentHint":  !starts || name == "cancel_backup",
			"openWorldHint":   starts || name == "list_restore_points",
		} {
			if ann[hint] != want {
				t.Fatalf("%s %s = %v, want %v", name, hint, ann[hint], want)
			}
		}
	}
}

func TestMCPUnknownArgumentIsToolError(t *testing.T) {
	h, _, _, key := newMCPToolRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})

	res := mcpCallTool(t, h, key, "get_storage_stats", `{"domain":"containers","force":true}`)
	if code := res.code(t); code != "invalid_argument" {
		t.Fatalf("code = %q, want invalid_argument (result %v)", code, res.Structured)
	}
	if msg := res.message(t); !strings.Contains(msg, "force") {
		t.Fatalf("message %q does not name the rejected field", msg)
	}
	if text := res.text(t); !strings.HasPrefix(text, "invalid_argument: ") {
		t.Fatalf("text = %q, want it to start with the code", text)
	}
}

// A fresh install has nothing in it, and every list a tool returns then has to
// be empty rather than absent: a null reads as "unknown" where the truth is
// "none", and an assistant passes that difference on to the operator.
func TestMCPReadToolsOnEmptyInstall(t *testing.T) {
	h, st, _, key := newMCPToolRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	settings := mustSettings(t, st)
	settings.FlashPath = "backups/flash"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	// The argument object a tool needs to answer at all; the tools that take no
	// input are absent from it.
	args := map[string]string{
		"get_storage_stats":   `{"domain":"containers"}`,
		"list_restore_points": `{"domain":"flash"}`,
	}
	arrays := map[string][]string{
		"get_status":          {"domains", "nextRuns"},
		"get_coverage":        {"domains"},
		"get_activity":        {"running"},
		"get_storage_stats":   {"samples"},
		"list_items":          {"domains"},
		"list_runs":           {"runs"},
		"list_restore_points": {"restorePoints"},
	}
	for _, tool := range mcpReadTools {
		res := mcpCallTool(t, h, key, tool, args[tool])
		if res.IsError {
			t.Fatalf("%s on a fresh install: %v", tool, res.Structured)
		}
		if res.Structured == nil {
			t.Fatalf("%s returned no structured content", tool)
		}
		var text map[string]any
		if err := json.Unmarshal([]byte(res.text(t)), &text); err != nil {
			t.Fatalf("%s text block is not the same JSON object: %v", tool, err)
		}
		for _, field := range arrays[tool] {
			if _, ok := res.Structured[field].([]any); !ok {
				t.Fatalf("%s: %s = %v, want an array", tool, field, res.Structured[field])
			}
		}
		if tool == "get_activity" {
			if _, ok := res.Structured["domainsBusy"].(map[string]any); !ok {
				t.Fatalf("domainsBusy = %v, want an object", res.Structured["domainsBusy"])
			}
		}
	}
}

func TestMCPToolSeesCallerFromGate(t *testing.T) {
	h, _, _, key := newMCPToolRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})

	res := mcpCallTool(t, h, key, "get_health", "")
	if res.IsError {
		t.Fatalf("get_health: %v", res.Structured)
	}
	caller, ok := res.Structured["key"].(map[string]any)
	if !ok {
		t.Fatalf("get_health carries no key block: %v", res.Structured)
	}
	if caller["label"] != "Laptop" {
		t.Fatalf("label = %v, want the key's own label", caller["label"])
	}
	if caller["canStartBackups"] != true {
		t.Fatalf("canStartBackups = %v, want true", caller["canStartBackups"])
	}
	if _, isNumber := caller["startsLeftThisHour"].(float64); !isNumber {
		t.Fatalf("startsLeftThisHour = %v, want a number", caller["startsLeftThisHour"])
	}

	readOnly, _ := createMCPKey(t, h, "Desktop", false)
	caller, _ = mcpCallTool(t, h, readOnly, "get_health", "").Structured["key"].(map[string]any)
	if caller["label"] != "Desktop" || caller["canStartBackups"] != false {
		t.Fatalf("a read-only key reports %v", caller)
	}
}
