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

// TestCopyStatusPercent pins the text-format parser that replaced the original
// (mistaken) "restic copy has no percentage at all" conclusion — issue #159.
// The real line shape, confirmed against upstream
// internal/ui/progress/terminal.go's newProgressMax and hands-on against the
// installed restic 0.17.3 binary, is "[M:SS] NN.NN%  X / Y packs copied"
// (or "[H:MM:SS] ..." past an hour). Deliberately loose (leading bracket +
// percent only), so a future wording change to the trailing "X / Y packs
// copied" text still parses — only a change to the bracket+percent PREFIX
// itself would stop parsing, and even then it degrades to "no percentage this
// line", never a crash (see copyStatusPercent's doc comment).
func TestCopyStatusPercent(t *testing.T) {
	cases := []struct {
		name   string
		line   string
		want   float64
		wantOK bool
	}{
		{"mid run", `[0:13] 50.00%  2 / 4 packs copied`, 50, true},
		{"complete", `[1:02] 100.00%  4 / 4 packs copied`, 100, true},
		{"past an hour", `[1:02:03] 12.50%  1 / 8 packs copied`, 12.5, true},
		{"no total yet (restic's own max==0 branch, no %% at all)", `[0:02]          0 packs copied`, 0, false},
		{"snapshot header", `  copy started, this may take a while...`, 0, false},
		{"summary/other", `snapshot abc123 saved, copied from source snapshot def456`, 0, false},
		{"non-json JSON line from an unrelated command", `{"message_type":"status","percent_done":0.5}`, 0, false},
		{"empty", ``, 0, false},
		{"over 100 clamped", `[0:01] 150.00%  9 / 4 packs copied`, 100, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := copyStatusPercent([]byte(c.line))
			if ok != c.wantOK || got != c.want {
				t.Fatalf("copyStatusPercent(%q) = (%v, %v); want (%v, %v)", c.line, got, ok, c.want, c.wantOK)
			}
		})
	}
}

// TestCopyStartedRegex pins the case-insensitive match for restic copy's
// per-snapshot section header, anchored to the START of the line (past any
// leading whitespace) but not the trailing wording ("...this may take a
// while..."), so a minor upstream punctuation tweak still advances the
// snapshot boundary while a user-controlled backed-up path that happens to
// CONTAIN the text "copy started" elsewhere in a line can no longer trigger
// a false boundary (see copyStartedRe's doc comment).
func TestCopyStartedRegex(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"  copy started, this may take a while...", true},
		{"Copy started", true},   // case-insensitive
		{"\tcopy started", true}, // any leading whitespace, not just two spaces
		{"[0:13] 50.00%  2 / 4 packs copied", false},
		{"snapshot abc123 saved, copied from source snapshot def456", false},
		// A backed-up path containing the literal text "copy started" must NOT
		// fire a spurious snapshot boundary just because the substring appears
		// somewhere mid-line — only a line that STARTS with it counts.
		{"snapshot abc123 of /mnt/user/copy started backups at 2026-08-16 10:00:00:", false},
		{"", false},
	}
	for _, c := range cases {
		if got := copyStartedRe.MatchString(c.line); got != c.want {
			t.Fatalf("copyStartedRe.MatchString(%q) = %v, want %v", c.line, got, c.want)
		}
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

// TestLastReasonDecodesJSONItemErrors pins the issue #110 fix: a restore run
// with --json (every BombVault restore) reports its per-file errors as
// {"message_type":"error",…} JSON objects on stderr, followed by a plain
// "Fatal: There were N errors" tally. The tally alone is useless — the decoded
// error.message causes must be surfaced (path-scrubbed), never the raw JSON.
// The stderr shape is taken from a live restic 0.17.3 repeat-restore repro.
func TestLastReasonDecodesJSONItemErrors(t *testing.T) {
	stderr := strings.Join([]string{
		`{"message_type":"error","error":{"message":"open /host/user/user/temp/host/user/appdata/plex/db.sqlite: file exists"},"during":"restore","item":"/host/user/appdata/plex/db.sqlite"}`,
		"Fatal: There were 1 errors",
	}, "\n")
	got := lastReason(stderr)
	if !strings.Contains(got, "There were 1 errors") {
		t.Fatalf("should keep the count summary, got %q", got)
	}
	if !strings.Contains(got, "file exists") {
		t.Fatalf("should surface the decoded per-item cause, got %q", got)
	}
	if strings.Contains(got, "/host/user") {
		t.Fatalf("should scrub the host path, got %q", got)
	}
	if strings.Contains(got, "message_type") {
		t.Fatalf("should never surface raw JSON, got %q", got)
	}
}

// TestLastReasonJSONCausesDedupedAndBounded pins the bounding contract: repeated
// identical causes collapse into one, at most three distinct causes are joined,
// and further distinct causes fold into a "(+N more)" count.
func TestLastReasonJSONCausesDedupedAndBounded(t *testing.T) {
	line := func(msg, item string) string {
		return `{"message_type":"error","error":{"message":"` + msg + `"},"during":"restore","item":"` + item + `"}`
	}
	stderr := strings.Join([]string{
		line("UtimesNano: operation not supported", "/a/1"),
		line("UtimesNano: operation not supported", "/a/2"), // duplicate cause — must collapse
		line("open /host/user/x: file exists", "/a/3"),
		line("symlink /host/user/y: invalid argument", "/a/4"),
		line("mkfifo /host/user/z: function not implemented", "/a/5"), // 4th distinct — folded
		"Fatal: There were 5 errors",
	}, "\n")
	got := lastReason(stderr)
	if strings.Count(got, "operation not supported") != 1 {
		t.Fatalf("identical causes must be deduplicated, got %q", got)
	}
	for _, want := range []string{"file exists", "invalid argument", "(+1 more)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("want %q in reason, got %q", want, got)
		}
	}
	if strings.Contains(got, "function not implemented") {
		t.Fatalf("4th distinct cause must be folded into the more-count, got %q", got)
	}
}

