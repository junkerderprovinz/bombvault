package schedule

import (
	"errors"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// entriesFor returns the registered entries of one job and domain.
func entriesFor(sc *Scheduler, job, domain string) []scheduledEntry {
	var out []scheduledEntry
	for _, e := range sc.entries {
		if e.job == job && e.domain == domain {
			out = append(out, e)
		}
	}
	return out
}

// newZFSScheduler wires a scheduler whose ZFS job records the item IDs it backs
// up.
func newZFSScheduler(items []store.ZFSDataset) (*Scheduler, *[]string) {
	backup, _ := recordingBackup()
	sc := New(backup, func() ([]store.Target, error) { return nil, nil })
	zfsBackup, rec := recordingBackup()
	sc.SetZFSJob(zfsBackup, func() ([]store.ZFSDataset, error) { return items, nil })
	return sc, rec
}

// TestReloadWithGatesRegistersZFSDomainJob checks that an enabled ZFS domain
// gets its own backup entry and that firing it covers every enabled item once.
func TestReloadWithGatesRegistersZFSDomainJob(t *testing.T) {
	items := []store.ZFSDataset{
		{ID: "appdata", Dataset: "tank/appdata", Enabled: true},
		{ID: "media", Dataset: "tank/media", Enabled: true},
		{ID: "old", Dataset: "tank/old", Enabled: false},
	}
	sc, rec := newZFSScheduler(items)

	settings := store.Settings{ZFSEnabled: true, ZFSSchedule: "daily 03:00"}
	if err := sc.ReloadWithGates(settings, DueGates{}); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := entriesFor(sc, "backup", "zfs"); len(got) != 1 {
		t.Fatalf("want one zfs backup entry, got %d", len(got))
	}

	for _, e := range sc.c.Entries() {
		e.Job.Run()
	}
	if len(*rec) != 2 || (*rec)[0] != "appdata" || (*rec)[1] != "media" {
		t.Fatalf("the domain run must cover the enabled items, got %v", *rec)
	}
}

// TestZFSDomainJobOffWhenDisabled checks that a cadence alone does not register
// the domain; the ZFS switch has to be on too.
func TestZFSDomainJobOffWhenDisabled(t *testing.T) {
	sc, _ := newZFSScheduler([]store.ZFSDataset{{ID: "a", Dataset: "tank/a", Enabled: true}})

	settings := store.Settings{ZFSSchedule: "daily 03:00"}
	if err := sc.ReloadWithGates(settings, DueGates{}); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := entriesFor(sc, "backup", "zfs"); len(got) != 0 {
		t.Fatalf("a switched-off domain must register nothing, got %d entries", len(got))
	}
}

// TestZFSOffsiteEntry checks that the ZFS off-site schedule registers with the
// domain on and stays inert with it off, because a domain that makes no
// snapshots has nothing to replicate.
func TestZFSOffsiteEntry(t *testing.T) {
	sc, _ := newZFSScheduler(nil)
	var replicated []string
	sc.SetOffsiteJob(func(domain string) error {
		replicated = append(replicated, domain)
		return nil
	})

	on := store.Settings{ZFSEnabled: true, ZFSOffsiteSchedule: "daily 04:00"}
	if err := sc.ReloadWithGates(on, DueGates{}); err != nil {
		t.Fatalf("reload (on): %v", err)
	}
	if got := entriesFor(sc, "offsite", "zfs"); len(got) != 1 {
		t.Fatalf("want one zfs off-site entry, got %d", len(got))
	}
	for _, e := range sc.c.Entries() {
		e.Job.Run()
	}
	if len(replicated) != 1 || replicated[0] != "zfs" {
		t.Fatalf("the off-site entry must replicate the zfs domain, got %v", replicated)
	}

	off := on
	off.ZFSEnabled = false
	if err := sc.ReloadWithGates(off, DueGates{}); err != nil {
		t.Fatalf("reload (off): %v", err)
	}
	if got := entriesFor(sc, "offsite", "zfs"); len(got) != 0 {
		t.Fatalf("a switched-off domain must not replicate, got %d entries", len(got))
	}
}

// TestRunZFSJobSkipsDisabledAndNamesFailuresByDataset checks that the job backs
// up by item ID, skips disabled items, keeps going after a failure and reports
// the failure under the dataset name the user knows.
func TestRunZFSJobSkipsDisabledAndNamesFailuresByDataset(t *testing.T) {
	items := []store.ZFSDataset{
		{ID: "one", Dataset: "tank/appdata", Enabled: true},
		{ID: "two", Dataset: "tank/media", Enabled: true},
		{ID: "three", Dataset: "tank/old", Enabled: false},
	}
	var asked []string
	attempted, failed, failures := RunZFSJob(items, func(id string) error {
		asked = append(asked, id)
		if id == "one" {
			return errors.New("pool is busy")
		}
		return nil
	})

	if len(asked) != 2 || asked[0] != "one" || asked[1] != "two" {
		t.Fatalf("the job must back up the enabled items by ID, got %v", asked)
	}
	if attempted != 2 || failed != 1 {
		t.Fatalf("got attempted=%d failed=%d, want 2 and 1", attempted, failed)
	}
	if len(failures) != 1 || failures[0].Name != "tank/appdata" {
		t.Fatalf("a failure must be named by its dataset, got %+v", failures)
	}
	if failures[0].Reason != "pool is busy" {
		t.Fatalf("the failure must carry the backup error, got %q", failures[0].Reason)
	}
}

// TestDomainRunZFSDatasetsDropsOverrides checks that with per-item schedules on,
// an item with its own cadence leaves the domain run, an item switched to "off"
// leaves it too, and an unusable override falls back to the domain.
func TestDomainRunZFSDatasetsDropsOverrides(t *testing.T) {
	items := []store.ZFSDataset{
		{ID: "a", Dataset: "tank/a", Enabled: true, ScheduleCadence: "weekly sun 04:00"},
		{ID: "b", Dataset: "tank/b", Enabled: true},
		{ID: "c", Dataset: "tank/c", Enabled: true, ScheduleCadence: "off"},
		{ID: "d", Dataset: "tank/d", Enabled: true, ScheduleCadence: "nonsense"},
		{ID: "e", Dataset: "tank/e", Enabled: true, ScheduleCadence: "everyN 3 04:00"},
	}
	if got := DomainRunZFSDatasets(items, false); len(got) != len(items) {
		t.Fatalf("the feature off must not filter: got %d of %d", len(got), len(items))
	}

	got := DomainRunZFSDatasets(items, true)
	want := []string{"b", "d", "e"}
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d: %+v", len(got), len(want), got)
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("position %d: got %q, want %q", i, got[i].ID, id)
		}
	}
}

