package api_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/secret"
)

var mentionsMCP = regexp.MustCompile(`\bMCP\b`)

// Switched off, neither the support bundle nor the recovery kit mentions the
// MCP server, even when the table still holds a key from an earlier build.
func TestSwitchedOffMCPServerStaysOutOfBundleAndKit(t *testing.T) {
	api.ShipMCP(t, false)

	h, st, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	key, err := secret.NewMCPKey()
	if err != nil {
		t.Fatal(err)
	}
	appKey := strings.Repeat("a", 64)
	if _, err := st.CreateMCPKey("k1", "Laptop", "", secret.HashMCPKey(appKey, key),
		secret.MCPKeyHint(key), secret.MCPKeyCheck(appKey, "k1"), true, 1789600000); err != nil {
		t.Fatal(err)
	}
	cookie := loginCookie(t, h, "correct horse battery staple")

	w := getRaw(t, h, "/api/diagnostics", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("diagnostics: status = %d body = %s", w.Code, w.Body.String())
	}
	if manifest := zipMembers(t, w.Body.Bytes())["manifest.json"]; strings.Contains(strings.ToLower(manifest), "mcp") {
		t.Errorf("the support bundle's manifest mentions MCP:\n%s", manifest)
	}

	w = getRaw(t, h, "/api/recovery-kit", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("recovery kit: status = %d body = %s", w.Code, w.Body.String())
	}
	// Word-bounded: the kit prints the test's temp directory, which carries
	// the test name.
	if mentionsMCP.MatchString(w.Body.String()) {
		t.Errorf("the recovery kit mentions MCP")
	}
}
