package api_test

// Like the scheduler, "Backup Everything" honours the per-domain switch and
// skips a domain with nothing to do instead of paying for a Healthchecks ping,
// a prune and an off-site copy. The tests check what a user sees (which domains
// ran, which checks were pinged): a green "0 of 0 items succeeded" ping for a
// pass that touched nothing turns a red check green.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/notify"
)

// hcRecorder is a Healthchecks endpoint that records the path of every ping.
type hcRecorder struct {
	mu    sync.Mutex
	paths []string
	srv   *httptest.Server
}

func newHCRecorder(t *testing.T) *hcRecorder {
	t.Helper()
	r := &hcRecorder{}
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.paths = append(r.paths, req.URL.Path)
		r.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(r.srv.Close)
	return r
}

func (r *hcRecorder) pinged(prefix string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.paths {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

func (r *hcRecorder) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.paths...)
}

// On a host without an Unraid flash a flash step would fail the parent run on
// every pass.
func TestEverythingSkipsSwitchedOffDomain(t *testing.T) {
	log := &everythingOrderLog{}
	eng := &orderedEngine{fakeResticEngine: &fakeResticEngine{}, log: log}
	svc, st, _, _ := everythingTestService(t, eng)

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.FlashEnabled = false // the fixture switches all five on; take flash back off
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	sum, err := svc.BackupEverything(context.Background())
	if err != nil {
		t.Fatalf("BackupEverything: %v", err)
	}

	for _, e := range log.entries {
		if e == "flash" {
			t.Fatalf("flash is switched off and must not run, order log = %v", log.entries)
		}
	}
	for _, d := range sum.Domains {
		if d.Domain == "flash" {
			t.Fatalf("a switched-off domain must not appear in the pass result, got %+v", sum.Domains)
		}
	}
	// The domains that are on still run, and a skipped domain is no failure.
	var ranContainers bool
	for _, e := range log.entries {
		if e == "containers" {
			ranContainers = true
		}
	}
	if !ranContainers {
		t.Fatalf("containers is switched on and must still run, order log = %v", log.entries)
	}
	if sum.Status != "success" {
		t.Fatalf("Status = %q, want %q (a skipped domain is not a failure): %+v", sum.Status, "success", sum.Domains)
	}
}

// A switched-on domain with nothing eligible must not ping its check, as with
// the scheduler's DomainRunHasWork gate.
func TestEverythingIdleDomainSkipsPingAndTail(t *testing.T) {
	hc := newHCRecorder(t)
	log := &everythingOrderLog{}
	eng := &orderedEngine{fakeResticEngine: &fakeResticEngine{}, log: log}
	svc, _, _, _ := everythingTestService(t, eng)

	// One check per domain. The fixture has one container target and no VMs or
	// file sets, so only containers has work.
	if err := svc.SetNotifyConfig(notify.Config{
		On: "always",
		HealthchecksByDomain: map[string]string{
			"container": hc.srv.URL + "/container",
			"VM":        hc.srv.URL + "/vm",
			"files":     hc.srv.URL + "/files",
		},
	}); err != nil {
		t.Fatalf("SetNotifyConfig: %v", err)
	}

	sum, err := svc.BackupEverything(context.Background())
	if err != nil {
		t.Fatalf("BackupEverything: %v", err)
	}

	if hc.pinged("/vm") {
		t.Fatalf("no VM is eligible, so its check must not be pinged, pings = %v", hc.seen())
	}
	if hc.pinged("/files") {
		t.Fatalf("no file set is eligible, so its check must not be pinged, pings = %v", hc.seen())
	}
	if !hc.pinged("/container") {
		t.Fatalf("containers HAS work and must still ping its check, pings = %v", hc.seen())
	}

	// An idle domain is still reported, as a success.
	var sawVMs bool
	for _, d := range sum.Domains {
		if d.Domain != "vms" {
			continue
		}
		sawVMs = true
		if d.Attempted != 0 || d.Failed != 0 {
			t.Fatalf("idle vms domain = %+v, want Attempted=0 Failed=0", d)
		}
	}
	if !sawVMs {
		t.Fatalf("vms is switched on and must still be reported, got %+v", sum.Domains)
	}
	if sum.Status != "success" {
		t.Fatalf("Status = %q, want %q: %+v", sum.Status, "success", sum.Domains)
	}
}

// The parent run's breakdown reaches the run history, the weekly digest and the
// widget feed, so host paths in it are scrubbed as in the child runs.
func TestEverythingBreakdownIsScrubbed(t *testing.T) {
	log := &everythingOrderLog{}
	eng := &orderedEngine{
		fakeResticEngine: &fakeResticEngine{backupErr: errFakeBackupPath},
		log:              log,
	}
	svc, st, _, _ := everythingTestService(t, eng)

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	// Only containers has a seeded target to reach the failing engine.
	s.VMsEnabled, s.FlashEnabled, s.FilesEnabled, s.ConfigEnabled = false, false, false, false
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	sum, err := svc.BackupEverything(context.Background())
	if err != nil {
		t.Fatalf("BackupEverything: %v", err)
	}
	if sum.Status != "failed" {
		t.Fatalf("Status = %q, want %q: %+v", sum.Status, "failed", sum.Domains)
	}
	if strings.Contains(sum.Error, "/mnt/user/appdata/secretpath") {
		t.Fatalf("breakdown carries a raw host path, want it scrubbed: %q", sum.Error)
	}
	if !strings.Contains(sum.Error, "containers:") {
		t.Fatalf("breakdown must still name the failing domain, got %q", sum.Error)
	}
}

// errFakeBackupPath is a backup failure with an absolute host path, which
// scrubSecrets strips.
var errFakeBackupPath = &pathErr{}

type pathErr struct{}

func (*pathErr) Error() string {
	return "restic: unable to read /mnt/user/appdata/secretpath/config.json"
}
