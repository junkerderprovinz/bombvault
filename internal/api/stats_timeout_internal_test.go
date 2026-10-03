package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// slowStatsEngine answers Snapshots at once and then walks until its context
// ends, the way restic stats behaves on a large remote repository, or fails
// outright when fail is set.
type slowStatsEngine struct {
	ResticEngine
	fail error
}

func (e *slowStatsEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return []restic.Snapshot{{ID: "abc", Time: "2026-10-03T00:00:00Z"}}, nil
}

func (e *slowStatsEngine) Stats(ctx context.Context, _, _ string, _ restic.Mode) (restic.StatsResult, error) {
	if e.fail != nil {
		return restic.StatsResult{}, e.fail
	}
	<-ctx.Done()
	return restic.StatsResult{}, fmt.Errorf("restic stats cancelled: %w", ctx.Err())
}

func logOf(fn func()) string {
	var buf bytes.Buffer
	prev, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() { log.SetOutput(prev); log.SetFlags(flags) }()
	fn()
	return buf.String()
}

func TestStatsTimeoutGivesOffsiteRepositoriesTheRemoteBound(t *testing.T) {
	svc, _ := statsTestService(t, &statsFakeEngine{})
	for _, source := range []string{"offsite", "offsite:4dec8deb76acfcaf5c871736fe28f179"} {
		if got := svc.statsTimeout("containers", source); got != statsRemoteTimeout {
			t.Errorf("%s: timeout %v, want %v", source, got, statsRemoteTimeout)
		}
	}
	if got := svc.statsTimeout("containers", "local"); got != statsLocalTimeout {
		t.Errorf("a local repository on disk: timeout %v, want %v", got, statsLocalTimeout)
	}
	if statsRemoteTimeout <= statsLocalTimeout {
		t.Errorf("the remote bound %v must be longer than the local %v", statsRemoteTimeout, statsLocalTimeout)
	}
}

func TestStatsSampleThatRunsOutOfTimeIsANoteNotAFailure(t *testing.T) {
	prev := statsLocalTimeout
	statsLocalTimeout = 50 * time.Millisecond
	t.Cleanup(func() { statsLocalTimeout = prev })

	svc, st := statsTestService(t, &slowStatsEngine{})
	out := logOf(func() { svc.sampleInBackground(context.Background(), "containers", "local") })

	if strings.Count(strings.TrimSpace(out), "\n") != 0 || out == "" {
		t.Fatalf("want exactly one log line, got:\n%s", out)
	}
	for _, alarm := range []string{"failed", "error", "deadline"} {
		if strings.Contains(strings.ToLower(out), alarm) {
			t.Fatalf("a sample that ran out of time reads as an alarm (%q):\n%s", alarm, out)
		}
	}
	if !strings.Contains(out, "measures it again") {
		t.Fatalf("the line must say what happens next:\n%s", out)
	}
	if rows, err := st.ListRepoStats("containers", "local", 0); err != nil || len(rows) != 0 {
		t.Fatalf("a stopped sample must record nothing, got %d rows err=%v", len(rows), err)
	}
}

func TestStatsSampleThatFailsStillSaysSo(t *testing.T) {
	svc, _ := statsTestService(t, &slowStatsEngine{fail: errors.New("restic stats failed: Fatal: unable to open repository")})
	out := logOf(func() { svc.sampleInBackground(context.Background(), "containers", "local") })

	if !strings.Contains(out, "failed") || !strings.Contains(out, "unable to open repository") {
		t.Fatalf("a real failure must be logged with its reason, got:\n%s", out)
	}
}
