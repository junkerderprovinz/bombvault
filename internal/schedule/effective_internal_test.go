package schedule

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// EffectiveFileSetSchedule feeds the sentence the Folders card shows, so it has
// to agree with what the scheduler actually does.

func baseSettings() store.Settings {
	return store.Settings{
		FilesEnabled:       true,
		PerItemSchedules:   true,
		FilesSchedule:      "off",
		EverythingSchedule: "off",
	}
}

func set(enabled bool, override string) store.FileSet {
	return store.FileSet{ID: "fs1", Name: "My_Backups", Path: "user/my_backups", Enabled: enabled, ScheduleCadence: override}
}

func TestEffectiveNoneWhenTheFoldersDomainIsOff(t *testing.T) {
	s := baseSettings()
	s.FilesEnabled = false
	s.FilesSchedule = "weekly mon 02:00"
	s.EverythingSchedule = "daily 05:00"
	got := EffectiveFileSetSchedule(set(true, ""), s)
	if got.Kind != EffectiveNone {
		t.Fatalf("domain off must mean nothing runs, got %q", got.Kind)
	}
}

// TestEffectiveNoneWhenIncludeInScheduleIsOff checks that with "Include in
// schedule" off nothing backs the set up, not even Backup Everything.
func TestEffectiveNoneWhenIncludeInScheduleIsOff(t *testing.T) {
	s := baseSettings()
	s.FilesSchedule = "weekly mon 02:00"
	s.EverythingSchedule = "daily 05:00"
	got := EffectiveFileSetSchedule(set(false, ""), s)
	if got.Kind != EffectiveNone {
		t.Fatalf("include off must mean nothing runs, got %q (spec %q)", got.Kind, got.Spec)
	}
}

func TestEffectiveOwnReportsTheOverrideTheUserTyped(t *testing.T) {
	s := baseSettings()
	s.FilesSchedule = "weekly mon 02:00"
	s.EverythingSchedule = "daily 05:00"
	got := EffectiveFileSetSchedule(set(true, "daily 03:00"), s)
	if got.Kind != EffectiveOwn {
		t.Fatalf("an override must give the set its own entry, got %q", got.Kind)
	}
	// Not the compiled cron expression: the card renders the user-facing form.
	if got.Spec != "daily 03:00" {
		t.Fatalf("spec must be the override as typed, got %q", got.Spec)
	}
	if got.AlsoSpec != "" {
		t.Fatalf("an own entry is excluded from both other runs, got a second spec %q", got.AlsoSpec)
	}
}

func TestEffectiveNoneForAnExplicitOffOverride(t *testing.T) {
	s := baseSettings()
	s.FilesSchedule = "weekly mon 02:00"
	s.EverythingSchedule = "daily 05:00"
	got := EffectiveFileSetSchedule(set(true, "off"), s)
	if got.Kind != EffectiveNone {
		t.Fatalf("an off override must mean nothing runs, got %q", got.Kind)
	}
}

func TestEffectiveDomainWhenOnlyTheFoldersScheduleIsOn(t *testing.T) {
	s := baseSettings()
	s.FilesSchedule = "weekly mon 02:00"
	got := EffectiveFileSetSchedule(set(true, ""), s)
	if got.Kind != EffectiveDomain || got.Spec != "weekly mon 02:00" {
		t.Fatalf("expected the domain schedule, got %q / %q", got.Kind, got.Spec)
	}
}

func TestEffectiveEverythingWhenTheFoldersScheduleIsOff(t *testing.T) {
	s := baseSettings()
	s.EverythingSchedule = "daily 05:00"
	got := EffectiveFileSetSchedule(set(true, ""), s)
	if got.Kind != EffectiveEverything || got.Spec != "daily 05:00" {
		t.Fatalf("expected Backup Everything to cover it, got %q / %q", got.Kind, got.Spec)
	}
}

func TestEffectiveBothNamesTwoSchedulesNotOne(t *testing.T) {
	s := baseSettings()
	s.FilesSchedule = "weekly mon 02:00"
	s.EverythingSchedule = "daily 05:00"
	got := EffectiveFileSetSchedule(set(true, ""), s)
	if got.Kind != EffectiveBoth {
		t.Fatalf("two schedules covering one set is a double backup, got %q", got.Kind)
	}
	if got.Spec != "weekly mon 02:00" || got.AlsoSpec != "daily 05:00" {
		t.Fatalf("both cadences must be named, got %q and %q", got.Spec, got.AlsoSpec)
	}
}

