package store_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func recordEvent(t *testing.T, r *store.Repo, keyID string, e store.MCPKeyEvent) {
	t.Helper()
	if err := r.RecordMCPKeyEvent(keyID, e); err != nil {
		t.Fatalf("RecordMCPKeyEvent(%s, %+v): %v", keyID, e, err)
	}
}

func TestMCPKeyEventsComeNewestFirstPerKey(t *testing.T) {
	r := newMCPRepo(t)
	addMCPKey(t, r, "laptop", "Laptop", true, 1000)
	addMCPKey(t, r, "desktop", "Desktop", true, 1000)

	recordEvent(t, r, "laptop", store.MCPKeyEvent{At: 2000, Tool: "get_health", Outcome: "ok"})
	recordEvent(t, r, "desktop", store.MCPKeyEvent{At: 2001, Tool: "list_runs", Outcome: "ok"})
	recordEvent(t, r, "laptop", store.MCPKeyEvent{At: 2002, Tool: "cancel_backup", Outcome: "ok", RunID: "run1"})
	recordEvent(t, r, "laptop", store.MCPKeyEvent{At: 2002, Outcome: "rate_limited"})

	got, err := r.MCPKeyEvents("laptop")
	if err != nil {
		t.Fatal(err)
	}
	want := []store.MCPKeyEvent{
		{At: 2002, Outcome: "rate_limited"},
		{At: 2002, Tool: "cancel_backup", Outcome: "ok", RunID: "run1"},
		{At: 2000, Tool: "get_health", Outcome: "ok"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %+v, want %+v", got, want)
	}
}

func TestMCPKeyEventsKeepOnlyTheNewestPerKey(t *testing.T) {
	r := newMCPRepo(t)
	addMCPKey(t, r, "laptop", "Laptop", true, 1000)
	addMCPKey(t, r, "desktop", "Desktop", true, 1000)
	recordEvent(t, r, "desktop", store.MCPKeyEvent{At: 1500, Tool: "get_health", Outcome: "ok"})

	for i := range store.MCPKeyEventsKept + 5 {
		recordEvent(t, r, "laptop", store.MCPKeyEvent{At: 2000 + int64(i), Tool: "start_backup", Outcome: "cooldown"})
	}
	got, err := r.MCPKeyEvents("laptop")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != store.MCPKeyEventsKept {
		t.Fatalf("%d events kept, want %d", len(got), store.MCPKeyEventsKept)
	}
	if got[0].At != 2000+int64(store.MCPKeyEventsKept+4) || got[len(got)-1].At != 2005 {
		t.Fatalf("kept %d..%d, want the newest %d", got[len(got)-1].At, got[0].At, store.MCPKeyEventsKept)
	}
	other, err := r.MCPKeyEvents("desktop")
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 1 {
		t.Fatalf("a busy key pushed another key's events out: %+v", other)
	}
}

// An assistant polling get_activity while a backup runs makes hundreds of
// reads, and those must not push the start, the cancel and the refusals of
// that backup out of the log.
func TestMCPKeyReadsDoNotCrowdOutWhatAKeyDid(t *testing.T) {
	r := newMCPRepo(t)
	addMCPKey(t, r, "laptop", "Laptop", true, 1000)
	recordEvent(t, r, "laptop", store.MCPKeyEvent{At: 2000, Tool: "start_backup", Outcome: "ok"})
	recordEvent(t, r, "laptop", store.MCPKeyEvent{At: 2001, Tool: "cancel_backup", Outcome: "ok", RunID: "run1"})
	recordEvent(t, r, "laptop", store.MCPKeyEvent{At: 2002, Tool: "get_status", Outcome: "failed"})
	polls := store.MCPKeyReadsKept + store.MCPKeyEventsKept
	for i := range polls {
		recordEvent(t, r, "laptop", store.MCPKeyEvent{At: 3000 + int64(i), Tool: "get_activity", Outcome: "ok", Routine: true})
	}

	got, err := r.MCPKeyEvents("laptop")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != store.MCPKeyReadsKept+3 {
		t.Fatalf("%d events kept, want the newest %d reads and the three others", len(got), store.MCPKeyReadsKept)
	}
	if got[0].At != 3000+int64(polls-1) || got[store.MCPKeyReadsKept-1].At != 3000+int64(polls-store.MCPKeyReadsKept) {
		t.Fatalf("the reads kept are not the newest: %d..%d", got[store.MCPKeyReadsKept-1].At, got[0].At)
	}
	want := []store.MCPKeyEvent{
		{At: 2002, Tool: "get_status", Outcome: "failed"},
		{At: 2001, Tool: "cancel_backup", Outcome: "ok", RunID: "run1"},
		{At: 2000, Tool: "start_backup", Outcome: "ok"},
	}
	if rest := got[store.MCPKeyReadsKept:]; !reflect.DeepEqual(rest, want) {
		t.Fatalf("the polls pushed out %+v, left %+v", want, rest)
	}
}

func TestMCPKeyEventsExpireWithAge(t *testing.T) {
	r := newMCPRepo(t)
	addMCPKey(t, r, "laptop", "Laptop", true, 1000)
	addMCPKey(t, r, "desktop", "Desktop", true, 1000)
	const start = 1_800_000_000
	recordEvent(t, r, "desktop", store.MCPKeyEvent{At: start, Tool: "get_health", Outcome: "ok"})
	recordEvent(t, r, "laptop", store.MCPKeyEvent{At: start + store.MCPKeyEventsMaxAge, Tool: "get_health", Outcome: "ok"})
	recordEvent(t, r, "laptop", store.MCPKeyEvent{At: start + store.MCPKeyEventsMaxAge + 1, Tool: "list_runs", Outcome: "ok"})

	old, err := r.MCPKeyEvents("desktop")
	if err != nil {
		t.Fatal(err)
	}
	if len(old) != 0 {
		t.Fatalf("an event older than the age limit survived: %+v", old)
	}
	events, refusals, err := r.MCPKeyEventTotals()
	if err != nil {
		t.Fatal(err)
	}
	if events != 2 || refusals != 0 {
		t.Fatalf("totals = %d events, %d refusals, want 2 and 0", events, refusals)
	}
}

func TestMCPKeyCallsCountToolCallsSinceAMoment(t *testing.T) {
	r := newMCPRepo(t)
	addMCPKey(t, r, "laptop", "Laptop", true, 1000)
	addMCPKey(t, r, "desktop", "Desktop", true, 1000)
	const midnight = 1_800_000_000 - 1_800_000_000%86400

	recordEvent(t, r, "laptop", store.MCPKeyEvent{At: midnight - 60, Tool: "get_health", Outcome: "ok"})
	for i := range 3 {
		recordEvent(t, r, "laptop", store.MCPKeyEvent{At: midnight + 60*int64(i), Tool: "get_status", Outcome: "ok"})
	}
	recordEvent(t, r, "laptop", store.MCPKeyEvent{At: midnight + 3600, Tool: "start_backup", Outcome: "cooldown"})
	recordEvent(t, r, "laptop", store.MCPKeyEvent{At: midnight + 3601, Outcome: "rate_limited"})
	recordEvent(t, r, "desktop", store.MCPKeyEvent{At: midnight + 7200, Tool: "list_items", Outcome: "ok"})

	got, err := r.MCPKeyCallsSince(midnight)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"laptop": 4, "desktop": 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("calls since midnight = %v, want %v", got, want)
	}

	_, refusals, err := r.MCPKeyEventTotals()
	if err != nil {
		t.Fatal(err)
	}
	if refusals != 2 {
		t.Fatalf("refusals = %d, want the cooldown and the rate limit", refusals)
	}
}