// TestDomainRunHasZFSWork checks that a list of only disabled items counts as no
// work, so the prune and the off-site copy after an empty pass are skipped.
func TestDomainRunHasZFSWork(t *testing.T) {
	if DomainRunHasZFSWork(nil) {
		t.Fatal("an empty list is no work")
	}
	if DomainRunHasZFSWork([]store.ZFSDataset{{ID: "a", Enabled: false}}) {
		t.Fatal("a disabled item is no work")
	}
	if !DomainRunHasZFSWork([]store.ZFSDataset{{ID: "a", Enabled: false}, {ID: "b", Enabled: true}}) {
		t.Fatal("one enabled item is work")
	}
}

// TestZFSDueGateUsesOnlyDomainRunRows checks that the gate asks about the items
// the domain run covers. An item on its own weekly cadence backing up today
// would otherwise hold the nightly gate closed for every item beside it.
func TestZFSDueGateUsesOnlyDomainRunRows(t *testing.T) {
	items := []store.ZFSDataset{
		{ID: "weekly", Dataset: "tank/media", Enabled: true, ScheduleCadence: "weekly sun 04:00"},
		{ID: "nightly", Dataset: "tank/appdata", Enabled: true},
		{ID: "paused", Dataset: "tank/old", Enabled: false},
	}
	st := &fakeGateStore{zfs: items, settings: store.Settings{PerItemSchedules: true}}
	if _, err := ZFSDueGate(st)(); err != nil {
		t.Fatalf("due gate: %v", err)
	}
	if len(st.askedFor) != 1 || st.askedFor[0] != "nightly" {
		t.Fatalf("the gate must ask only about the domain-cadence items, got %v", st.askedFor)
	}

	st2 := &fakeGateStore{zfs: items, settings: store.Settings{}}
	if _, err := ZFSDueGate(st2)(); err != nil {
		t.Fatalf("due gate (per-item off): %v", err)
	}
	if len(st2.askedFor) != 2 {
		t.Fatalf("per-item off: want both enabled items, got %v", st2.askedFor)
	}
}

