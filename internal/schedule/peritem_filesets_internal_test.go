package schedule

import (
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// fakeGateStore is a minimal DomainGateStore that records which item IDs the
// gate asked about.
type fakeGateStore struct {
	sets     []store.FileSet
	settings store.Settings
	askedFor []string
}

func (f *fakeGateStore) GetSettings() (store.Settings, error)     { return f.settings, nil }
func (f *fakeGateStore) ListTargets() ([]store.Target, error)     { return nil, nil }
func (f *fakeGateStore) ListVMTargets() ([]store.VMTarget, error) { return nil, nil }
func (f *fakeGateStore) ListFileSets() ([]store.FileSet, error)   { return f.sets, nil }
func (f *fakeGateStore) LastSuccessfulBackupAmong(ids []string) (time.Time, error) {
	f.askedFor = append([]string{}, ids...)
	return time.Time{}, nil
}

// With PerItemSchedules on, a folder set with its own cadence leaves the domain
// run and gets a cron entry of its own, so one large set can run weekly while
// the rest run nightly.

// TestDomainRunFileSetsIsIdentityWhileTheFeatureIsOff checks that the list stays
// untouched with the feature off, so an install that never uses it sees no
// change.
func TestDomainRunFileSetsIsIdentityWhileTheFeatureIsOff(t *testing.T) {
	sets := []store.FileSet{
		{ID: "a", Name: "media", Enabled: true, ScheduleCadence: "weekly sun 04:00"},
		{ID: "b", Name: "docs", Enabled: true},
		{ID: "c", Name: "old", Enabled: true, ScheduleCadence: "off"},
	}
	got := DomainRunFileSets(sets, false)
	if len(got) != len(sets) {
		t.Fatalf("feature off must not filter: got %d of %d", len(got), len(sets))
	}
	for i := range got {
		if got[i].ID != sets[i].ID {
			t.Fatalf("feature off must preserve order: %v", got)
		}
	}
}

// TestDomainRunFileSetsDropsOverriddenAndOffSets checks that with the feature on,
// a set carrying its own cadence leaves the domain run, a set explicitly "off"
// leaves it too, and everything else stays.
func TestDomainRunFileSetsDropsOverriddenAndOffSets(t *testing.T) {
	sets := []store.FileSet{
		{ID: "a", Name: "media", Enabled: true, ScheduleCadence: "weekly sun 04:00"}, // own entry
		{ID: "b", Name: "docs", Enabled: true},                                       // domain default
		{ID: "c", Name: "old", Enabled: true, ScheduleCadence: "off"},                // not scheduled
		{ID: "d", Name: "junk", Enabled: true, ScheduleCadence: "nonsense"},          // invalid -> domain
		{ID: "e", Name: "n", Enabled: true, ScheduleCadence: "everyN 3 04:00"},       // unsupported -> domain
	}
	got := DomainRunFileSets(sets, true)
	want := []string{"b", "d", "e"}
	if len(got) != len(want) {
		t.Fatalf("got %d sets, want %d: %+v", len(got), len(want), got)
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("position %d: got %q, want %q", i, got[i].ID, id)
		}
	}
}

// TestPerItemFileSetEntriesFollowTheToggle checks that a set on its own cadence
// gets a cron entry of its own, and that toggling the feature off takes it away
// again.
func TestPerItemFileSetEntriesFollowTheToggle(t *testing.T) {
	sets := []store.FileSet{
		{ID: "a", Name: "media", Enabled: true, ScheduleCadence: "weekly sun 04:00"}, // own entry
		{ID: "b", Name: "docs", Enabled: true},                                       // domain default
		{ID: "c", Name: "old", Enabled: true, ScheduleCadence: "off"},                // none
		{ID: "d", Name: "gone", Enabled: false, ScheduleCadence: "daily 05:00"},      // disabled -> none
	}
	backup, _ := recordingBackup()
	sc := New(backup, func() ([]store.Target, error) { return nil, nil })
	filesBackup, _ := recordingBackup()
	sc.SetFilesJob(filesBackup, func() ([]store.FileSet, error) { return sets, nil })

	off := store.Settings{FilesEnabled: true, FilesSchedule: "daily 03:00"}
	if err := sc.ReloadWithDueChecks(off, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("reload (off): %v", err)
	}
	if got := len(sc.entries); got != 1 {
		t.Fatalf("feature off: want 1 entry (files domain), got %d", got)
	}

	on := off
	on.PerItemSchedules = true
	if err := sc.ReloadWithDueChecks(on, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("reload (on): %v", err)
	}
	if got := len(sc.entries); got != 2 {
		t.Fatalf("feature on: want 2 entries (domain + media), got %d", got)
	}

	if err := sc.ReloadWithDueChecks(off, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("reload (off again): %v", err)
	}
	if got := len(sc.entries); got != 1 {
		t.Fatalf("toggled off: want 1 entry again, got %d", got)
	}
}

