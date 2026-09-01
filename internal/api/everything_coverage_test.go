package api_test

// Issue #177 (manilx): a server backed up entirely by the "Backup Everything"
// pass reported "Not scheduled" for every domain on the protection card, since
// each domain's own cadence was off.
//
// The visible half was the wrong label. The expensive half was invisible: the
// same reading feeds the overdue watchdog, where a period of 0 means "no
// expectation" and therefore switches the dead-man's switch off entirely. That
// configuration had no overdue alerting on any domain, and nothing said so.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestDomainStatusCoveredByEverything: every domain switched ON with no cadence
// of its own, and only the Everything pass scheduled. Each domain must come back
// with a real RPO window and must NOT read as unscheduled.
func TestDomainStatusCoveredByEverything(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)

	s := mustSettings(t, st)
	s.ContainersEnabled, s.VMsEnabled, s.FlashEnabled, s.ConfigEnabled, s.FilesEnabled = true, true, true, true, true
	// Exactly manilx' configuration: no per-domain schedule anywhere, one
	// whole-server pass at 05:00.
	s.ContainersSchedule, s.VMsSchedule, s.FlashSchedule, s.ConfigSchedule, s.FilesSchedule = "off", "off", "off", "off", "off"
	s.EverythingSchedule = "daily 05:00"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	statuses, err := svc.DomainStatus()
	if err != nil {
		t.Fatalf("DomainStatus: %v", err)
	}
	if len(statuses) == 0 {
		t.Fatal("no domains returned")
	}
	for _, d := range statuses {
		if d.Status == "off" {
			t.Errorf("%s: status = %q, want a real status — the Everything pass backs it up nightly", d.Domain, d.Status)
		}
		if d.PeriodSeconds != 86400 {
			t.Errorf("%s: PeriodSeconds = %d, want 86400 (the pass's daily cadence)", d.Domain, d.PeriodSeconds)
		}
		if d.CoveredBy != "daily 05:00" {
			t.Errorf("%s: CoveredBy = %q, want %q so the card can name what covers it", d.Domain, d.CoveredBy, "daily 05:00")
		}
	}
}

// TestDomainStatusOwnScheduleWins: a domain with a cadence of its own reports
// that cadence and leaves CoveredBy empty, so the card keeps saying what it
// always said. The window is the SHORTER of the two, because whichever fires
// more often is what bounds how stale a backup can get.
func TestDomainStatusOwnScheduleWins(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)

	s := mustSettings(t, st)
	s.ContainersEnabled, s.FilesEnabled = true, true
	s.ContainersSchedule = "daily 02:00" // more often than the pass
	s.FilesSchedule = "weekly Mon 02:00" // less often than the pass
	s.EverythingSchedule = "daily 05:00"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	statuses, err := svc.DomainStatus()
	if err != nil {
		t.Fatalf("DomainStatus: %v", err)
	}
	for _, d := range statuses {
		switch d.Domain {
		case "containers":
			if d.CoveredBy != "" {
				t.Errorf("containers has its own schedule, CoveredBy must stay empty, got %q", d.CoveredBy)
			}
			if d.PeriodSeconds != 86400 {
				t.Errorf("containers: PeriodSeconds = %d, want 86400", d.PeriodSeconds)
			}
		case "files":
			if d.CoveredBy != "" {
				t.Errorf("files has its own schedule, CoveredBy must stay empty, got %q", d.CoveredBy)
			}
			// Weekly on its own, but the daily pass also backs it up, so the
			// window is a day, not a week.
			if d.PeriodSeconds != 86400 {
				t.Errorf("files: PeriodSeconds = %d, want 86400 (the daily pass is the shorter window)", d.PeriodSeconds)
			}
		}
	}
}

// TestDomainStatusEverythingOffStaysOff: with no schedule anywhere the status is
// still "off". The fix must not invent coverage.
func TestDomainStatusEverythingOffStaysOff(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)

	s := mustSettings(t, st)
	s.ContainersEnabled = true
	s.ContainersSchedule = "off"
	s.EverythingSchedule = "off"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	statuses, err := svc.DomainStatus()
	if err != nil {
		t.Fatalf("DomainStatus: %v", err)
	}
	for _, d := range statuses {
		if d.Domain != "containers" {
			continue
		}
		if d.Status != "off" {
			t.Fatalf("containers: status = %q, want %q — nothing is scheduled", d.Status, "off")
		}
		if d.CoveredBy != "" {
			t.Fatalf("containers: CoveredBy = %q, want empty", d.CoveredBy)
		}
	}
}

