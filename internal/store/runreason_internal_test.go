package store

// The frontend translates a run reason by matching the exact English string in
// RUN_REASONS (web/src/lib/runReason.ts). Rewording a constant here breaks no
// build, it only leaves the reason untranslated, so these tests read that table.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReasonsMatchTheFrontend(t *testing.T) {
	path := filepath.Join("..", "..", "web", "src", "lib", "runReason.ts")
	raw, err := os.ReadFile(path) //nolint:gosec // G304: fixed repo-relative path built one line above, not input
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	table := string(raw)

	for _, tc := range []struct{ name, reason string }{
		{"ReasonInterrupted", ReasonInterrupted},
		{"ReasonShutdown", ReasonShutdown},
		{"ReasonContainerGone", ReasonContainerGone},
		{"ReasonCancelled", ReasonCancelled},
	} {
		// A bare substring check would still pass after a partial reword.
		if !strings.Contains(table, `"`+tc.reason+`"`) {
			t.Errorf("%s = %q is not in runReason.ts, so the UI will show it untranslated.\n"+
				"Change both, or neither.", tc.name, tc.reason)
		}
	}
}

func TestRunReasonsAreDistinct(t *testing.T) {
	// The frontend keys off the string itself, so two reasons that are equal
	// would silently collapse into one translation.
	seen := map[string]string{}
	for name, r := range map[string]string{
		"ReasonInterrupted":   ReasonInterrupted,
		"ReasonShutdown":      ReasonShutdown,
		"ReasonContainerGone": ReasonContainerGone,
	} {
		if r == "" {
			t.Errorf("%s is empty", name)
		}
		if prev, dup := seen[r]; dup {
			t.Errorf("%s and %s are the same string %q", name, prev, r)
		}
		seen[r] = name
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
