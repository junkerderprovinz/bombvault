package api

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// listingFails is a pullRecorder whose listing of one repository fails.
type listingFails struct {
	pullRecorder
	repo string
	err  error
}

func (e *listingFails) Snapshots(ctx context.Context, repo string, m restic.Mode) ([]restic.Snapshot, error) {
	if repo == e.repo {
		return nil, e.err
	}
	return e.pullRecorder.Snapshots(ctx, repo, m)
}

// openFails is a pullRecorder whose source cannot be opened.
type openFails struct {
	pullRecorder
	err error
}

func (e *openFails) RepoOpensErr(_ context.Context, repo string, m restic.Mode) error {
	if repo == pullSrcLoc {
		return e.err
	}
	return e.pullRecorder.RepoOpensErr(context.Background(), repo, m)
}

func catchAlerts(t *testing.T, svc *Service) func() []string {
	t.Helper()
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		bodies = append(bodies, string(b))
	}))
	t.Cleanup(srv.Close)
	if err := svc.SetNotifyConfig(oldNotify(map[string]any{"on": "failure", "webhookEnabled": true, "webhookUrl": srv.URL})); err != nil {
		t.Fatal(err)
	}
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), bodies...)
	}
}

func pullFailures(t *testing.T, st *store.Repo) []store.Run {
	t.Helper()
	runs, err := st.ListRuns(50)
	if err != nil {
		t.Fatal(err)
	}
	var out []store.Run
	for _, r := range runs {
		if r.Kind == "pull" && r.Status == "failed" {
			out = append(out, r)
		}
	}
	return out
}

func TestAPullThatCannotListItsCopyIsNoPullOK(t *testing.T) {
	dest := pullDomain + "/pulled/"
	eng := &listingFails{pullRecorder: pullRecorder{snaps: map[string][]restic.Snapshot{pullSrcLoc: {{ID: "aaa"}, {ID: "bbb"}}}}, err: errors.New("503 Service Unavailable")}
	svc, st, ps := pullFixture(t, eng)
	eng.repo = dest + ps.ID

	n, err := svc.PullFromSource(context.Background(), ps)
	if err == nil {
		t.Fatalf("n=%d and no error from a pull that could not read its own copy", n)
	}
	got, _, _ := st.GetPullSource(ps.ID)
	if !got.LastPullOK.Valid || got.LastPullOK.Bool {
		t.Fatalf("lastPullOK = %+v, want a failure", got.LastPullOK)
	}
}

func TestAPullThatCannotListTheSourceIsNoPullOK(t *testing.T) {
	eng := &listingFails{repo: pullSrcLoc, err: errors.New("could not load snapshots")}
	svc, st, ps := pullFixture(t, eng)

	if _, err := svc.PullFromSource(context.Background(), ps); err == nil {
		t.Fatal("no error from a pull that could not read the source")
	}
	got, _, _ := st.GetPullSource(ps.ID)
	if !got.LastPullOK.Valid || got.LastPullOK.Bool {
		t.Fatalf("lastPullOK = %+v, want a failure", got.LastPullOK)
	}
}

func TestAFailedPullAlertsOnceAndLogsEveryAttempt(t *testing.T) {
	eng := &openFails{err: errors.New("dial tcp 192.168.1.9:8000: connect: connection refused")}
	svc, st, ps := pullFixture(t, eng)
	alerts := catchAlerts(t, svc)

	for range 3 {
		if _, err := svc.PullFromSource(context.Background(), ps); err == nil {
			t.Fatal("a dead source pulled fine")
		}
		ps, _, _ = st.GetPullSource(ps.ID)
	}

	got := alerts()
	if len(got) != 1 || !strings.Contains(got[0], "Pull from Tower failed") {
		t.Fatalf("alerts = %q, want one naming the source", got)
	}
	runs := pullFailures(t, st)
	if len(runs) != 3 {
		t.Fatalf("activity log has %d failed pulls, want one per attempt", len(runs))
	}
	if runs[0].Subject != "Tower" || runs[0].TargetID != "containers" || !strings.Contains(runs[0].Error, "could not reach") {
		t.Fatalf("run = %+v", runs[0])
	}
}

func TestAPullThatWorksAgainReArmsTheAlert(t *testing.T) {
	eng := &openFails{err: errors.New("dial tcp 192.168.1.9:8000: connect: connection refused")}
	svc, st, ps := pullFixture(t, eng)
	alerts := catchAlerts(t, svc)

	if _, err := svc.PullFromSource(context.Background(), ps); err == nil {
		t.Fatal("a dead source pulled fine")
	}
	eng.err = nil
	ps, _, _ = st.GetPullSource(ps.ID)
	if _, err := svc.PullFromSource(context.Background(), ps); err != nil {
		t.Fatal(err)
	}
	eng.err = errors.New("dial tcp 192.168.1.9:8000: connect: connection refused")
	ps, _, _ = st.GetPullSource(ps.ID)
	if _, err := svc.PullFromSource(context.Background(), ps); err == nil {
		t.Fatal("a dead source pulled fine")
	}
	if got := alerts(); len(got) != 2 {
		t.Fatalf("alerts = %q, want one per episode", got)
	}
}

func TestASourceThatIsNotThereYetIsNoAlarmBeforeItsFirstPull(t *testing.T) {
	eng := &openFails{err: errors.New("Fatal: repository does not exist: unable to open config file")}
	svc, st, ps := pullFixture(t, eng)
	alerts := catchAlerts(t, svc)

	if _, err := svc.PullFromSource(context.Background(), ps); err == nil {
		t.Fatal("no error for a source that is not there")
	}
	if got := alerts(); len(got) != 0 {
		t.Fatalf("alerts = %q on the normal setup order", got)
	}
	if runs := pullFailures(t, st); len(runs) != 0 {
		t.Fatalf("activity log = %+v on the normal setup order", runs)
	}

	// The same answer from a source that has been pulled means it is gone.
	ps.LastPullOK = sql.NullBool{Bool: true, Valid: true}
	if _, err := svc.PullFromSource(context.Background(), ps); err == nil {
		t.Fatal("no error for a source that is gone")
	}
	if got := alerts(); len(got) != 1 {
		t.Fatalf("alerts = %q for a source that was there and went", got)
	}
}

func TestAPullCutOffByAStopIsNoVerdict(t *testing.T) {
	eng := &openFails{err: context.Canceled}
	svc, st, ps := pullFixture(t, eng)
	alerts := catchAlerts(t, svc)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := svc.PullFromSource(ctx, ps); err == nil {
		t.Fatal("no error from a pull that was cut off")
	}
	got, _, _ := st.GetPullSource(ps.ID)
	if got.LastPullOK.Valid || got.LastPullAt != 0 {
		t.Fatalf("a stop left a verdict: %+v at %d", got.LastPullOK, got.LastPullAt)
	}
	if a := alerts(); len(a) != 0 {
		t.Fatalf("alerts = %q", a)
	}
}
