package api_test

// Regression tests for the two guards "Backup Everything" was missing against
// its own sibling, the real scheduler: the operator's per-domain on/off switch,
// and the has-work check that keeps a domain with nothing to do from paying for
// an aggregate Healthchecks ping, a prune and an off-site replication.
//
// Both are asserted through what a user can actually observe — which domains
// ran, and which Healthchecks checks were pinged — rather than through internal
// call counts, because the ping IS the damage in the second case: a green
// "0 of 0 items succeeded" at the dead-man's switch for a pass that touched
// nothing turns a check that had gone red back to green.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// hcRecorder is a Healthchecks endpoint that records the path of every ping it
// receives, so a test can assert which DOMAIN's check was pinged (each domain
// gets its own path below) and, just as importantly, which was not.
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

// TestEverythingSkipsSwitchedOffDomain: a domain the operator switched off is
// not part of the pass. Backup Everything used to call all five domain steps
// unconditionally, which had a consequence in both directions — on a host
// without an Unraid flash the flash step failed on every pass and failed the
// PARENT run with it, i.e. the one signal the whole feature exists to produce;
// and with the flash domain off on Unraid the step would create a repo the
// operator never asked for.
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
	// The domains that ARE on still run, and the pass is still a success: a
	// skipped domain is the operator's own configuration, not a failure.
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

// TestEverythingIdleDomainSkipsPingAndTail: a domain that IS switched on but has
// nothing eligible this pass must not ping its Healthchecks check.
//
// The real scheduler gates its whole domain closure on exactly this
// (schedule.DomainRunHasWork, whose comment reads "no loop, no ping, and above
// all no prune and no off-site copy"); the Everything pass reproduced the loop
// and left the gate behind. On a box with no VMs and no file sets — an ordinary
// state, not an exotic one — every pass therefore pinged a green "0 of 0 items
// succeeded" at those domains' checks.
func TestEverythingIdleDomainSkipsPingAndTail(t *testing.T) {
	hc := newHCRecorder(t)
	log := &everythingOrderLog{}
	eng := &orderedEngine{fakeResticEngine: &fakeResticEngine{}, log: log}
	svc, _, _, _ := everythingTestService(t, eng)

	// Per-domain checks so a ping identifies WHICH domain pinged. The fixture
	// registers one container target and no VM targets or file sets at all, so
	// containers has work and vms/files do not.
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
		t.Fatalf("no VM is eligible — its check must not be pinged, pings = %v", hc.seen())
	}
	if hc.pinged("/files") {
		t.Fatalf("no file set is eligible — its check must not be pinged, pings = %v", hc.seen())
	}
	if !hc.pinged("/container") {
		t.Fatalf("containers HAS work and must still ping its check, pings = %v", hc.seen())
	}

	// An idle domain is still reported, and still reported as fine: nothing was
	// eligible is a benign no-op (design spec, decision 3), not a failure.
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

// TestEverythingBreakdownIsScrubbed: the parent run's breakdown goes out to the
// run history, the weekly digest (Discord/Matrix/SMTP) and the token-gated
// widget feed. It used to bypass truncateRunErr — "the one function that writes
// runs.error" per its own doc — so absolute host paths reached all three raw,
// while the CHILD run of the very same failure got the scrubbed text.
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
	// Containers alone: it is the domain with a seeded target, so it is the one
	// that reaches the failing engine.
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

// errFakeBackupPath is a backup failure whose message embeds an absolute host
// path, the exact shape scrubSecrets exists to strip.
var errFakeBackupPath = &pathErr{}

type pathErr struct{}

func (*pathErr) Error() string {
	return "restic: unable to read /mnt/user/appdata/secretpath/config.json"
}

var _ = store.Settings{}
