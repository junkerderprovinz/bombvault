package schedule

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Containers and VMs answer the same question file sets already answered: for
// THIS item, with THESE settings, does anything actually back it up?
//
// The rules are not obvious and not per-domain-uniform-by-accident: the domain
// toggle, the item's include flag, the per-item override and the Backup
// Everything pass all reach the same item, and three of the four can silently
// mean "never". A coverage report that got this wrong would be worse than none,
// because it would tell an operator that something is protected when it is not.
//
// Every case below is asserted for BOTH containers and VMs, because the two
// carry separate flags on separate tables and a helper written for one of them
// is exactly where a copy-paste error lands.

func containerSettings() store.Settings {
	return store.Settings{
		ContainersEnabled:  true,
		VMsEnabled:         true,
		PerItemSchedules:   true,
		ContainersSchedule: "off",
		VMsSchedule:        "off",
		EverythingSchedule: "off",
	}
}

// bothKinds runs one case against a container and a VM with the same inputs, so
// the two helpers cannot drift.
func bothKinds(t *testing.T, s store.Settings, included bool, override string) (container, vm EffectiveSchedule) {
	t.Helper()
	container = EffectiveContainerSchedule(
		store.Target{ContainerName: "plex", IncludeInSchedule: included, ScheduleCadence: override}, s)
	vm = EffectiveVMSchedule(
		store.VMTarget{Name: "win11", IncludeInSchedule: included, ScheduleCadence: override}, s)
	return container, vm
}

func wantKind(t *testing.T, got EffectiveSchedule, want, what string) {
	t.Helper()
	if got.Kind != want {
		t.Fatalf("%s: got %q want %q", what, got.Kind, want)
	}
}

func TestItemEffectiveNoneWhenTheDomainIsOff(t *testing.T) {
	s := containerSettings()
	s.ContainersEnabled = false
	s.VMsEnabled = false
	s.ContainersSchedule = "daily 02:00"
	s.VMsSchedule = "daily 02:00"
	s.EverythingSchedule = "daily 05:00"

	c, v := bothKinds(t, s, true, "")
	wantKind(t, c, EffectiveNone, "container, domain off")
	wantKind(t, v, EffectiveNone, "vm, domain off")
}

// The branch whose label lies, same as for file sets: "include in schedule" off
// is not "skipped by the domain schedule", it is "never backed up by anything",
// because the domain run AND Backup Everything both skip it.
func TestItemEffectiveNoneWhenNotIncluded(t *testing.T) {
	s := containerSettings()
	s.ContainersSchedule = "daily 02:00"
	s.VMsSchedule = "daily 02:00"
	s.EverythingSchedule = "daily 05:00"

	c, v := bothKinds(t, s, false, "")
	wantKind(t, c, EffectiveNone, "container, not included")
	wantKind(t, v, EffectiveNone, "vm, not included")
}

// A literal "off" override takes the item out of everything, even while the
// domain schedule and Backup Everything are both running.
func TestItemEffectiveNoneWhenTheOverrideIsOff(t *testing.T) {
	s := containerSettings()
	s.ContainersSchedule = "daily 02:00"
	s.VMsSchedule = "daily 02:00"
	s.EverythingSchedule = "daily 05:00"

	c, v := bothKinds(t, s, true, "off")
	wantKind(t, c, EffectiveNone, "container, override off")
	wantKind(t, v, EffectiveNone, "vm, override off")
}

func TestItemEffectiveOwnEntry(t *testing.T) {
	s := containerSettings()
	s.ContainersSchedule = "daily 02:00"
	s.VMsSchedule = "daily 02:00"

	c, v := bothKinds(t, s, true, "weekly mon 03:00")
	wantKind(t, c, EffectiveOwn, "container, own cadence")
	wantKind(t, v, EffectiveOwn, "vm, own cadence")
	// The cadence the USER typed, not the compiled cron expression.
	if c.Spec != "weekly mon 03:00" || v.Spec != "weekly mon 03:00" {
		t.Fatalf("the override must be reported as typed, got %q / %q", c.Spec, v.Spec)
	}
}

func TestItemEffectiveDomainRun(t *testing.T) {
	s := containerSettings()
	s.ContainersSchedule = "daily 02:00"
	s.VMsSchedule = "daily 04:00"

	c, v := bothKinds(t, s, true, "")
	wantKind(t, c, EffectiveDomain, "container, domain run")
	wantKind(t, v, EffectiveDomain, "vm, domain run")
	if c.Spec != "daily 02:00" || v.Spec != "daily 04:00" {
		t.Fatalf("each domain must report ITS OWN schedule, got %q / %q", c.Spec, v.Spec)
	}
}

// The shape that is easy to miss: the domain schedule is off, so the item is
// covered only by the Backup Everything pass. Reporting that as "none" would
// tell an operator their backups are not running when they are.
func TestItemEffectiveEverythingOnly(t *testing.T) {
	s := containerSettings()
	s.EverythingSchedule = "daily 05:00"

	c, v := bothKinds(t, s, true, "")
	wantKind(t, c, EffectiveEverything, "container, everything only")
	wantKind(t, v, EffectiveEverything, "vm, everything only")
}

func TestItemEffectiveBoth(t *testing.T) {
	s := containerSettings()
	s.ContainersSchedule = "daily 02:00"
	s.VMsSchedule = "daily 02:00"
	s.EverythingSchedule = "daily 05:00"

	c, v := bothKinds(t, s, true, "")
	wantKind(t, c, EffectiveBoth, "container, both")
	wantKind(t, v, EffectiveBoth, "vm, both")
	if c.AlsoSpec != "daily 05:00" {
		t.Fatalf("the second cadence must be named, got %q", c.AlsoSpec)
	}
}

// Per-item overrides are only consulted when the setting is on. With it off, an
// item carrying a stale override still follows its domain, and a report that
// honoured the override anyway would describe a schedule nobody runs.
func TestItemOverrideIgnoredWhenPerItemSchedulesIsOff(t *testing.T) {
	s := containerSettings()
	s.PerItemSchedules = false
	s.ContainersSchedule = "daily 02:00"
	s.VMsSchedule = "daily 02:00"

	c, v := bothKinds(t, s, true, "off")
	wantKind(t, c, EffectiveDomain, "container, override ignored")
	wantKind(t, v, EffectiveDomain, "vm, override ignored")
}
