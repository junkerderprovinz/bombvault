package virshcli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func recordingVirsh(t *testing.T) (*Client, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs a shell script as the virsh binary")
	}
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	bin := filepath.Join(dir, "virsh")
	script := "#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> '" + calls + "'; done\necho '@@end' >> '" + calls + "'\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { //nolint:gosec // G306: the test needs it executable
		t.Fatal(err)
	}
	return &Client{bin: bin}, calls
}

func recordedCalls(t *testing.T, file string) []string {
	t.Helper()
	b, err := os.ReadFile(file) //nolint:gosec // G304: a file of the test
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	for _, c := range strings.Split(strings.TrimSuffix(string(b), "@@end\n"), "\n@@end\n") {
		calls = append(calls, strings.Join(strings.Fields(c), " "))
	}
	return calls
}

func TestDomainNamesAreNeverReadAsOptions(t *testing.T) {
	c, file := recordingVirsh(t)
	ctx := context.Background()
	const dom = "--undefine"
	_, _ = c.State(ctx, dom)
	_, _ = c.DumpXML(ctx, dom)
	_, _ = c.DumpXMLInactive(ctx, dom)
	_ = c.Shutdown(ctx, dom)
	_ = c.Destroy(ctx, dom)
	_ = c.Start(ctx, dom)
	_ = c.Undefine(ctx, dom)
	_ = c.Autostart(ctx, dom, false)
	_ = c.BlockCommitActivePivot(ctx, dom, "vda")
	_ = c.GuestAgentPing(ctx, dom)
	calls := recordedCalls(t, file)
	if len(calls) != 10 {
		t.Fatalf("%d calls recorded: %q", len(calls), calls)
	}
	for _, call := range calls {
		if !strings.Contains(" "+call+" ", " -- "+dom+" ") {
			t.Errorf("%q passes the domain where virsh could read it as an option", call)
		}
	}
}
