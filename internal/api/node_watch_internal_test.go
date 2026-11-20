package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// fakeNode answers home's calls as a node would, from what the test says the
// node holds.
type fakeNode struct {
	mu      sync.Mutex
	away    bool
	busy    bool
	runs    []nodeRunReport
	open    AnomalyOpenCounts
	started []string
}

func (f *fakeNode) call(_ context.Context, _, action, path string, in, out any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.away {
		return errNodeAway
	}
	switch {
	case path == nodeRunPath && action == keydeposit.ProofNodeRun:
		if f.busy {
			return errNodeRunBusy
		}
		f.started = append(f.started, in.(nodeRunCall).Domain)
		*out.(*nodeRunAnswer) = nodeRunAnswer{OK: true, Started: 1}
	case path == nodeAPIPath && action == keydeposit.ProofNodeAPI:
		var body any
		switch in.(nodeAPIRequest).Path {
		case "/api/runs":
			body = map[string]any{"ok": true, "runs": f.runs}
		case "/api/anomalies/summary":
			body = map[string]any{"ok": true, "summary": AnomalySummary{Open: f.open}}
		default:
			return errNodeRoute
		}
		raw, _ := json.Marshal(body)
		var zipped bytes.Buffer
		zw := gzip.NewWriter(&zipped)
		_, _ = zw.Write(raw)
		_ = zw.Close()
		*out.(*nodeAPIAnswer) = nodeAPIAnswer{OK: true, Status: 200, Body: zipped.Bytes()}
	default:
		return errors.New("a call the watch does not make")
	}
	return nil
}

func (f *fakeNode) set(change func(f *fakeNode)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	change(f)
}

func (f *fakeNode) starts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.started)
}

// watchUnderTest is a nodeWatch over one adopted node, with what it sent and
// pruned.
type watchUnderTest struct {
	w      *nodeWatch
	st     *store.Repo
	node   *fakeNode
	alerts []string
	pruned int
}

// inWeek is a time in the week of Wednesday 18 November 2026, on the clock the
// cadences read.
func inWeek(day, hour, minute int) time.Time {
	return time.Date(2026, time.November, day, hour, minute, 0, 0, time.Local)
}

func newWatchUnderTest(t *testing.T, n store.Node) *watchUnderTest {
	t.Helper()
	wt := &watchUnderTest{st: newTestStore(t), node: &fakeNode{}}
	if n.ID == "" {
		n.ID = "aa"
	}
	n.Name, n.AppKeyEnc, n.RelayMode, n.CreatedAt = "vps", []byte{1}, "project", inWeek(16, 12, 0).Unix()
	if n.AdoptedAt == 0 {
		n.AdoptedAt = inWeek(16, 12, 0).Unix()
	}
	if err := wt.st.AddNode(n); err != nil {
		t.Fatal(err)
	}
	wt.w = newNodeWatch(wt.st, wt.node.call,
		func(_ context.Context, title, message string) { wt.alerts = append(wt.alerts, title+"\n"+message) },
		func(context.Context, store.Node) error { wt.pruned++; return nil })
	return wt
}

func (wt *watchUnderTest) tick(t *testing.T, now time.Time) {
	t.Helper()
	if err := wt.w.Tick(context.Background(), now); err != nil {
		t.Fatal(err)
	}
}