func TestMCPKeyCallsKeepCountingPastTheEventCap(t *testing.T) {
	r := newMCPRepo(t)
	addMCPKey(t, r, "laptop", "Laptop", true, 1000)
	const since = 1_800_000_000
	calls := store.MCPKeyEventsKept + 50
	for i := range calls {
		recordEvent(t, r, "laptop", store.MCPKeyEvent{At: since + int64(i), Tool: "get_status", Outcome: "ok"})
	}
	got, err := r.MCPKeyCallsSince(since)
	if err != nil {
		t.Fatal(err)
	}
	if got["laptop"] != calls {
		t.Fatalf("calls = %d, want %d", got["laptop"], calls)
	}
}

func TestRunsStartedByMCPKeyNamesOnlyThatKeysRuns(t *testing.T) {
	r := newMCPRepo(t)
	addMCPKey(t, r, "laptop", "Laptop", true, 1000)
	addMCPKey(t, r, "desktop", "Desktop", true, 1000)

	var mine []string
	for i := range 3 {
		id, err := r.StartRunWith("target", "backup", store.RunMeta{StartedVia: "mcp", StartedViaKey: "laptop"})
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if err := r.FinishRun(id, "cancelled", "", 0, "cancelled"); err != nil {
				t.Fatal(err)
			}
		}
		mine = append(mine, id)
	}
	if _, err := r.StartRunWith("target", "backup", store.RunMeta{StartedVia: "mcp", StartedViaKey: "desktop"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.StartRun("target", "backup"); err != nil {
		t.Fatal(err)
	}

	runs, err := r.RunsStartedByMCPKey("laptop", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("%d runs, want the limit of 2", len(runs))
	}
	for _, run := range runs {
		if run.StartedViaKey != "laptop" {
			t.Fatalf("a run of another origin came back: %+v", run)
		}
	}
	all, err := r.RunsStartedByMCPKey("laptop", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(mine) {
		t.Fatalf("%d runs, want %d", len(all), len(mine))
	}
}

func TestPurgeMCPKeyTakesItsActivityAlong(t *testing.T) {
	r := newMCPRepo(t)
	addMCPKey(t, r, "laptop", "Laptop", true, 1000)
	addMCPKey(t, r, "desktop", "Desktop", true, 1000)
	recordEvent(t, r, "laptop", store.MCPKeyEvent{At: 2000, Tool: "get_health", Outcome: "ok"})
	recordEvent(t, r, "desktop", store.MCPKeyEvent{At: 2000, Tool: "get_health", Outcome: "ok"})
	if err := r.RevokeMCPKey("laptop", "user", 3000); err != nil {
		t.Fatal(err)
	}
	if err := r.PurgeMCPKey("laptop"); err != nil {
		t.Fatal(err)
	}

	events, err := r.MCPKeyEvents("laptop")
	if err != nil {
		t.Fatal(err)
	}
	calls, err := r.MCPKeyCallsSince(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 || calls["laptop"] != 0 {
		t.Fatalf("a purged key left events %+v and %d calls behind", events, calls["laptop"])
	}
	if calls["desktop"] != 1 {
		t.Fatalf("the purge took another key's calls: %v", calls)
	}
	if _, err := r.GetMCPKey("laptop"); !errors.Is(err, store.ErrMCPKeyNotFound) {
		t.Fatalf("the key is still there: %v", err)
	}
}
