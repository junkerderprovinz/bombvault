package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func newScheduleSourceTestService(t *testing.T) (*Service, *store.Repo) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)
	return &Service{store: st}, st
}

// The scheduler registers the "<domain>-offsite" cron entry from the Settings
// column alone. Reading a target row's schedule instead would let a blank
// column plus a scheduled row skip the copy after each backup with no cron
// entry to make up for it, and the domain would never replicate.
func TestOffsiteScheduleComesFromSettingsNotTarget(t *testing.T) {
	s, st := newScheduleSourceTestService(t)

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	// Files has an off-site repo and a blank off-site cadence, so it replicates
	// after each backup.
	settings.FilesEnabled = true
	settings.FilesPath = "backups/files"
	settings.FilesSchedule = "daily 06:00"
	settings.FilesOffsite = "rest:http://192.168.1.2:8000/files"
	settings.FilesOffsiteSchedule = ""
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	// The primary target row carries a cadence of its own.
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "files", Name: "Primary", Repo: settings.FilesOffsite,
		Schedule: "weekly Sun 03:00", Enabled: true, SortOrder: 0,
	}); err != nil {
		t.Fatal(err)
	}

	if cad, err := schedule.ParseCadence(settings.FilesOffsiteSchedule); err != nil || cad.Enabled {
		t.Fatalf("precondition: blank FilesOffsiteSchedule must not register a cron entry (enabled=%v err=%v)", cad.Enabled, err)
	}

	if got := s.offsiteScheduleFor("files", settings); got != "" {
		t.Fatalf("offsiteScheduleFor(files) = %q, want %q; the Settings column is the single source of truth for the off-site cadence; a stray target-row schedule must not override it (#150)", got, "")
	}
	if s.offsiteReplicatesOnOwnSchedule("files", settings) {
		t.Fatal("files must NOT be treated as replicating on its own off-site schedule: Settings › Schedules is blank, so no cron entry exists and the coupled after-backup copy is the only thing that can replicate it (#150)")
	}
}

// With a cadence in the Settings column, the domain's own cron entry
// replicates it and the copy after each backup stands down, whatever the
// target row says.
func TestOffsiteScheduleHonoursSettingsCadence(t *testing.T) {
	s, st := newScheduleSourceTestService(t)

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.FilesOffsite = "rest:http://192.168.1.2:8000/files"
	settings.FilesOffsiteSchedule = "weekly Sun 03:00"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "files", Name: "Primary", Repo: settings.FilesOffsite,
		Schedule: "", Enabled: true, SortOrder: 0,
	}); err != nil {
		t.Fatal(err)
	}

	if got := s.offsiteScheduleFor("files", settings); got != "weekly Sun 03:00" {
		t.Fatalf("offsiteScheduleFor(files) = %q, want the Settings cadence %q", got, "weekly Sun 03:00")
	}
	if !s.offsiteReplicatesOnOwnSchedule("files", settings) {
		t.Fatal("files with a Settings off-site cadence must replicate on its own schedule (its cron entry drives it)")
	}
}