func (wt *watchUnderTest) get(t *testing.T) store.Node {
	t.Helper()
	n, err := wt.st.GetNode("aa")
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func finished(at time.Time) *int64 {
	u := at.Unix()
	return &u
}

func TestHomeStartsANodesSectionOncePerFireAndOnlyOnceItKnowsWhereBackupsGo(t *testing.T) {
	wt := newWatchUnderTest(t, store.Node{ContainersCadence: "daily 03:00", FilesCadence: "off"})
	wt.tick(t, inWeek(18, 3, 5))
	if got := wt.node.starts(); len(got) != 0 {
		t.Fatalf("a node with nowhere to back up to was started: %v", got)
	}
	if err := wt.st.SetNodePlaced("aa", nodePlaceReceiver, "https://backup.example.org"); err != nil {
		t.Fatal(err)
	}
	wt.tick(t, inWeek(18, 3, 6))
	wt.tick(t, inWeek(18, 3, 30))
	if got := wt.node.starts(); !slices.Equal(got, []string{"containers"}) {
		t.Fatalf("starts = %v, want the containers once", got)
	}
	if n := wt.get(t); n.StartedAt("containers") != inWeek(18, 3, 6).Unix() || n.StartedAt("files") != 0 {
		t.Errorf("started = %d and %d", n.StartedAt("containers"), n.StartedAt("files"))
	}
	if next, ok := wt.w.Next(inWeek(18, 3, 30)); !ok || !next.Equal(inWeek(19, 3, 0)) {
		t.Errorf("next = %v %v, want tomorrow at three", next, ok)
	}
	wt.tick(t, inWeek(19, 3, 1))
	if got := wt.node.starts(); len(got) != 2 {
		t.Errorf("starts = %v, want the next fire too", got)
	}
}

func TestAStartTheNodeMissedWhileAwayIsDueUntilItCanBeMade(t *testing.T) {
	wt := newWatchUnderTest(t, store.Node{FilesCadence: "daily 03:00", Placement: nodePlaceReceiver})
	wt.node.set(func(f *fakeNode) { f.away = true })
	wt.tick(t, inWeek(18, 3, 5))
	wt.tick(t, inWeek(18, 8, 0))
	if got := wt.node.starts(); len(got) != 0 {
		t.Fatalf("starts while away = %v", got)
	}
	wt.node.set(func(f *fakeNode) { f.away = false })
	wt.tick(t, inWeek(18, 9, 0))
	wt.tick(t, inWeek(18, 9, 1))
	if got := wt.node.starts(); !slices.Equal(got, []string{"files"}) {
		t.Errorf("starts once back = %v, want the missed one once", got)
	}
}

func TestABusyNodeIsAskedAgainOnlyAfterAWhile(t *testing.T) {
	wt := newWatchUnderTest(t, store.Node{ContainersCadence: "daily 03:00", Placement: nodePlaceReceiver})
	wt.node.set(func(f *fakeNode) { f.busy = true })
	wt.tick(t, inWeek(18, 3, 0))
	wt.node.set(func(f *fakeNode) { f.busy = false })
	wt.tick(t, inWeek(18, 3, 1))
	if got := wt.node.starts(); len(got) != 0 {
		t.Fatalf("a busy node was asked again a minute later: %v", got)
	}
	wt.tick(t, inWeek(18, 3, 0).Add(nodePollEvery))
	if got := wt.node.starts(); !slices.Equal(got, []string{"containers"}) {
		t.Errorf("starts = %v, want the containers once the node had time", got)
	}
}

func TestNoNodeBackupStartsWhileHomePrunesItsRepositories(t *testing.T) {
	wt := newWatchUnderTest(t, store.Node{ContainersCadence: "daily 03:00", Placement: nodePlaceReceiver})
	sunday := inWeek(22, 3, 2)
	wt.node.set(func(f *fakeNode) {
		f.runs = []nodeRunReport{{ID: "r1", Kind: "backup", Domain: "container", Target: "web", Status: "running", StartedAt: inWeek(22, 2, 50).Unix()}}
	})
	wt.tick(t, sunday)
	if wt.pruned != 0 || len(wt.node.starts()) != 0 {
		t.Fatalf("with a backup still running on the node: pruned %d, started %v", wt.pruned, wt.node.starts())
	}
	wt.node.set(func(f *fakeNode) { f.runs[0].Status, f.runs[0].FinishedAt = "success", finished(inWeek(22, 3, 4)) })
	wt.tick(t, sunday.Add(nodePollEvery))
	if wt.pruned != 1 || wt.get(t).PrunedWindow != inWeek(22, 3, 0).Unix() {
		t.Fatalf("pruned %d times in window %d", wt.pruned, wt.get(t).PrunedWindow)
	}
	if got := wt.node.starts(); !slices.Equal(got, []string{"containers"}) {
		t.Errorf("starts after the prune = %v, want the one it held", got)
	}
	wt.tick(t, sunday.Add(2*nodePollEvery))
	if wt.pruned != 1 {
		t.Errorf("pruned %d times in one window", wt.pruned)
	}
}

func TestAFailedNodeRunAlertsOncePerEpisodeWithTheNodesName(t *testing.T) {
	wt := newWatchUnderTest(t, store.Node{})
	run := func(id, status, failure string, minute int) nodeRunReport {
		return nodeRunReport{ID: id, Kind: "backup", Domain: "container", Target: "web", Status: status, Error: failure,
			StartedAt: inWeek(18, 3, minute).Unix(), FinishedAt: finished(inWeek(18, 3, minute+1))}
	}
	now := inWeek(18, 4, 0)
	step := func(runs ...nodeRunReport) {
		wt.node.set(func(f *fakeNode) { f.runs = append(runs, f.runs...) })
		wt.tick(t, now)
		now = now.Add(nodePollEvery)
	}
	step(run("r1", "failed", "disk full", 0))
	if len(wt.alerts) != 1 || !strings.Contains(wt.alerts[0], "vps") || !strings.Contains(wt.alerts[0], "\nvps: ") || !strings.Contains(wt.alerts[0], "disk full") {
		t.Fatalf("alerts = %q, want one led by the node's name", wt.alerts)
	}
	step(run("r2", "failed", "disk full", 10))
	if len(wt.alerts) != 1 {
		t.Fatalf("a second failure in the same episode alerted again: %q", wt.alerts)
	}
	step(run("r3", "success", "", 20))
	if open, _ := wt.st.NodeEpisodes("aa"); len(open) != 0 {
		t.Fatalf("a success left %v open", open)
	}
	step(run("r4", "failed", "repository locked", 30))
	if len(wt.alerts) != 2 || !strings.Contains(wt.alerts[1], "repository locked") {
		t.Errorf("alerts = %q, want a new episode", wt.alerts)
	}
	if runs, _ := wt.st.NodeRuns("aa"); len(runs) != 4 || runs[0].Domain != "containers" {
		t.Errorf("home kept %+v", runs)
	}
}

func TestANodeAwayForHoursAlertsOnceAndClearsWhenItIsBack(t *testing.T) {
	wt := newWatchUnderTest(t, store.Node{SeenAt: inWeek(18, 3, 0).Unix()})
	wt.node.set(func(f *fakeNode) { f.away = true })
	wt.tick(t, inWeek(18, 4, 0))
	if len(wt.alerts) != 0 {
		t.Fatalf("an hour away alerted: %q", wt.alerts)
	}
	wt.tick(t, inWeek(18, 3, 0).Add(nodeAwayAlertAfter))
	wt.tick(t, inWeek(18, 3, 0).Add(nodeAwayAlertAfter+nodePollEvery))
	if len(wt.alerts) != 1 || !strings.Contains(wt.alerts[0], "\nvps: ") {
		t.Fatalf("alerts = %q, want one for the outage", wt.alerts)
	}
	wt.node.set(func(f *fakeNode) { f.away = false; f.open = AnomalyOpenCounts{Critical: 1, Warning: 2, Info: 4} })
	back := inWeek(18, 3, 0).Add(nodeAwayAlertAfter + 2*nodePollEvery)
	wt.tick(t, back)
	if open, _ := wt.st.NodeEpisodes("aa"); len(open) != 0 {
		t.Errorf("the outage is still open once back: %v", open)
	}
	if n := wt.get(t); n.SeenAt != back.Unix() {
		t.Errorf("seen at %d, want %d", n.SeenAt, back.Unix())
	}
	if got := wt.w.anomalies("aa"); got != 3 {
		t.Errorf("anomalies = %d, want the critical and warning ones", got)
	}
}

func TestTheEpisodesOfANodeFollowWhatHomeLearnedOfIt(t *testing.T) {
	now := inWeek(18, 12, 0).Unix()
	hours := func(h int64) int64 { return now - h*3600 }
	ran := func(id, domain, target, status string, started int64) store.NodeRun {
		return store.NodeRun{ID: id, Domain: domain, Target: target, Status: status, StartedAt: started, FinishedAt: started + 60}
	}
	webFailed := nodeFailedEpisode("containers", "web")
	cases := []struct {
		name    string
		open    map[string]int64
		sighted nodeSighting
		opened  []string
		closed  []string
	}{
		{name: "a node in reach with nothing failed", sighted: nodeSighting{now: now, seenAt: now, reached: true}},
		{name: "a node away for a short while", sighted: nodeSighting{now: now, seenAt: hours(1)}},
		{name: "a node away for hours", sighted: nodeSighting{now: now, seenAt: hours(7)}, opened: []string{nodeAwayEpisode}},
		{name: "a node still away", open: map[string]int64{nodeAwayEpisode: hours(1)}, sighted: nodeSighting{now: now, seenAt: hours(8)}},
		{name: "a node that is back", open: map[string]int64{nodeAwayEpisode: hours(1)}, sighted: nodeSighting{now: now, seenAt: now, reached: true}, closed: []string{nodeAwayEpisode}},
		{
			name:    "a new failure",
			sighted: nodeSighting{now: now, reached: true, seenAt: now, runs: []store.NodeRun{ran("r1", "containers", "web", "failed", hours(1))}},
			opened:  []string{webFailed},
		},
		{
			name:    "a failure in an open episode",
			open:    map[string]int64{webFailed: hours(2)},
			sighted: nodeSighting{now: now, reached: true, seenAt: now, runs: []store.NodeRun{ran("r2", "containers", "web", "failed", hours(1))}},
		},
		{
			name:    "a success after the failures",
			open:    map[string]int64{webFailed: hours(2)},
			sighted: nodeSighting{now: now, reached: true, seenAt: now, runs: []store.NodeRun{ran("r3", "containers", "web", "success", hours(1)), ran("r2", "containers", "web", "failed", hours(2))}},
			closed:  []string{webFailed},
		},
		{
			name: "a run under way after a failure",
			sighted: nodeSighting{now: now, reached: true, seenAt: now, runs: []store.NodeRun{
				{ID: "r4", Domain: "containers", Target: "web", Status: "running", StartedAt: hours(1)},
				ran("r3", "containers", "web", "failed", hours(2)),
			}},
			opened: []string{webFailed},
		},
		{
			name: "failures of two entries",
			sighted: nodeSighting{now: now, reached: true, seenAt: now, runs: []store.NodeRun{
				ran("r5", "files", "photos", "failed", hours(1)),
				ran("r6", "containers", "web", "failed", hours(2)),
				ran("r7", "containers", "db", "success", hours(3)),
			}},
			opened: []string{webFailed, nodeFailedEpisode("files", "photos")},
		},
		{
			name:    "a cancelled run",
			sighted: nodeSighting{now: now, reached: true, seenAt: now, runs: []store.NodeRun{ran("r8", "containers", "web", "cancelled", hours(1))}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opened, closed := nodeEpisodes(c.open, c.sighted)
			var keys []string
			for _, ep := range opened {
				keys = append(keys, ep.key)
			}
			if !slices.Equal(keys, c.opened) || !slices.Equal(closed, c.closed) {
				t.Errorf("opened %v and closed %v, want %v and %v", keys, closed, c.opened, c.closed)
			}
		})
	}
}