// TestPerItemFileSetRunsOnlyItsOwnSet checks that the nightly run leaves out the
// weekly set and still covers the rest, so each set is backed up exactly once
// across both entries.
func TestPerItemFileSetRunsOnlyItsOwnSet(t *testing.T) {
	sets := []store.FileSet{
		{ID: "media", Name: "media", Enabled: true, ScheduleCadence: "weekly sun 04:00"},
		{ID: "docs", Name: "docs", Enabled: true},
	}
	backup, _ := recordingBackup()
	sc := New(backup, func() ([]store.Target, error) { return nil, nil })
	filesBackup, rec := recordingBackup()
	sc.SetFilesJob(filesBackup, func() ([]store.FileSet, error) { return sets, nil })

	on := store.Settings{FilesEnabled: true, FilesSchedule: "daily 03:00", PerItemSchedules: true}
	if err := sc.ReloadWithDueChecks(on, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(sc.entries) != 2 {
		t.Fatalf("want 2 entries (domain + media), got %d", len(sc.entries))
	}

	for _, e := range sc.c.Entries() {
		e.Job.Run()
	}

	counts := map[string]int{}
	for _, id := range *rec {
		counts[id]++
	}
	if counts["media"] != 1 || counts["docs"] != 1 || len(counts) != 2 {
		t.Fatalf("each set must run exactly once across both entries, got %v", counts)
	}
}

// TestFilesDueGateIgnoresSetsOnTheirOwnCadence checks that the due gate ignores
// a set on its own cadence; otherwise one weekly set would hold the nightly gate
// closed for every daily set beside it.
func TestFilesDueGateIgnoresSetsOnTheirOwnCadence(t *testing.T) {
	sets := []store.FileSet{
		{ID: "media", Name: "media", Enabled: true, ScheduleCadence: "weekly sun 04:00"},
		{ID: "docs", Name: "docs", Enabled: true},
		{ID: "off", Name: "off", Enabled: false},
	}
	st := &fakeGateStore{sets: sets, settings: store.Settings{PerItemSchedules: true}}
	if _, err := FilesDueGate(st)(); err != nil {
		t.Fatalf("due gate: %v", err)
	}
	if len(st.askedFor) != 1 || st.askedFor[0] != "docs" {
		t.Fatalf("the gate must ask only about the domain-cadence sets, got %v", st.askedFor)
	}

	// With the feature off, every enabled set counts again.
	st2 := &fakeGateStore{sets: sets, settings: store.Settings{}}
	if _, err := FilesDueGate(st2)(); err != nil {
		t.Fatalf("due gate (off): %v", err)
	}
	if len(st2.askedFor) != 2 {
		t.Fatalf("feature off: want both enabled sets, got %v", st2.askedFor)
	}
}

// TestPerItemFileSetEntryRegistersWithTheDomainScheduleOff checks that with the
// Folders schedule off and Backup Everything doing the nightly run, a set on its
// own weekly cadence still gets its entry, although its domain has no schedule.
func TestPerItemFileSetEntryRegistersWithTheDomainScheduleOff(t *testing.T) {
	sets := []store.FileSet{
		{ID: "iso", Name: "ISO", Enabled: true},
		{ID: "mine", Name: "My_Backups", Enabled: true, ScheduleCadence: "weekly sun 04:00"},
		{ID: "data", Name: "data", Enabled: true},
		{ID: "dock", Name: "dockhand", Enabled: true},
	}
	backup, _ := recordingBackup()
	sc := New(backup, func() ([]store.Target, error) { return nil, nil })
	filesBackup, rec := recordingBackup()
	sc.SetFilesJob(filesBackup, func() ([]store.FileSet, error) { return sets, nil })

	on := store.Settings{FilesEnabled: true, FilesSchedule: "off", PerItemSchedules: true}
	if err := sc.ReloadWithDueChecks(on, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := len(sc.entries); got != 1 {
		t.Fatalf("want exactly 1 entry (My_Backups, no domain entry), got %d", got)
	}

	for _, e := range sc.c.Entries() {
		e.Job.Run()
	}
	if len(*rec) != 1 || (*rec)[0] != "mine" {
		t.Fatalf("the weekly entry must back up My_Backups alone, got %v", *rec)
	}
}

// TestBackupEverythingSkipsAFolderSetOnItsOwnCadence checks that Backup
// Everything skips a set on its own cadence and still covers the rest. The API's
// everythingRunFiles applies this filter.
func TestBackupEverythingSkipsAFolderSetOnItsOwnCadence(t *testing.T) {
	sets := []store.FileSet{
		{ID: "iso", Name: "ISO", Enabled: true},
		{ID: "mine", Name: "My_Backups", Enabled: true, ScheduleCadence: "weekly sun 04:00"},
		{ID: "data", Name: "data", Enabled: true},
		{ID: "dock", Name: "dockhand", Enabled: true},
	}
	got := DomainRunFileSets(sets, true)
	var names []string
	for _, fs := range got {
		names = append(names, fs.Name)
	}
	want := []string{"ISO", "data", "dockhand"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("got %v, want %v", names, want)
		}
	}
}
