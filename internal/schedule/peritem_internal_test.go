package schedule

import (
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// recordingBackup returns a BackupFunc that records the names it is called with.
func recordingBackup() (BackupFunc, *[]string) {
	var mu sync.Mutex
	var got []string
	return func(name string) error {
		mu.Lock()
		got = append(got, name)
		mu.Unlock()
		return nil
	}, &got
}

// TestClassifyItemOverride covers the three outcomes of an item's override:
// follow the domain default, an own entry on its cadence, or not scheduled.
func TestClassifyItemOverride(t *testing.T) {
	if got := classifyItemOverride(""); !got.inDomainRun || got.ownEntry {
		t.Fatalf("empty override: expected inDomainRun, got %+v", got)
	}
	if got := classifyItemOverride("   "); !got.inDomainRun || got.ownEntry {
		t.Fatalf("blank override: expected inDomainRun, got %+v", got)
	}
	// An invalid override falls back to the domain run instead of leaving the
	// item unscheduled.
	if got := classifyItemOverride("nonsense cadence"); !got.inDomainRun || got.ownEntry {
		t.Fatalf("invalid override: expected inDomainRun fallback, got %+v", got)
	}
	// There is no per-item last-run gate, so everyN falls back as well.
	if got := classifyItemOverride("everyN 3 04:00"); !got.inDomainRun || got.ownEntry {
		t.Fatalf("everyN override: expected inDomainRun fallback, got %+v", got)
	}
	if got := classifyItemOverride("off"); got.inDomainRun || got.ownEntry {
		t.Fatalf("off override: expected neither, got %+v", got)
	}
	got := classifyItemOverride("daily 06:00")
	if !got.ownEntry || got.inDomainRun {
		t.Fatalf("concrete override: expected ownEntry, got %+v", got)
	}
	if want := "0 6 * * *"; got.Spec != want {
		t.Fatalf("concrete override spec: got %q, want %q (its OWN cadence, not the domain's)", got.Spec, want)
	}
}

// TestDomainRunTargetsFlagOff checks that with the feature off overrides are
// ignored and every included item stays in the domain run.
func TestDomainRunTargetsFlagOff(t *testing.T) {
	targets := []store.Target{
		{ContainerName: "web", IncludeInSchedule: true, ScheduleCadence: "daily 06:00"},
		{ContainerName: "db", IncludeInSchedule: true},
	}
	got := DomainRunTargets(targets, false)
	if len(got) != 2 {
		t.Fatalf("flag off: expected all %d targets (overrides ignored), got %d", len(targets), len(got))
	}

	backup, rec := recordingBackup()
	attempted, failed, _ := RunContainersJob(got, backup)
	if attempted != 2 || failed != 0 {
		t.Fatalf("flag off: expected attempted 2 failed 0, got %d/%d", attempted, failed)
	}
	if len(*rec) != 2 {
		t.Fatalf("flag off: expected both web+db backed up, got %v", *rec)
	}
}

// TestDomainRunTargetsFlagOn checks that with the feature on, an item with a
// valid override leaves the domain run for its own entry, an item without one
// stays, and an "off" override drops the item from scheduling.
func TestDomainRunTargetsFlagOn(t *testing.T) {
	targets := []store.Target{
		{ContainerName: "web", IncludeInSchedule: true, ScheduleCadence: "daily 06:00"}, // own entry
		{ContainerName: "db", IncludeInSchedule: true},                                  // domain default
		{ContainerName: "cache", IncludeInSchedule: true, ScheduleCadence: "off"},       // not scheduled
		{ContainerName: "bad", IncludeInSchedule: true, ScheduleCadence: "gibberish"},   // invalid -> domain default
	}
	got := DomainRunTargets(targets, true)

	backup, rec := recordingBackup()
	RunContainersJob(got, backup)

	want := map[string]bool{"db": true, "bad": true}
	if len(*rec) != len(want) {
		t.Fatalf("flag on domain run: expected %v, got %v", keys(want), *rec)
	}
	for _, n := range *rec {
		if !want[n] {
			t.Fatalf("flag on domain run backed up %q, which should be excluded (own entry or off): got %v", n, *rec)
		}
	}
}

// TestPerItemEntriesRegistered checks that each included item with an override
// gets its own cron entry while the feature is on, and that turning it off
// leaves only the domain entry.
func TestPerItemEntriesRegistered(t *testing.T) {
	targets := []store.Target{
		{ContainerName: "web", IncludeInSchedule: true, ScheduleCadence: "daily 06:00"},       // own entry
		{ContainerName: "api", IncludeInSchedule: true, ScheduleCadence: "weekly Sun 02:00"},  // own entry
		{ContainerName: "db", IncludeInSchedule: true},                                        // domain default
		{ContainerName: "off1", IncludeInSchedule: true, ScheduleCadence: "off"},              // no entry
		{ContainerName: "excluded", IncludeInSchedule: false, ScheduleCadence: "daily 07:00"}, // not included -> no entry
	}
	backup, _ := recordingBackup()
	sc := New(backup, func() ([]store.Target, error) { return targets, nil })

	off := store.Settings{ContainersEnabled: true, ContainersSchedule: "daily 03:00"}
	if err := sc.ReloadWithDueChecks(off, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks (off): %v", err)
	}
	if got := len(sc.entries); got != 1 {
		t.Fatalf("feature off: expected exactly 1 entry (containers domain), got %d", got)
	}

	// The domain entry plus web and api; off1 and excluded get none.
	on := off
	on.PerItemSchedules = true
	if err := sc.ReloadWithDueChecks(on, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks (on): %v", err)
	}
	if got := len(sc.entries); got != 3 {
		t.Fatalf("feature on: expected 3 entries (1 domain + 2 per-item), got %d", got)
	}

	if err := sc.ReloadWithDueChecks(off, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks (off again): %v", err)
	}
	if got := len(sc.entries); got != 1 {
		t.Fatalf("feature toggled off: expected 1 entry again, got %d", got)
	}
}

// TestPerItemEntryRunsOnlyItsItem fires the per-item and the domain entry and
// checks that each container is backed up once: web by its own entry, db by
// the domain run.
func TestPerItemEntryRunsOnlyItsItem(t *testing.T) {
	targets := []store.Target{
		{ContainerName: "web", IncludeInSchedule: true, ScheduleCadence: "daily 06:00"}, // own entry
		{ContainerName: "db", IncludeInSchedule: true},                                  // domain default
	}
	backup, rec := recordingBackup()
	sc := New(backup, func() ([]store.Target, error) { return targets, nil })

	on := store.Settings{ContainersEnabled: true, ContainersSchedule: "daily 03:00", PerItemSchedules: true}
	if err := sc.ReloadWithDueChecks(on, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}
	if len(sc.entries) != 2 {
		t.Fatalf("expected 2 entries (domain + web), got %d", len(sc.entries))
	}

	for _, e := range sc.entries {
		entry := sc.c.Entry(e.id)
		if entry.WrappedJob != nil {
			entry.WrappedJob.Run()
		}
	}
	counts := map[string]int{}
	for _, n := range *rec {
		counts[n]++
	}
	if counts["web"] != 1 || counts["db"] != 1 || len(counts) != 2 {
		t.Fatalf("expected web=1 db=1 exactly, got %v", counts)
	}
}

// TestDomainRunVMTargetsFlag checks the same filter for VMs.
func TestDomainRunVMTargetsFlag(t *testing.T) {
	vms := []store.VMTarget{
		{Name: "win", IncludeInSchedule: true, ScheduleCadence: "daily 06:00"}, // own entry
		{Name: "lin", IncludeInSchedule: true},                                 // domain default
	}
	if got := DomainRunVMTargets(vms, false); len(got) != 2 {
		t.Fatalf("flag off: expected both VMs, got %d", len(got))
	}
	got := DomainRunVMTargets(vms, true)
	if len(got) != 1 || got[0].Name != "lin" {
		t.Fatalf("flag on: expected only the non-overridden VM (lin), got %+v", got)
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