func TestEffectiveIgnoresOverridesWhilePerItemSchedulesIsOff(t *testing.T) {
	s := baseSettings()
	s.PerItemSchedules = false
	s.FilesSchedule = "weekly mon 02:00"
	got := EffectiveFileSetSchedule(set(true, "daily 03:00"), s)
	if got.Kind != EffectiveDomain {
		t.Fatalf("with the feature toggle off an override must be inert, got %q", got.Kind)
	}
}

// TestEffectiveTreatsAnUnparseableCadenceAsNotRunning checks that the card does
// not promise a run for a cadence the scheduler cannot register.
func TestEffectiveTreatsAnUnparseableCadenceAsNotRunning(t *testing.T) {
	s := baseSettings()
	s.FilesSchedule = "wöchentlich am Dienstag"
	got := EffectiveFileSetSchedule(set(true, ""), s)
	if got.Kind != EffectiveNone {
		t.Fatalf("a cadence that cannot fire must not be reported as running, got %q", got.Kind)
	}
}

// TestBothSchedulesBackUpEveryIncludedFolderTwice checks that with both the
// Folders and the Backup Everything schedule on, every included set without an
// override is backed up by both.
func TestBothSchedulesBackUpEveryIncludedFolderTwice(t *testing.T) {
	s := baseSettings()
	s.FilesSchedule = "weekly mon 02:00"
	s.EverythingSchedule = "daily 05:00"
	for _, name := range []string{"ISO", "My_Backups", "data", "dockhand"} {
		fs := store.FileSet{ID: name, Name: name, Enabled: true}
		got := EffectiveFileSetSchedule(fs, s)
		if got.Kind != EffectiveBoth {
			t.Fatalf("%s: expected a double backup, got %q", name, got.Kind)
		}
	}
}

// TestExcludedFoldersAreNotBackedUpByAnySchedule checks that sets with "Include
// in schedule" off do not fall back to Backup Everything.
func TestExcludedFoldersAreNotBackedUpByAnySchedule(t *testing.T) {
	s := baseSettings()
	s.FilesSchedule = "weekly mon 02:00"
	s.EverythingSchedule = "daily 05:00"
	for _, name := range []string{"ISO", "data", "dockhand"} {
		fs := store.FileSet{ID: name, Name: name, Enabled: false}
		if got := EffectiveFileSetSchedule(fs, s); got.Kind != EffectiveNone {
			t.Fatalf("%s: expected no backup at all, got %q", name, got.Kind)
		}
	}
	kept := store.FileSet{ID: "My_Backups", Name: "My_Backups", Enabled: true}
	if got := EffectiveFileSetSchedule(kept, s); got.Kind != EffectiveBoth {
		t.Fatalf("My_Backups: expected a double backup, got %q", got.Kind)
	}
}

// TestEverythingPlusOneOverrideRunsEachFolderExactlyOnce checks that with the
// Folders schedule off, every set included and one override, each set runs
// exactly once.
func TestEverythingPlusOneOverrideRunsEachFolderExactlyOnce(t *testing.T) {
	s := baseSettings()
	s.FilesSchedule = "off"
	s.EverythingSchedule = "daily 05:00"
	for _, name := range []string{"ISO", "data", "dockhand"} {
		fs := store.FileSet{ID: name, Name: name, Enabled: true}
		if got := EffectiveFileSetSchedule(fs, s); got.Kind != EffectiveEverything {
			t.Fatalf("%s: expected Backup Everything to cover it, got %q", name, got.Kind)
		}
	}
	own := store.FileSet{ID: "My_Backups", Name: "My_Backups", Enabled: true, ScheduleCadence: "weekly mon 02:00"}
	got := EffectiveFileSetSchedule(own, s)
	if got.Kind != EffectiveOwn || got.Spec != "weekly mon 02:00" {
		t.Fatalf("My_Backups: expected its own weekly entry, got %q / %q", got.Kind, got.Spec)
	}
}
