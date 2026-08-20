package schedule

import (
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// recordingBackup is a BackupFunc that records the names it was called with, so a
// test can assert exactly WHICH items a run backed up.
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

// TestClassifyItemOverride pins the per-item due-selection seam (#121): the pure
// classification of an override string into "follow the domain default", "own
// entry on this cadence", or "off (not scheduled)".
func TestClassifyItemOverride(t *testing.T) {
	// Empty override → follow the domain default (the unchanged case).
	if got := classifyItemOverride(""); !got.inDomainRun || got.ownEntry {
		t.Fatalf("empty override: expected inDomainRun, got %+v", got)
	}
	// Whitespace-only is treated as empty.
	if got := classifyItemOverride("   "); !got.inDomainRun || got.ownEntry {
		t.Fatalf("blank override: expected inDomainRun, got %+v", got)
	}
	// Invalid override → fall back to the domain default (never silently unscheduled).
	if got := classifyItemOverride("nonsense cadence"); !got.inDomainRun || got.ownEntry {
		t.Fatalf("invalid override: expected inDomainRun fallback, got %+v", got)
	}
	// everyN is unsupported per-item (no per-item last-run gate) → domain default.
	if got := classifyItemOverride("everyN 3 04:00"); !got.inDomainRun || got.ownEntry {
		t.Fatalf("everyN override: expected inDomainRun fallback, got %+v", got)
	}
	// "off" → not scheduled at all (neither its own entry nor the domain run).
	if got := classifyItemOverride("off"); got.inDomainRun || got.ownEntry {
		t.Fatalf("off override: expected neither, got %+v", got)
	}
	// A concrete cadence → its own entry, with the parsed cron spec. "daily 06:00"
	// is "minute hour * * *" = "0 6 * * *".
	got := classifyItemOverride("daily 06:00")
	if !got.ownEntry || got.inDomainRun {
		t.Fatalf("concrete override: expected ownEntry, got %+v", got)
	}
	if want := "0 6 * * *"; got.Spec != want {
		t.Fatalf("concrete override spec: got %q, want %q (its OWN cadence, not the domain's)", got.Spec, want)
	}
}

// TestDomainRunTargetsFlagOff proves that with the feature OFF the domain run is
// byte-for-byte unchanged: overrides are ignored and the SAME slice is returned.
func TestDomainRunTargetsFlagOff(t *testing.T) {
	targets := []store.Target{
		{ContainerName: "web", IncludeInSchedule: true, ScheduleCadence: "daily 06:00"},
		{ContainerName: "db", IncludeInSchedule: true},
	}
	got := DomainRunTargets(targets, false)
	if len(got) != 2 {
		t.Fatalf("flag off: expected all %d targets (overrides ignored), got %d", len(targets), len(got))
	}

	// The domain run backs up EVERY included item, override or not — exactly as today.
	backup, rec := recordingBackup()
	attempted, failed, _ := RunContainersJob(got, backup)
	if attempted != 2 || failed != 0 {
		t.Fatalf("flag off: expected attempted 2 failed 0, got %d/%d", attempted, failed)
	}
	if len(*rec) != 2 {
		t.Fatalf("flag off: expected both web+db backed up, got %v", *rec)
	}
}

// TestDomainRunTargetsFlagOn proves that with the feature ON an item with a valid
// override is filtered OUT of the domain run (it fires on its own entry instead),
// an item without an override stays in the domain run (domain cadence), and an
// "off" override drops the item from all scheduling.
func TestDomainRunTargetsFlagOn(t *testing.T) {
	targets := []store.Target{
		{ContainerName: "web", IncludeInSchedule: true, ScheduleCadence: "daily 06:00"}, // own entry
		{ContainerName: "db", IncludeInSchedule: true},                                  // domain default
		{ContainerName: "cache", IncludeInSchedule: true, ScheduleCadence: "off"},       // not scheduled
		{ContainerName: "bad", IncludeInSchedule: true, ScheduleCadence: "gibberish"},   // invalid → domain default
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

// TestPerItemEntriesRegistered verifies the scheduler registers a dedicated cron
// entry per overridden+included item only when the feature is on, and that toggling
// it off removes those entries — with the OFF case registering exactly the single
// domain entry (byte-for-byte as before).
func TestPerItemEntriesRegistered(t *testing.T) {
	targets := []store.Target{
		{ContainerName: "web", IncludeInSchedule: true, ScheduleCadence: "daily 06:00"},       // own entry
		{ContainerName: "api", IncludeInSchedule: true, ScheduleCadence: "weekly Sun 02:00"},  // own entry
		{ContainerName: "db", IncludeInSchedule: true},                                        // domain default
		{ContainerName: "off1", IncludeInSchedule: true, ScheduleCadence: "off"},              // no entry
		{ContainerName: "excluded", IncludeInSchedule: false, ScheduleCadence: "daily 07:00"}, // not included → no entry
	}
	backup, _ := recordingBackup()
	sc := New(backup, func() ([]store.Target, error) { return targets, nil })

	// Feature OFF: exactly one entry (the containers domain job). Overrides ignored.
	off := store.Settings{ContainersEnabled: true, ContainersSchedule: "daily 03:00"}
	if err := sc.ReloadWithDueChecks(off, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks (off): %v", err)
	}
	if got := len(sc.entries); got != 1 {
		t.Fatalf("feature off: expected exactly 1 entry (containers domain), got %d", got)
	}

	// Feature ON: the domain entry PLUS one per-item entry for each of the two
	// valid, included overrides (web, api) = 3 entries. off1 (off) and excluded
	// (not in schedule) get none.
	on := off
	on.PerItemSchedules = true
	if err := sc.ReloadWithDueChecks(on, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks (on): %v", err)
	}
	if got := len(sc.entries); got != 3 {
		t.Fatalf("feature on: expected 3 entries (1 domain + 2 per-item), got %d", got)
	}

	// Toggling back off removes the per-item entries.
	if err := sc.ReloadWithDueChecks(off, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks (off again): %v", err)
	}
	if got := len(sc.entries); got != 1 {
		t.Fatalf("feature toggled off: expected 1 entry again, got %d", got)
	}
}

// TestPerItemEntryRunsOnlyItsItem drives a registered per-item entry's job and the
// domain entry's job directly (via their wrapped cron jobs), proving the per-item
// entry backs up ONLY its own overridden container and the domain job backs up ONLY
// the domain-default containers — no item is backed up twice.
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

	// Fire every registered entry once through its wrapped cron job (what a real
	// fire runs). The union must be exactly {web, db}, each once — web from its own
	// entry, db from the domain entry, and web NOT also from the domain run.
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

// TestDomainRunVMTargetsFlag mirrors the container filter for VMs.
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
