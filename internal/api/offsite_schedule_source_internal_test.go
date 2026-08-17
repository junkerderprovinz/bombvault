package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// newScheduleSourceTestService builds a Service over a migrated in-memory store —
// enough for the settings/target reads these gate tests exercise.
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

// TestOffsiteScheduleComesFromSettingsNotTarget is the issue #150 regression.
//
// The off-site CADENCE has exactly one owner: the per-domain Settings column that
// Settings › Schedules edits. The scheduler registers each "<domain>-offsite" cron
// entry from THAT column and nothing else (internal/schedule/schedule.go's offsite()
// block), so if the coupled-vs-decoupled decision read the cadence from anywhere
// else the two could disagree — and when they disagree in the direction "some other
// source says decoupled, Settings says blank", the domain's off-site copy is
// suppressed after every backup AND has no cron entry to run it instead: it simply
// never happens, silently, forever.
//
// That is exactly what a per-target `schedule` value used to cause: offsiteScheduleFor
// preferred the off-site TARGET ROW's schedule over the Settings column. A row can
// carry one via the off-site-targets CRUD API (PUT /api/offsite/targets/{id} accepts
// the field) or via a settings import, while Settings › Schedules — which reads the
// Settings column — still shows the cadence as blank. The Folders domain then backed
// up on schedule, pruned on schedule, and never replicated: no error, no log line, no
// run row.
//
// Per-target schedules are deliberately NOT a feature (see OffsiteTargetsSection:
// "every target of a domain replicates on that domain's off-site schedule"), so the
// Settings column is authoritative and a stray row value must be ignored.
func TestOffsiteScheduleComesFromSettingsNotTarget(t *testing.T) {
	s, st := newScheduleSourceTestService(t)

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	// The reporter's shape: Folders has a local repo + an off-site repo, and the
	// off-site cadence in Settings › Schedules is BLANK ("replicate after each
	// backup" — the coupled mode).
	settings.FilesEnabled = true
	settings.FilesPath = "backups/files"
	settings.FilesSchedule = "daily 06:00"
	settings.FilesOffsite = "rest:http://192.168.1.2:8000/files"
	settings.FilesOffsiteSchedule = ""
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	// ...but the domain's primary off-site target row carries a cadence of its own.
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "files", Name: "Primary", Repo: settings.FilesOffsite,
		Schedule: "weekly Sun 03:00", Enabled: true, SortOrder: 0,
	}); err != nil {
		t.Fatal(err)
	}

	// The scheduler registers the decoupled off-site entry from the SETTINGS column,
	// which is blank — so there is no "files-offsite" cron entry at all.
	if cad, err := schedule.ParseCadence(settings.FilesOffsiteSchedule); err != nil || cad.Enabled {
		t.Fatalf("precondition: blank FilesOffsiteSchedule must not register a cron entry (enabled=%v err=%v)", cad.Enabled, err)
	}

	// Therefore the coupled path MUST own the replication: the domain must not be
	// treated as replicating on its own schedule, or the copy is lost entirely.
	if got := s.offsiteScheduleFor("files", settings); got != "" {
		t.Fatalf("offsiteScheduleFor(files) = %q, want %q — the Settings column is the single source of truth for the off-site cadence; a stray target-row schedule must not override it (#150)", got, "")
	}
	if s.offsiteReplicatesOnOwnSchedule("files", settings) {
		t.Fatal("files must NOT be treated as replicating on its own off-site schedule: Settings › Schedules is blank, so no cron entry exists and the coupled after-backup copy is the only thing that can replicate it (#150)")
	}
}

// TestOffsiteScheduleStillHonoursSettingsCadence is the other half of the contract:
// when the Settings column DOES carry a cadence, the domain is decoupled (its own
// cron entry drives replication) and the coupled after-backup copy stands down — so
// the fix above cannot regress the decoupled mode into replicating twice.
func TestOffsiteScheduleStillHonoursSettingsCadence(t *testing.T) {
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
	// A target row whose schedule is blank must NOT drag the domain back into
	// coupled mode either — Settings still wins.
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
