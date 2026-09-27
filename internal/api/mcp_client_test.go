package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// keyTransport puts the MCP key on every request the SDK client sends, which is
// what a client configured with a header does.
type keyTransport struct {
	key string
}

func (t keyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+t.key)
	return http.DefaultTransport.RoundTrip(r)
}

// The gate, the transport and the tool registry only have to agree with each
// other in the tests above; here they have to agree with the library every
// client in the field is built on.
func TestMCPModernEraWithOfficialClient(t *testing.T) {
	router, _, _, key := newMCPToolRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "bombvault-tests", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             server.URL + "/mcp",
		HTTPClient:           &http.Client{Transport: keyTransport{key: key}},
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close() //nolint:errcheck // the test server goes away with the test

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if want := len(mcpReadTools) + len(mcpStartTools); len(tools.Tools) != want {
		t.Fatalf("the client sees %d tools, want %d", len(tools.Tools), want)
	}
	if tools.CacheScope != "private" {
		t.Fatalf("cacheScope = %q, want private: a tool list is one operator's own instance", tools.CacheScope)
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_health"})
	if err != nil {
		t.Fatalf("tools/call get_health: %v", err)
	}
	if res.IsError {
		t.Fatalf("get_health: %v", res.Content)
	}
	health, ok := res.StructuredContent.(map[string]any)
	if !ok || health["ok"] != true {
		t.Fatalf("structured content = %v", res.StructuredContent)
	}
}
