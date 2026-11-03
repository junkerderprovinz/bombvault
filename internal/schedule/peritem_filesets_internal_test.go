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
	zfs      []store.ZFSDataset
	askedFor []string
	// passes are the recorded domain passes by job name.
	passes map[string]time.Time
}

func (f *fakeGateStore) ListTargets() ([]store.Target, error)     { return nil, nil }
func (f *fakeGateStore) ListVMTargets() ([]store.VMTarget, error) { return nil, nil }
func (f *fakeGateStore) ListFileSets() ([]store.FileSet, error)   { return f.sets, nil }
func (f *fakeGateStore) ListZFSDatasets() ([]store.ZFSDataset, error) {
	return f.zfs, nil
}
func (f *fakeGateStore) LastSuccessfulBackupAmong(ids []string) (time.Time, error) {
	f.askedFor = append([]string{}, ids...)
	return time.Time{}, nil
}
func (f *fakeGateStore) LastScheduleJobRun(job string) (time.Time, error) {
	return f.passes[job], nil
}

// A folder set with its own cadence leaves the domain run and gets a cron entry
// of its own, so one large set can run weekly while the rest run nightly.

// TestDomainRunFileSetsDropsOverriddenAndOffSets checks that a set carrying its
// own cadence leaves the domain run, a set explicitly "off" leaves it too, and
// everything else stays.
func TestDomainRunFileSetsDropsOverriddenAndOffSets(t *testing.T) {
	sets := []store.FileSet{
		{ID: "a", Name: "media", Enabled: true, ScheduleCadence: "weekly sun 04:00"}, // own entry
		{ID: "b", Name: "docs", Enabled: true},                                       // domain default
		{ID: "c", Name: "old", Enabled: true, ScheduleCadence: "off"},                // not scheduled
		{ID: "d", Name: "junk", Enabled: true, ScheduleCadence: "nonsense"},          // invalid -> domain
		{ID: "e", Name: "n", Enabled: true, ScheduleCadence: "everyN 3 04:00"},       // unsupported -> domain
	}
	got := DomainRunFileSets(sets)
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

// TestAFolderSetWithItsOwnPlanGetsAnEntryOfItsOwn checks that a stored plan
// counts by itself: the set gets a cron entry beside the domain's, while an
// "off" or disabled set gets none.
func TestAFolderSetWithItsOwnPlanGetsAnEntryOfItsOwn(t *testing.T) {
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

	settings := store.Settings{FilesEnabled: true, FilesSchedule: "daily 03:00"}
	if err := sc.ReloadWithDueChecks(settings, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := len(sc.entries); got != 2 {
		t.Fatalf("want 2 entries (domain + media), got %d", got)
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

	on := store.Settings{FilesEnabled: true, FilesSchedule: "daily 03:00"}
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
	st := &fakeGateStore{sets: sets}
	if _, err := FilesDueGate(st)(); err != nil {
		t.Fatalf("due gate: %v", err)
	}
	if len(st.askedFor) != 1 || st.askedFor[0] != "docs" {
		t.Fatalf("the gate must ask only about the domain-cadence sets, got %v", st.askedFor)
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

	on := store.Settings{FilesEnabled: true, FilesSchedule: "off"}
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
	got := DomainRunFileSets(sets)
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
