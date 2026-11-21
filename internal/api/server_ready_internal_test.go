package api

import (
	"strings"
	"testing"
)

func TestTheReadyLinePointsAtTheWebInterfaceOnlyWhereThereIsOne(t *testing.T) {
	if line := readyLine("HTTPS", 3443, false); !strings.Contains(line, "Open the WebUI now (HTTPS 3443)") {
		t.Errorf("full install: %q", line)
	}
	line := readyLine("HTTPS", 3443, true)
	if strings.Contains(line, "WebUI") || !strings.Contains(line, "node") {
		t.Errorf("node: %q, want it to say it runs as a node without pointing at a web interface", line)
	}
}
