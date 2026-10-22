package restic

import (
	"fmt"
	"strings"
	"testing"
)

func TestTheSkippedFilesAreNamedUpToALimitAndCounted(t *testing.T) {
	var lines []string
	for i := range maxUnreadableItems + 2 {
		lines = append(lines, fmt.Sprintf(`{"message_type":"error","error":{"message":"open /data/f%d: permission denied"},"during":"archival","item":"/data/f%d"}`, i, i))
	}
	lines = append(lines, lines[0], "Warning: at least one source file could not be read")

	got := unreadableItems(strings.Join(lines, "\n"))
	if !strings.Contains(got, "/data/f0") || !strings.Contains(got, fmt.Sprintf("/data/f%d", maxUnreadableItems-1)) {
		t.Fatalf("reason = %q, want the first files named", got)
	}
	if strings.Contains(got, fmt.Sprintf("/data/f%d", maxUnreadableItems)) || !strings.HasSuffix(got, "(+2 more)") {
		t.Fatalf("reason = %q, want the rest counted", got)
	}
	if unreadableItems("Warning: at least one source file could not be read") != "" {
		t.Fatal("a stderr without error objects named files")
	}
}
