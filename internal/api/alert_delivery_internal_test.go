package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/notify"
)

// pathRecorder is an endpoint that answers with status and remembers the
// paths it was asked for.
type pathRecorder struct {
	srv    *httptest.Server
	status atomic.Int32
	mu     sync.Mutex
	paths  []string
}

func newPathRecorder(t *testing.T) *pathRecorder {
	t.Helper()
	p := &pathRecorder{}
	p.status.Store(http.StatusOK)
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.paths = append(p.paths, r.URL.Path)
		p.mu.Unlock()
		w.WriteHeader(int(p.status.Load()))
	}))
	t.Cleanup(p.srv.Close)
	return p
}

func (p *pathRecorder) got() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.paths...)
}

// In summary mode the round's summary stands for the items' results only. A
// guest left frozen, a retention that failed and a budget crossed are none of
// them and still go out on the item's context.
func TestScheduledSummaryStillSendsTheAlertsOfAnItem(t *testing.T) {
	wh := newPathRecorder(t)
	s := unraidNotifyService(t, nil)
	if err := s.SetNotifyConfig(notify.Config{
		On: "failure", WebhookEnabled: true, WebhookURL: wh.srv.URL, WebhookFormat: "generic", ScheduledSummary: true,
	}); err != nil {
		t.Fatal(err)
	}
	item := notify.WithMessagesSuppressed(notify.WithHealthchecksSuppressed(context.Background()))

	s.notifyBackup(item, "container", "plex", "", false, backup.Summary{}, errors.New("disk full"))
	if n := len(wh.got()); n != 0 {
		t.Fatalf("the item's own result belongs in the summary, got %d messages", n)
	}
	s.notifyGuestStillFrozen(item, "win11", errors.New("thaw timed out"))
	s.notifyRetentionFailed(item, "containers", "container:plex", "repository is locked")
	s.applyGrowthBudget(item, "containers", "primary:containers", 1, "primary", 2<<30)
	if n := len(wh.got()); n != 3 {
		t.Fatalf("got %d alerts, want the frozen guest, the failed retention and the budget", n)
	}
}

// A retention failure is the domain's, so it pings the check of the backup that
// just ran rather than falling back to the global one.
func TestARetentionFailurePingsTheDomainsCheck(t *testing.T) {
	global, containers := newPathRecorder(t), newPathRecorder(t)
	s := unraidNotifyService(t, nil)
	if err := s.SetNotifyConfig(notify.Config{
		On: "failure", HealthchecksURL: global.srv.URL,
		HealthchecksByDomain: map[string]string{"container": containers.srv.URL},
	}); err != nil {
		t.Fatal(err)
	}

	s.notifyRetentionFailed(context.Background(), "containers", "container:web", "repository is locked")

	if got := containers.got(); len(got) != 1 || got[0] != "/fail" {
		t.Fatalf("containers check got %v, want one /fail", got)
	}
	if got := global.got(); len(got) != 0 {
		t.Fatalf("global check got %v, want nothing", got)
	}
}

// The budget alert fires on the crossing only, so a crossing nobody heard of
// is reported again on the next check.
func TestABudgetAlertThatReachedNobodyIsSentAgain(t *testing.T) {
	wh := newPathRecorder(t)
	wh.status.Store(http.StatusBadGateway)
	s := unraidNotifyService(t, nil)
	if err := s.SetNotifyConfig(notify.Config{On: "failure", WebhookEnabled: true, WebhookURL: wh.srv.URL, WebhookFormat: "generic"}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	s.applyGrowthBudget(ctx, "containers", "offsite:containers", 1, "off-site", 2<<30)
	wh.status.Store(http.StatusOK)
	s.applyGrowthBudget(ctx, "containers", "offsite:containers", 1, "off-site", 2<<30)
	if n := len(wh.got()); n != 2 {
		t.Fatalf("got %d attempts, want the failed one and its retry", n)
	}
	s.applyGrowthBudget(ctx, "containers", "offsite:containers", 1, "off-site", 2<<30)
	if n := len(wh.got()); n != 2 {
		t.Fatalf("got %d attempts, want no third once the alert went out", n)
	}
}
