package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestFirstOffsiteFailureAfterNamesTheEarliestFailureThatSaysSomethingAboutTheTarget(t *testing.T) {
	r := newRepo(t)
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
	for _, c := range []struct{ since, want int64 }{{0, 600}, {200, 600}, {650, 700}, {700, 0}} {
		got, err := r.FirstOffsiteFailureAfter("t1", c.since)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("FirstOffsiteFailureAfter(t1, %d) = %d, want %d", c.since, got, c.want)
		}
	}
}

func TestFirstOffsiteFailureAfterForgetsAFailureALaterSuccessAnswered(t *testing.T) {
	r := newRepo(t)
	for _, run := range []struct {
		started int64
		ok      bool
		errText string
	}{{100, false, "timeout"}, {200, true, ""}, {300, true, ""}} {
		id, err := r.RecordOffsiteRunForTarget("containers", "t1", run.started)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.FinishOffsiteRun(id, run.ok, run.errText); err != nil {
			t.Fatal(err)
		}
	}
	got, err := r.FirstOffsiteFailureAfter("t1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Errorf("FirstOffsiteFailureAfter(t1, 0) = %d, want 0: the target answered twice since", got)
	}
}
