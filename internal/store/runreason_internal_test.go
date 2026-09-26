package store

// The frontend translates a run reason by matching the exact English string,
// or the head of one that carries a detail, against the tables in
// web/src/lib/runReason.ts. Rewording a constant here breaks no build, it only
// leaves the reason untranslated, so these tests read those tables.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// namedReasons returns every reason and note the frontend has to recognise.
func namedReasons() map[string]string {
	return map[string]string{
		"ReasonInterrupted":         ReasonInterrupted,
		"ReasonShutdown":            ReasonShutdown,
		"ReasonContainerGone":       ReasonContainerGone,
		"ReasonCancelled":           ReasonCancelled,
		"ReasonStalled":             ReasonStalled,
		"ReasonDBDumpAuth":          ReasonDBDumpAuth,
		"ReasonDBDumpPrivileges":    ReasonDBDumpPrivileges,
		"ReasonDBDumpUnreachable":   ReasonDBDumpUnreachable,
		"ReasonDBDumpNoClient":      ReasonDBDumpNoClient,
		"ReasonDBDumpSecret":        ReasonDBDumpSecret,
		"ReasonDBDumpNoCredentials": ReasonDBDumpNoCredentials,
		"ReasonDBDumpNotRunning":    ReasonDBDumpNotRunning,
		"ReasonDBDumpNeedsUpgrade":  ReasonDBDumpNeedsUpgrade,
		"ReasonDBDumpTimeout":       ReasonDBDumpTimeout,
		"ReasonDBDumpBackupCap":     ReasonDBDumpBackupCap,
		"ReasonDBDumpStalled":       ReasonDBDumpStalled,
		"ReasonDBDumpEmpty":         ReasonDBDumpEmpty,
		"ReasonDBDumpIncomplete":    ReasonDBDumpIncomplete,
		"ReasonDBDumpTool":          ReasonDBDumpTool,
		"ReasonDBDumpDocker":        ReasonDBDumpDocker,
		"ReasonDBDumpRepository":    ReasonDBDumpRepository,
		"ReasonDBDumpHelper":        ReasonDBDumpHelper,
		"ReasonDBDumpMismatch":      ReasonDBDumpMismatch,
		"ReasonDBDumpLeftover":      ReasonDBDumpLeftover,
		"NoteDBDumpOneDatabase":     NoteDBDumpOneDatabase,
		"NoteDBDumpNotRecorded":     NoteDBDumpNotRecorded,
		"ReasonDBImportPrepare":     ReasonDBImportPrepare,
		"ReasonDBImportRollback":    ReasonDBImportRollback,
		"ReasonDBImportFailed":      ReasonDBImportFailed,
		"NoteDBImportKeptOld":       NoteDBImportKeptOld,
		"NoteDBImportErrors":        NoteDBImportErrors,
		"ImportTailAppsDown":        ImportTailAppsDown,
		"ImportTailAppsStopped":     ImportTailAppsStopped,
	}
}

func TestRunReasonsMatchTheFrontend(t *testing.T) {
	path := filepath.Join("..", "..", "web", "src", "lib", "runReason.ts")
	raw, err := os.ReadFile(path) //nolint:gosec // G304: fixed repo-relative path built one line above, not input
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	table := string(raw)

	for name, reason := range namedReasons() {
		// A bare substring check would still pass after a partial reword.
		if !strings.Contains(table, `"`+reason+`"`) {
			t.Errorf("%s = %q is not in runReason.ts, so the UI will show it untranslated.\n"+
				"Change both, or neither.", name, reason)
		}
	}
}

func TestRunReasonsAreDistinct(t *testing.T) {
	// The frontend keys off the string itself, so two reasons that are equal
	// would silently collapse into one translation. A reason that is the
	// beginning of another one is just as bad: the reasons that carry a detail
	// are matched by prefix, and the shorter one would swallow the longer.
	reasons := namedReasons()
	seen := map[string]string{}
	for name, r := range reasons {
		if r == "" {
			t.Errorf("%s is empty", name)
		}
		if prev, dup := seen[r]; dup {
			t.Errorf("%s and %s are the same string %q", name, prev, r)
		}
		seen[r] = name
	}
	for name, r := range reasons {
		for otherName, other := range reasons {
			if name == otherName || r == other {
				continue
			}
			if strings.HasPrefix(other, r) {
				t.Errorf("%s = %q begins %s = %q, so a detail cannot be told from a longer reason",
					name, r, otherName, other)
			}
		}
	}
}

// The frontend reads the hours and the dataset back out of this sentence, so
// its shape is pinned here as well as in runReason.test.ts.
func TestStalledReasonNamesTheHoursAndTheDataset(t *testing.T) {
	cases := []struct {
		after   time.Duration
		dataset string
		want    string
	}{
		{time.Hour, "", "stopped by the stall guard after 1 hour without progress"},
		{2 * time.Hour, "", "stopped by the stall guard after 2 hours without progress"},
		{time.Hour, "tank/appdata/plex", "stopped by the stall guard after 1 hour without progress while reading tank/appdata/plex"},
	}
	for _, c := range cases {
		if got := StalledReason(c.after, c.dataset); got != c.want {
			t.Errorf("StalledReason(%v, %q) = %q, want %q", c.after, c.dataset, got, c.want)
		}
	}
}

func TestReapWritesTheNamedReason(t *testing.T) {
	// If ReapInterruptedRuns wrote a literal instead of the constant, the
	// frontend match could break without anything noticing.
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := New(db)

	runID, err := r.StartRun("t-reap", "backup")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	n, err := r.ReapInterruptedRuns()
	if err != nil {
		t.Fatalf("ReapInterruptedRuns: %v", err)
	}
	if n != 1 {
		t.Fatalf("reaped %d runs, want 1", n)
	}

	var status, errMsg string
	if qErr := db.QueryRow(`SELECT status, error FROM runs WHERE id = ?`, runID).
		Scan(&status, &errMsg); qErr != nil {
		t.Fatalf("read back: %v", qErr)
	}
	if status != "failed" {
		t.Errorf("status = %q, want failed", status)
	}
	if errMsg != ReasonInterrupted {
		t.Errorf("error = %q, want the ReasonInterrupted constant %q", errMsg, ReasonInterrupted)
	}
}