// TestDomainStatusSwitchedOffDomainIgnoresEverything: the pass skips a
// switched-off domain, so the pass must not lend that domain a status either.
func TestDomainStatusSwitchedOffDomainIgnoresEverything(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)

	s := mustSettings(t, st)
	s.VMsEnabled = false
	s.VMsSchedule = "off"
	s.EverythingSchedule = "daily 05:00"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	statuses, err := svc.DomainStatus()
	if err != nil {
		t.Fatalf("DomainStatus: %v", err)
	}
	for _, d := range statuses {
		if d.Domain == "vms" && d.Status != "off" {
			t.Fatalf("vms is switched off, status = %q, want %q", d.Status, "off")
		}
	}
}

// TestScheduleNextRanksEverythingAgainstADomain pins how the pass is weighed
// against a domain's own cadence now that #187 retired the cadence-string
// approximation the "next backup" cell used to run in the browser.
//
// GET /api/status used to carry a separate `everythingSchedule` field for that
// cell, because CoveredBy cannot carry it: domainCoverage empties CoveredBy the
// moment a domain has a schedule of its own, which is exactly the comparison
// that matters. The cell now reads real fire times from GET /api/schedule/next
// instead, so the field is gone and this is the contract that replaced it.
//
// The configuration is the one that produced #186: a weekly domain cadence plus
// a nightly pass. Ranking the strings put Sunday first; ranking the scheduler's
// actual fire times must put the pass first on any day that is not a Sunday.
func TestScheduleNextRanksEverythingAgainstADomain(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	d := &fakeServiceDocker{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{})

	settings := store.Settings{
		ContainersEnabled:  true,
		ContainersSchedule: "weekly Sun 04:00",
		EverythingSchedule: "daily 05:00",
	}
	sched := schedule.New(func(string) error { return nil }, st.ListTargets)
	if err := sched.Reload(settings); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	sched.Start()
	defer sched.Stop()

	h := api.NewHandler(cfg, st, d, svc, sched, spike.DefaultProbes()).Router()

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/schedule/next", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Runs []struct {
			Job    string    `json:"job"`
			Domain string    `json:"domain"`
			Next   time.Time `json:"next"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}

	// Both entries have to be there at all: the cell filters on job=="backup",
	// and an absent pass is precisely the blindness #186 reported.
	var pass, containers bool
	for _, r := range got.Runs {
		if r.Job != "backup" {
			continue
		}
		switch r.Domain {
		case "everything":
			pass = true
		case "containers":
			containers = true
		}
	}
	if !pass || !containers {
		t.Fatalf("want a backup entry for both the pass and containers, got %+v", got.Runs)
	}

	// Soonest-first, and on any day but Sunday the nightly pass is soonest. Guard
	// the exception rather than skipping, so the test still asserts on a Sunday.
	first := got.Runs[0]
	if first.Next.Weekday() == time.Sunday && first.Domain == "containers" {
		return
	}
	if first.Job != "backup" || first.Domain != "everything" {
		t.Fatalf("weekly domain plus a nightly pass: soonest must be the pass, got %+v", got.Runs)
	}
}

// TestStatusEndpointDropsEverythingSchedule holds the removal down. The field
// existed for one consumer, that consumer is gone (#187), and a field nothing
// reads is one more shape a peer or a future page can be tempted to rank by.
func TestStatusEndpointDropsEverythingSchedule(t *testing.T) {
	h, st, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})

	s := mustSettings(t, st)
	s.ContainersEnabled = true
	s.ContainersSchedule = "weekly Sun 04:00"
	s.EverythingSchedule = "daily 05:00"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	w, m := doJSON(t, h, http.MethodGet, "/api/status", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if _, present := m["everythingSchedule"]; present {
		t.Errorf("everythingSchedule is retired, still served as %v", m["everythingSchedule"])
	}

	// The domains themselves are untouched: containers still reports its OWN
	// cadence and an empty CoveredBy, which is what made the separate field
	// necessary in the first place and is why the answer had to move rather than
	// be folded into this payload.
	domains, ok := m["domains"].([]any)
	if !ok || len(domains) == 0 {
		t.Fatalf("domains missing from the envelope: %v", m["domains"])
	}
	var seen bool
	for _, raw := range domains {
		d, ok := raw.(map[string]any)
		if !ok || d["domain"] != "containers" {
			continue
		}
		seen = true
		if d["coveredBy"] != "" {
			t.Errorf("containers has its own schedule, coveredBy must stay empty, got %v", d["coveredBy"])
		}
		if d["schedule"] != "weekly Sun 04:00" {
			t.Errorf("containers schedule = %v, want its own cadence", d["schedule"])
		}
	}
	if !seen {
		t.Fatal("containers domain not present in the status envelope")
	}
}
