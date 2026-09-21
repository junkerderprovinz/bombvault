package restic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/progress"
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

// TestCopyStatusPercent covers the text progress line of restic copy,
// "[M:SS] NN.NN%  X / Y packs copied" or "[H:MM:SS] ..." past an hour, as
// printed by newProgressMax in restic's internal/ui/progress/terminal.go. Only
// the bracket and percent are matched, so new wording after them still parses.
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

// TestCopyStartedRegex covers the per-snapshot header of restic copy. The match
// ignores case, leading whitespace and the wording after "copy started", and is
// anchored to the start of the line so a backed-up path containing the phrase
// does not start a new snapshot.
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
		// "copy started" inside a path, not at the start of the line.
		{"snapshot abc123 of /mnt/user/copy started backups at 2026-08-16 10:00:00:", false},
		{"", false},
	}
	for _, c := range cases {
		if got := copyStartedRe.MatchString(c.line); got != c.want {
			t.Fatalf("copyStartedRe.MatchString(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

// TestLastReasonPrefersInformativeLine checks that restic's data-corruption
// error is reported by its "Detected data corruption" line, not by the generic
// "open an issue" trailer that follows it.
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

// TestLastReasonAppendsItemErrorToCount checks that a bare "There were N errors"
// summary gets the first per-item error appended, with its host path scrubbed.
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

// TestLastReasonDecodesJSONItemErrors covers restore with --json, which every
// BombVault restore uses. Per-file errors then arrive on stderr as
// {"message_type":"error",...} objects before a plain "Fatal: There were N
// errors" tally. The tally alone says nothing, so the decoded, path-scrubbed
// messages are shown, never the raw JSON. The stderr is from restic 0.17.3.
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

// TestLastReasonJSONCausesDedupedAndBounded checks that identical causes
// collapse into one, at most three distinct causes are joined, and the rest
// fold into a "(+N more)" count.
func TestLastReasonJSONCausesDedupedAndBounded(t *testing.T) {
	line := func(msg, item string) string {
		return `{"message_type":"error","error":{"message":"` + msg + `"},"during":"restore","item":"` + item + `"}`
	}
	stderr := strings.Join([]string{
		line("UtimesNano: operation not supported", "/a/1"),
		line("UtimesNano: operation not supported", "/a/2"), // duplicate cause
		line("open /host/user/x: file exists", "/a/3"),
		line("symlink /host/user/y: invalid argument", "/a/4"),
		line("mkfifo /host/user/z: function not implemented", "/a/5"), // fourth distinct cause
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

// TestLastReasonRawJSONLineDecoded checks that a per-item JSON error chosen as
// the reason line, as when restic dies before printing its tally, is shown
// decoded.
func TestLastReasonRawJSONLineDecoded(t *testing.T) {
	got := lastReason(`{"message_type":"error","error":{"message":"open /host/user/x: no space left on device"},"during":"restore","item":"/x"}`)
	if strings.Contains(got, "message_type") {
		t.Fatalf("raw JSON must be decoded, got %q", got)
	}
	if !strings.Contains(got, "no space left on device") {
		t.Fatalf("decoded cause must be surfaced, got %q", got)
	}
}

// TestIsMetadataOnlyRestoreFailure covers the classifier that downgrades a files
// restore to a success with a warning. It holds only when every error is an
// ownership or permission error on the target (the /mnt/user FUSE case), so a
// real data, space or fatal error is never masked.
func TestIsMetadataOnlyRestoreFailure(t *testing.T) {
	metaOnly := strings.Join([]string{
		"ignoring error for /host/user/restore/docs/a.txt: Lchown: operation not permitted",
		"ignoring error for /host/user/restore/docs/b.txt: Lchown: operation not permitted",
		"Fatal: There were 2 errors",
	}, "\n")
	if !isMetadataOnlyRestoreFailure(metaOnly) {
		t.Fatal("all-metadata-permission stderr must classify as metadata-only")
	}

	withRealError := strings.Join([]string{
		"ignoring error for /host/user/restore/docs/a.txt: Lchown: operation not permitted",
		"ignoring error for /host/user/restore/docs/big.bin: no space left on device",
		"Fatal: There were 2 errors",
	}, "\n")
	if isMetadataOnlyRestoreFailure(withRealError) {
		t.Fatal("a no-space per-file error must NOT be treated as metadata-only")
	}

	if isMetadataOnlyRestoreFailure("Fatal: unable to load snapshot: no matching ID found") {
		t.Fatal("a fatal load error must NOT be metadata-only")
	}

	if isMetadataOnlyRestoreFailure("") {
		t.Fatal("empty stderr must NOT be metadata-only")
	}
}

// TestIsMetadataOnlyRestoreFailureJSONForm covers restic's --json error
// objects, which is what every BombVault restore produces. The text form
// "ignoring error for ..." only appears without --json.
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

	mixed := strings.Join([]string{
		permLine,
		`{"message_type":"error","error":{"message":"open /host/user/restore/docs/big.bin: no space left on device"},"during":"restore","item":"/docs/big.bin"}`,
		"Fatal: There were 2 errors",
	}, "\n")
	if isMetadataOnlyRestoreFailure(mixed) {
		t.Fatal("a non-permission JSON per-item error must NOT be metadata-only")
	}

	err := runError([]string{"-r", "/repo", "restore", "--target", "/t", "--", "abc:/p"},
		permLine+"\nFatal: There were 1 errors")
	if !errors.Is(err, ErrRestoreMetadataOnly) {
		t.Fatalf("JSON-form metadata-only restore failure must wrap ErrRestoreMetadataOnly, got %v", err)
	}
}

// TestRunErrorTagsMetadataOnlyRestore checks that runError wraps
// ErrRestoreMetadataOnly for a metadata-only restore failure without changing
// the message, and tags neither real failures nor other subcommands.
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

	genuine := runError([]string{"-r", "/repo", "restore"}, "Fatal: unable to load snapshot: no matching ID found")
	if errors.Is(genuine, ErrRestoreMetadataOnly) {
		t.Fatalf("a genuine restore failure must not be tagged metadata-only, got %v", genuine)
	}

	backup := runError([]string{"-r", "/repo", "backup"}, "ignoring error for /x: operation not permitted\nFatal: There were 1 errors")
	if errors.Is(backup, ErrRestoreMetadataOnly) {
		t.Fatalf("only the restore subcommand may be tagged metadata-only, got %v", backup)
	}
}

// TestWithAddedWatcherChains checks that a second watcher joins the one already
// on the context instead of replacing it. A dump reports to the stall guard and
// to the byte publisher on the same restic call.
func TestWithAddedWatcherChains(t *testing.T) {
	t.Run("nil leaves the context alone", func(t *testing.T) {
		ctx := context.Background()
		if WithAddedWatcher(ctx, nil) != ctx {
			t.Fatal("a nil watcher must return the same context")
		}
		first := func(Progress) {}
		withFirst := WithWatcher(ctx, first)
		if WatcherFrom(WithAddedWatcher(withFirst, nil)) == nil {
			t.Fatal("a nil watcher must leave the existing one in place")
		}
	})
	t.Run("both watchers see every line in order", func(t *testing.T) {
		var seen []string
		ctx := WithWatcher(context.Background(), func(p Progress) {
			seen = append(seen, fmt.Sprintf("first:%d", p.BytesDone))
		})
		ctx = WithAddedWatcher(ctx, func(p Progress) {
			seen = append(seen, fmt.Sprintf("second:%d", p.BytesDone))
		})
		watch := WatcherFrom(ctx)
		for _, done := range []uint64{10, 20} {
			p, ok := ParseProgress([]byte(fmt.Sprintf(`{"message_type":"status","bytes_done":%d}`, done)))
			if !ok {
				t.Fatalf("status line for %d did not parse", done)
			}
			watch(p)
		}
		want := []string{"first:10", "second:10", "first:20", "second:20"}
		if !reflect.DeepEqual(seen, want) {
			t.Fatalf("watchers saw %v, want %v", seen, want)
		}
	})
	t.Run("an added watcher alone is reachable", func(t *testing.T) {
		var got uint64
		ctx := WithAddedWatcher(context.Background(), func(p Progress) { got = p.BytesDone })
		p, _ := ParseProgress([]byte(`{"message_type":"status","bytes_done":7}`))
		WatcherFrom(ctx)(p)
		if got != 7 {
			t.Fatalf("bytes_done = %d, want 7", got)
		}
	})
}

// TestParseBackupSummaryReadsTotalBytesProcessed covers the field a dump is
// measured by: it must equal the byte count the helper reported.
func TestParseBackupSummaryReadsTotalBytesProcessed(t *testing.T) {
	out := []byte(`{"message_type":"status","percent_done":0.5}
{"message_type":"summary","files_new":1,"files_changed":0,"data_added":4096,"total_bytes_processed":1048576,"snapshot_id":"deadbeef"}
`)
	sum, err := ParseBackupSummary(out)
	if err != nil {
		t.Fatalf("ParseBackupSummary: %v", err)
	}
	if sum.TotalBytesProcessed != 1048576 {
		t.Fatalf("TotalBytesProcessed = %d, want 1048576", sum.TotalBytesProcessed)
	}
	if sum.SnapshotID != "deadbeef" {
		t.Fatalf("SnapshotID = %q, want deadbeef", sum.SnapshotID)
	}
}

// TestBackupFromCommandNeverFeedsTheSink checks that a dump's status lines
// reach the watcher but not the progress sink on the context. The sink carries
// the container backup's own percentage, and a stage-less event from the dump
// call would wipe the card's dump stage.
func TestBackupFromCommandNeverFeedsTheSink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell to exec a shebang script as the fake restic binary")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "fake-restic.sh")
	body := "#!/bin/sh\n" +
		"echo '{\"message_type\":\"status\",\"percent_done\":0.25,\"bytes_done\":100}'\n" +
		"echo '{\"message_type\":\"status\",\"percent_done\":0.75,\"bytes_done\":300}'\n" +
		"echo '{\"message_type\":\"summary\",\"snapshot_id\":\"abc123\",\"total_bytes_processed\":300}'\n" +
		"echo 'subprocess bombvault: bombvault-dbdump-pid 4711' >&2\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil { //nolint:gosec // G306: test-only helper script, needs the exec bit
		t.Fatalf("write fake restic script: %v", err)
	}

	var sinkCalls int
	var watched []uint64
	ctx := progress.WithSink(context.Background(), func(float64) { sinkCalls++ })
	ctx = WithWatcher(ctx, func(p Progress) { watched = append(watched, p.BytesDone) })

	r := Restic{Bin: script}
	sum, lines, err := r.BackupFromCommand(ctx, "/repo", "/dbdump/pg.sql", []string{"dbdump:pg"},
		[]string{"/usr/local/bin/bombvault", "dbdump-stream"}, Mode{Encrypted: false})
	if err != nil {
		t.Fatalf("BackupFromCommand: %v", err)
	}
	if sum.SnapshotID != "abc123" || sum.TotalBytesProcessed != 300 {
		t.Fatalf("summary = %+v, want snapshot abc123 and 300 bytes", sum)
	}
	if !reflect.DeepEqual(watched, []uint64{100, 300}) {
		t.Fatalf("watcher saw %v, want every status line", watched)
	}
	if sinkCalls != 0 {
		t.Fatalf("the sink was called %d times, want none", sinkCalls)
	}
	if !reflect.DeepEqual(lines, []string{"subprocess bombvault: bombvault-dbdump-pid 4711"}) {
		t.Fatalf("subprocess lines = %v", lines)
	}
}

// TestFailedCommandBackupLogsOnlyResticsOwnLines checks that the log of a
// failed dump backup leaves out what the dump command wrote, which can quote a
// database row, while every other subcommand still logs its whole stderr.
func TestFailedCommandBackupLogsOnlyResticsOwnLines(t *testing.T) {
	const row = "subprocess bombvault: ERROR: duplicate key (email)=(alice@example.com)"
	logged := func(fn func()) string {
		var buf bytes.Buffer
		prev, flags := log.Writer(), log.Flags()
		log.SetOutput(&buf)
		log.SetFlags(0)
		defer func() { log.SetOutput(prev); log.SetFlags(flags) }()
		fn()
		return buf.String()
	}

	t.Run("other subcommands", func(t *testing.T) {
		out := logged(func() { _ = runError([]string{"-r", "/repo", "backup"}, row+"\nFatal: unable to save snapshot") })
		if !strings.Contains(out, "alice@example.com") {
			t.Fatalf("a plain backup's log lost part of its stderr:\n%s", out)
		}
	})

	t.Run("a backup from a command", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("needs a POSIX shell to exec a shebang script as the fake restic binary")
		}
		script := filepath.Join(t.TempDir(), "fake-restic.sh")
		body := "#!/bin/sh\n" +
			"echo 'subprocess bombvault: bombvault-dbdump-pid 4711' >&2\n" +
			"echo '" + row + "' >&2\n" +
			"echo 'Fatal: unable to save snapshot: repository is already locked' >&2\n" +
			"exit 1\n"
		if err := os.WriteFile(script, []byte(body), 0o700); err != nil { //nolint:gosec // G306: test-only helper script, needs the exec bit
			t.Fatalf("write fake restic script: %v", err)
		}

		var lines []string
		var err error
		out := logged(func() {
			_, lines, err = Restic{Bin: script}.BackupFromCommand(context.Background(), "/repo", "/dbdump/pg.sql", nil,
				[]string{"/usr/local/bin/bombvault", "dbdump-stream"}, Mode{Encrypted: false})
		})
		if err == nil {
			t.Fatal("BackupFromCommand succeeded, want the failure the fake restic reported")
		}
		if strings.Contains(out, "alice@example.com") || strings.Contains(out, "dbdump-pid") {
			t.Errorf("the log carries a line the dump command wrote:\n%s", out)
		}
		if !strings.Contains(out, "repository is already locked") {
			t.Errorf("the log lost restic's own line:\n%s", out)
		}
		if len(lines) != 2 {
			t.Errorf("subprocess lines = %v, want both for the caller", lines)
		}
	})
}
