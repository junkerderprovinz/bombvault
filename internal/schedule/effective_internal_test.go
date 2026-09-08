package schedule

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The sentence the Folders card shows must be the truth about what the scheduler
// will do, not a second opinion. These tests pin every outcome, and the last two
// rebuild manilx's screenshots from #199 literally rather than reasoning about
// them - reasoning about them is exactly what produced the wrong answer twice.

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

// The branch whose label lies. "Include in schedule" off is not "skipped by the
// Folders schedule", it is "never backed up by anything".
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

// The shape #199 was actually built for, and the one manilx wanted: Folders
// schedule off, Backup Everything doing the work.
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

// An unparseable cadence never fires (registerJobs logs and skips it), so the
// card must not promise a run.
func TestEffectiveTreatsAnUnparseableCadenceAsNotRunning(t *testing.T) {
	s := baseSettings()
	s.FilesSchedule = "wöchentlich am Dienstag"
	got := EffectiveFileSetSchedule(set(true, ""), s)
	if got.Kind != EffectiveNone {
		t.Fatalf("a cadence that cannot fire must not be reported as running, got %q", got.Kind)
	}
}

// manilx's first screenshot, verbatim: Folders weekly Mon 02:00, Backup
// Everything daily 05:00, all four sets included and none overridden. He asked
// "Now ALL folders backup weekly, right?" - no. Weekly AND daily, all four.
func TestManilxFirstScreenshotBacksUpEveryFolderTwice(t *testing.T) {
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

// manilx's second screenshot: same schedules, but only My_Backups left with
// "Include in schedule" on. He assumed the other three would fall back to
// Backup Everything. They fall out of everything instead.
func TestManilxSecondScreenshotLeavesThreeFoldersUnprotected(t *testing.T) {
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

// And the configuration he should have: Folders schedule off, every set
// included, one override. Exactly one run each, nothing duplicated, nothing
// dropped.
func TestManilxTargetConfigurationRunsEachFolderExactlyOnce(t *testing.T) {
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
