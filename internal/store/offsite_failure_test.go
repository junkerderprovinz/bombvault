package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// recordFailureScene records runs to t1 and t2: failures, a success, one on
// unreadable copy rules and one still running.
func recordFailureScene(t *testing.T, r *store.Repo) {
	t.Helper()
	runs := []struct {
		target  string
		started int64
		ok      bool
		errText string
		open    bool
	}{
		{target: "t1", started: 100, errText: "timeout"},
		{target: "t1", started: 300, ok: true},
		{target: "t1", started: 400, errText: store.ReasonCopyRulesUnreadable},
		{target: "t1", started: 500, open: true},
		{target: "t2", started: 550, errText: "timeout"},
		{target: "t1", started: 700, errText: "timeout"},
		{target: "t1", started: 600, errText: "connection refused"},
	}
	for _, run := range runs {
		id, err := r.RecordOffsiteRunForTarget("containers", run.target, run.started)
		if err != nil {
			t.Fatal(err)
		}
		if run.open {
			continue
		}
		if err := r.FinishOffsiteRun(id, run.ok, run.errText); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFirstOffsiteFailureAfterNamesTheEarliestFailureThatSaysSomethingAboutTheTarget(t *testing.T) {
	r := newRepo(t)
	recordFailureScene(t, r)
	for _, c := range []struct{ since, want int64 }{{200, 600}, {650, 700}, {700, 0}} {
		got, err := r.FirstOffsiteFailureAfter("t1", c.since)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("FirstOffsiteFailureAfter(t1, %d) = %d, want %d", c.since, got, c.want)
		}
	}
}

func TestLatestOffsiteFailureAfterNamesTheNewestFailureWithItsReason(t *testing.T) {
	r := newRepo(t)
	recordFailureScene(t, r)
	for _, c := range []struct {
		since   int64
		want    int64
		errText string
	}{{200, 700, "timeout"}, {650, 700, "timeout"}, {700, 0, ""}} {
		got, found, err := r.LatestOffsiteFailureAfter("t1", c.since)
		if err != nil {
			t.Fatal(err)
		}
		if found != (c.want != 0) || got.StartedAt != c.want || got.Error != c.errText || got.OK {
			t.Errorf("LatestOffsiteFailureAfter(t1, %d) = %+v, %v, want %d with %q", c.since, got, found, c.want, c.errText)
		}
	}
}
