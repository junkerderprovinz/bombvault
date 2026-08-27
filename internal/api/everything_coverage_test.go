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
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
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

var _ = store.Settings{}
