package restic

import (
	"errors"
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

// TestIsMetadataOnlyRestoreFailure pins the Part-3 classifier used to downgrade a
// files restore to success-with-warning: it is true ONLY when every error is a
// per-file ownership/permission error on the target (the /mnt/user FUSE case), and
// false the moment any genuine data/space/fatal error appears — so a real failure
// is never masked.
func TestIsMetadataOnlyRestoreFailure(t *testing.T) {
	metaOnly := strings.Join([]string{
		"ignoring error for /host/user/restore/docs/a.txt: Lchown: operation not permitted",
		"ignoring error for /host/user/restore/docs/b.txt: Lchown: operation not permitted",
		"Fatal: There were 2 errors",
	}, "\n")
	if !isMetadataOnlyRestoreFailure(metaOnly) {
		t.Fatal("all-metadata-permission stderr must classify as metadata-only")
	}

	// A no-space per-file error mixed in is a GENUINE failure — must not be masked.
	withRealError := strings.Join([]string{
		"ignoring error for /host/user/restore/docs/a.txt: Lchown: operation not permitted",
		"ignoring error for /host/user/restore/docs/big.bin: no space left on device",
		"Fatal: There were 2 errors",
	}, "\n")
	if isMetadataOnlyRestoreFailure(withRealError) {
		t.Fatal("a no-space per-file error must NOT be treated as metadata-only")
	}

	// A hard fatal (missing snapshot / unreachable repo) is never metadata-only.
	if isMetadataOnlyRestoreFailure("Fatal: unable to load snapshot: no matching ID found") {
		t.Fatal("a fatal load error must NOT be metadata-only")
	}

	// No error lines at all → nothing to downgrade.
	if isMetadataOnlyRestoreFailure("") {
		t.Fatal("empty stderr must NOT be metadata-only")
	}
}

// TestRunErrorTagsMetadataOnlyRestore pins that runError wraps ErrRestoreMetadataOnly
// for a metadata-only RESTORE failure while keeping the displayed message identical
// (so the container to-path restore is unchanged), and never tags genuine failures
// or non-restore subcommands.
func TestRunErrorTagsMetadataOnlyRestore(t *testing.T) {
	stderr := "ignoring error for /host/user/restore/docs/a.txt: Lchown: operation not permitted\nFatal: There were 1 errors"
	err := runError([]string{"-r", "/repo", "restore", "--target", "/t", "--", "abc:/p"}, stderr)
	if !errors.Is(err, ErrRestoreMetadataOnly) {
		t.Fatalf("a metadata-only restore failure must wrap ErrRestoreMetadataOnly, got %v", err)
	}
	if !strings.HasPrefix(err.Error(), "restic restore failed:") {
		t.Fatalf("message text must be preserved (unchanged for other callers), got %q", err.Error())
	}
	if strings.Contains(err.Error(), "/host/user") {
		t.Fatalf("host path must be scrubbed, got %q", err.Error())
	}

	// A genuine restore failure must NOT wrap the sentinel.
	genuine := runError([]string{"-r", "/repo", "restore"}, "Fatal: unable to load snapshot: no matching ID found")
	if errors.Is(genuine, ErrRestoreMetadataOnly) {
		t.Fatalf("a genuine restore failure must not be tagged metadata-only, got %v", genuine)
	}

	// The same permission text on a NON-restore subcommand stays a plain failure.
	backup := runError([]string{"-r", "/repo", "backup"}, "ignoring error for /x: operation not permitted\nFatal: There were 1 errors")
	if errors.Is(backup, ErrRestoreMetadataOnly) {
		t.Fatalf("only the restore subcommand may be tagged metadata-only, got %v", backup)
	}
}