// TestLastReasonRawJSONLineDecoded pins that when the chosen reason line IS a
// raw per-item JSON error (restic died before printing its tally), the decoded
// cause is shown instead of the JSON object.
func TestLastReasonRawJSONLineDecoded(t *testing.T) {
	got := lastReason(`{"message_type":"error","error":{"message":"open /host/user/x: no space left on device"},"during":"restore","item":"/x"}`)
	if strings.Contains(got, "message_type") {
		t.Fatalf("raw JSON must be decoded, got %q", got)
	}
	if !strings.Contains(got, "no space left on device") {
		t.Fatalf("decoded cause must be surfaced, got %q", got)
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

// TestIsMetadataOnlyRestoreFailureJSONForm pins that the classifier ALSO
// understands restic's --json per-item error objects — the phrasing every
// BombVault restore actually produces (the text "ignoring error for …" form
// only appears without --json). Before issue #110 the JSON form was never
// recognized, so the files-restore success-with-warning downgrade could never
// fire in production.
func TestIsMetadataOnlyRestoreFailureJSONForm(t *testing.T) {
	permLine := `{"message_type":"error","error":{"message":"Lchown: lchown /host/user/restore/docs/a.txt: operation not permitted"},"during":"restore","item":"/docs/a.txt"}`
	metaOnly := strings.Join([]string{
		permLine,
		`{"message_type":"error","error":{"message":"UtimesNano: operation not permitted"},"during":"restore","item":"/docs/b.txt"}`,
		"Fatal: There were 2 errors",
	}, "\n")
	if !isMetadataOnlyRestoreFailure(metaOnly) {
		t.Fatal("all-permission JSON stderr must classify as metadata-only")
	}

	// A non-permission JSON per-item error mixed in is a GENUINE failure.
	mixed := strings.Join([]string{
		permLine,
		`{"message_type":"error","error":{"message":"open /host/user/restore/docs/big.bin: no space left on device"},"during":"restore","item":"/docs/big.bin"}`,
		"Fatal: There were 2 errors",
	}, "\n")
	if isMetadataOnlyRestoreFailure(mixed) {
		t.Fatal("a non-permission JSON per-item error must NOT be metadata-only")
	}

	// runError must tag the sentinel for the JSON form too.
	err := runError([]string{"-r", "/repo", "restore", "--target", "/t", "--", "abc:/p"},
		permLine+"\nFatal: There were 1 errors")
	if !errors.Is(err, ErrRestoreMetadataOnly) {
		t.Fatalf("JSON-form metadata-only restore failure must wrap ErrRestoreMetadataOnly, got %v", err)
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
