package restic

import (
	"strings"
	"testing"
)

func TestStatusPercent(t *testing.T) {
	cases := []struct {
		name   string
		line   string
		want   float64
		wantOK bool
	}{
		{"status mid", `{"message_type":"status","percent_done":0.25}`, 25, true},
		{"status complete", `{"message_type":"status","percent_done":1}`, 100, true},
		{"summary line", `{"message_type":"summary","snapshot_id":"abc"}`, 0, false},
		{"non-json", `Fatal: something broke`, 0, false},
		{"empty", ``, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := statusPercent([]byte(c.line))
			if ok != c.wantOK || got != c.want {
				t.Fatalf("statusPercent(%q) = (%v, %v); want (%v, %v)", c.line, got, ok, c.want, c.wantOK)
			}
		})
	}
}

// TestLastReasonPrefersInformativeLine pins the behaviour that surfaced the
// real cause to forum users: restic's data-corruption error ends with a generic
// "open an issue" trailer, but we must show the "Detected data corruption" line.
func TestLastReasonPrefersInformativeLine(t *testing.T) {
	stderr := `Fatal: unable to save snapshot: failed to save blob from file "/host/user/cache/vms/Windows 11/vdisk1.img": Detected data corruption while saving blob 52fefaee: hash mismatch
Corrupted blobs are either caused by hardware issues or software bugs. Please open an issue at https://github.com/restic/restic/issues/new/choose for further troubleshooting.`

	got := lastReason(stderr)
	if !strings.Contains(got, "Detected data corruption") {
		t.Fatalf("lastReason should surface the corruption cause, got %q", got)
	}
	if strings.Contains(got, "Please open an issue") {
		t.Fatalf("lastReason should NOT surface the generic boilerplate, got %q", got)
	}
	if strings.Contains(got, "Windows") {
		t.Fatalf("lastReason should scrub the host path, got %q", got)
	}
}

func TestLastReasonSingleLine(t *testing.T) {
	got := lastReason("Fatal: repository is already locked")
	if got != "Fatal: repository is already locked" {
		t.Fatalf("got %q", got)
	}
}

func TestLastReasonEmpty(t *testing.T) {
	if got := lastReason("   \n  \n"); got != "" {
		t.Fatalf("empty stderr should yield empty reason, got %q", got)
	}
}

// TestLastReasonAppendsItemErrorToCount pins that a count-only restore summary
// ("There were N errors") is enriched with the first concrete per-item error and
// that the host path in that sample is scrubbed.
func TestLastReasonAppendsItemErrorToCount(t *testing.T) {
	stderr := strings.Join([]string{
		"ignoring error for /host/user/bombvault/flash-restore/bzimage: Lchown: operation not permitted",
		"Fatal: There were 1104 errors",
	}, "\n")
	got := lastReason(stderr)
	if !strings.Contains(got, "There were 1104 errors") {
		t.Fatalf("should keep the count summary, got %q", got)
	}
	if !strings.Contains(got, "operation not permitted") {
		t.Fatalf("should append a concrete per-item cause, got %q", got)
	}
	if strings.Contains(got, "/host/user") {
		t.Fatalf("should scrub the host path, got %q", got)
	}
}
