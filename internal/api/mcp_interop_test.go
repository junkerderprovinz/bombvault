package api_test

// Replaying the call sequences MCP clients put on the wire.
//
// The gate tests state what BombVault answers to a request the tests compose
// themselves; these fixtures state what a client sends, down to the header
// spelling and the order of the calls. They are the check that survives an SDK
// bump: a compatibility default that changes under us shows up here as a status
// or an envelope that no longer matches.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mcpTranscriptKeyPlaceholder stands in for the key in every fixture, so a
// recording can be committed without the secret it was made with.
const mcpTranscriptKeyPlaceholder = "<key>"

// mcpTranscriptStep is one exchange: the request a client sent and the part of
// the answer a replay can compare, which is the status and whether the
// JSON-RPC envelope carried a result or an error.
type mcpTranscriptStep struct {
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
	Expect  struct {
		Status int  `json:"status"`
		Result bool `json:"result"`
		Error  bool `json:"error"`
	} `json:"expect"`
}

func TestMCPRecordedClientTranscripts(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "mcp", "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no recorded transcripts under testdata/mcp, so this test proves nothing")
	}

	for _, file := range files {
		t.Run(strings.TrimSuffix(filepath.Base(file), ".jsonl"), func(t *testing.T) {
			raw, err := os.ReadFile(file) //nolint:gosec // G304: a test fixture path from this package's own testdata
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "bvmcp_") {
				t.Fatalf("%s holds a real key; recordings carry %s instead", file, mcpTranscriptKeyPlaceholder)
			}

			h, _, _, key := newMCPToolRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
			for i, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				var step mcpTranscriptStep
				if err := json.Unmarshal([]byte(line), &step); err != nil {
					t.Fatalf("step %d: %v", i+1, err)
				}
				replayMCPStep(t, h, i+1, step, key)
			}
		})
	}
}

func replayMCPStep(t *testing.T, h http.Handler, n int, step mcpTranscriptStep, key string) {
	t.Helper()
	r := httptest.NewRequest(step.Method, "/mcp", strings.NewReader(string(step.Body)))
	for name, value := range step.Headers {
		r.Header.Set(name, strings.ReplaceAll(value, mcpTranscriptKeyPlaceholder, key))
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != step.Expect.Status {
		t.Fatalf("step %d: status = %d, want %d, body = %q", n, w.Code, step.Expect.Status, w.Body.String())
	}
	if w.Body.Len() == 0 {
		if step.Expect.Result || step.Expect.Error {
			t.Fatalf("step %d: empty body, want a JSON-RPC envelope", n)
		}
		return
	}
	var env struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("step %d: decode %q: %v", n, w.Body.String(), err)
	}
	if got := len(env.Result) > 0; got != step.Expect.Result {
		t.Errorf("step %d: result present = %t, want %t: %s", n, got, step.Expect.Result, w.Body.String())
	}
	if got := len(env.Error) > 0; got != step.Expect.Error {
		t.Errorf("step %d: error present = %t, want %t: %s", n, got, step.Expect.Error, w.Body.String())
	}
}
