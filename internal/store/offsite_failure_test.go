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

func TestAcknowledgedCopyFailuresStopCountingAndALaterOneCountsAgain(t *testing.T) {
	r := newRepo(t)
	run := func(target string, started int64, ok bool) int64 {
		t.Helper()
		id, err := r.RecordOffsiteRunForTarget("containers", target, started)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.FinishOffsiteRun(id, ok, "timeout"); err != nil {
			t.Fatal(err)
		}
		return id
	}
	open := func(target string) int64 {
		t.Helper()
		at, err := r.FirstUnacknowledgedOffsiteFailureAfter(target, 0)
		if err != nil {
			t.Fatal(err)
		}
		return at
	}
	run("t1", 100, false)
	run("t1", 200, false)
	run("t2", 150, false)
	if _, err := r.RecordOffsiteRunForTarget("containers", "t1", 250); err != nil {
		t.Fatal(err)
	}

	n, err := r.AcknowledgeFailedCopies([]string{"t1"})
	if err != nil || n != 2 {
		t.Fatalf("acknowledged %d runs, %v, want the two that failed and not the one still running", n, err)
	}

	if got := open("t1"); got != 0 {
		t.Errorf("t1 still has an open failure at %d", got)
	}
	if got := open("t2"); got != 150 {
		t.Errorf("t2's open failure = %d, want 150: nobody acknowledged it", got)
	}
	if at, _ := r.FirstOffsiteFailureAfter("t1", 0); at != 100 {
		t.Errorf("FirstOffsiteFailureAfter(t1) = %d, want 100: an acknowledged failure is still one", at)
	}

	run("t1", 300, false)
	if got := open("t1"); got != 300 {
		t.Errorf("t1's open failure = %d, want the new one at 300", got)
	}
	if n, err := r.AcknowledgeAllFailedCopies(); err != nil || n != 2 {
		t.Fatalf("acknowledging everything changed %d runs, %v, want t1's new one and t2's", n, err)
	}
	if n, _ := r.AcknowledgeFailedCopies(nil); n != 0 {
		t.Fatalf("an empty list acknowledged %d runs", n)
	}
}