// TestRegisterPerItemEntriesZFS checks that an item with its own cadence gets a
// cron entry of its own and is backed up exactly once across both entries.
func TestRegisterPerItemEntriesZFS(t *testing.T) {
	items := []store.ZFSDataset{
		{ID: "media", Dataset: "tank/media", Enabled: true, ScheduleCadence: "weekly sun 04:00"},
		{ID: "appdata", Dataset: "tank/appdata", Enabled: true},
		{ID: "paused", Dataset: "tank/old", Enabled: false, ScheduleCadence: "daily 05:00"},
	}
	sc, rec := newZFSScheduler(items)

	settings := store.Settings{ZFSEnabled: true, ZFSSchedule: "daily 03:00", PerItemSchedules: true}
	if err := sc.ReloadWithGates(settings, DueGates{}); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := entriesFor(sc, "backup", "zfs"); len(got) != 2 {
		t.Fatalf("want the domain entry plus one per-item entry, got %d", len(got))
	}

	for _, e := range sc.c.Entries() {
		e.Job.Run()
	}
	counts := map[string]int{}
	for _, id := range *rec {
		counts[id]++
	}
	if counts["media"] != 1 || counts["appdata"] != 1 || len(counts) != 2 {
		t.Fatalf("each item must run exactly once across both entries, got %v", counts)
	}
}

// TestEffectiveZFSDatasetSchedule checks the sentence the page shows for one
// item against the inputs the scheduler reads.
func TestEffectiveZFSDatasetSchedule(t *testing.T) {
	cases := []struct {
		name     string
		item     store.ZFSDataset
		settings store.Settings
		want     string
	}{
		{
			name:     "domain off",
			item:     store.ZFSDataset{Enabled: true},
			settings: store.Settings{ZFSSchedule: "daily 03:00"},
			want:     EffectiveNone,
		},
		{
			name:     "item off",
			item:     store.ZFSDataset{},
			settings: store.Settings{ZFSEnabled: true, ZFSSchedule: "daily 03:00"},
			want:     EffectiveNone,
		},
		{
			name:     "domain cadence",
			item:     store.ZFSDataset{Enabled: true},
			settings: store.Settings{ZFSEnabled: true, ZFSSchedule: "daily 03:00"},
			want:     EffectiveDomain,
		},
		{
			name:     "own cadence",
			item:     store.ZFSDataset{Enabled: true, ScheduleCadence: "weekly sun 04:00"},
			settings: store.Settings{ZFSEnabled: true, ZFSSchedule: "daily 03:00", PerItemSchedules: true},
			want:     EffectiveOwn,
		},
		{
			name:     "override off",
			item:     store.ZFSDataset{Enabled: true, ScheduleCadence: "off"},
			settings: store.Settings{ZFSEnabled: true, ZFSSchedule: "daily 03:00", PerItemSchedules: true},
			want:     EffectiveNone,
		},
		{
			name:     "everything only",
			item:     store.ZFSDataset{Enabled: true},
			settings: store.Settings{ZFSEnabled: true, ZFSSchedule: "off", EverythingSchedule: "daily 02:00"},
			want:     EffectiveEverything,
		},
		{
			name:     "domain and everything",
			item:     store.ZFSDataset{Enabled: true},
			settings: store.Settings{ZFSEnabled: true, ZFSSchedule: "daily 03:00", EverythingSchedule: "daily 02:00"},
			want:     EffectiveBoth,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EffectiveZFSDatasetSchedule(tc.item, tc.settings); got.Kind != tc.want {
				t.Fatalf("got %+v, want kind %q", got, tc.want)
			}
		})
	}
}

// TestZFSDomainJobJoinsTheCatchUp checks that the domain entry carries its
// everyN interval and last-run query, so a box that slept through the fire
// still backs up once it is back.
func TestZFSDomainJobJoinsTheCatchUp(t *testing.T) {
	sc, _ := newZFSScheduler([]store.ZFSDataset{{ID: "a", Dataset: "tank/a", Enabled: true}})

	settings := store.Settings{ZFSEnabled: true, ZFSSchedule: "everyN 3 03:00"}
	gates := DueGates{ZFS: func() (time.Time, error) { return time.Time{}, nil }}
	if err := sc.ReloadWithGates(settings, gates); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(sc.catchUps) != 1 || sc.catchUps[0].domain != "zfs" {
		t.Fatalf("want the zfs domain in the catch-up set, got %+v", sc.catchUps)
	}

	// Without a gate an everyN cadence would fire daily, so the entry is
	// refused instead.
	if err := sc.ReloadWithGates(settings, DueGates{}); err != nil {
		t.Fatalf("reload (no gate): %v", err)
	}
	if got := entriesFor(sc, "backup", "zfs"); len(got) != 0 {
		t.Fatalf("everyN without a gate must not register, got %d entries", len(got))
	}
}
