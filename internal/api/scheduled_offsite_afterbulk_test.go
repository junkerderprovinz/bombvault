package api_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestScheduledFilesRunReplicatesOffsite is the end-to-end regression for issue
// #150 ("scheduled offsite for Folders never runs"): a REAL Service wired to a
// REAL Scheduler exactly the way cmd/bombvault/main.go wires them, driven through
// the actual registered cron job, must replicate the Folders domain off-site after
// its scheduled backup loop — and record the kind="offsite" run the activity log
// renders as "Off-site replication done — Folders".
//
// The reporter's install had the shape below: Folders backed up on schedule every
// day and pruned on schedule, but never replicated, while Containers/VMs/Flash/
// Self-Backup all replicated fine. Settings › Schedules showed every off-site
// cadence blank (coupled mode: "replicate after each backup"), yet the Folders
// primary off-site TARGET ROW carried a cadence of its own — a value the CRUD API
// accepts and a settings import restores, and which nothing in the UI surfaces.
// offsiteScheduleFor preferred that row over the Settings column, so the domain was
// judged "replicates on its own schedule" and the coupled after-bulk copy stood
// down — while the scheduler, which registers "<domain>-offsite" cron entries from
// the SETTINGS column (blank), had never registered an entry to do it instead. The
// copy fell between the two and vanished silently.
//
// The divergent row is seeded deliberately: without it this test passes even on the
// broken code, so it is what makes this a real regression guard.
func TestScheduledFilesRunReplicatesOffsite(t *testing.T) {
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
	st := newMemStore(t)

	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.FilesEnabled = true
	s.FilesPath = "backups/files"
	s.FilesSchedule = "daily 06:00"
	s.FilesOffsite = "rest:http://192.168.1.2:8000/files"
	s.FilesOffsiteSchedule = "" // blank in Settings › Schedules = coupled
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	// The divergent primary target row (see the doc comment): a cadence lives here
	// while Settings says blank. The Settings column must win.
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "files", Name: "Primary", Repo: s.FilesOffsite,
		Schedule: "weekly Sun 03:00", Enabled: true, SortOrder: 0,
	}); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{"data/docs", "data/pics"} {
		if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(p)), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.CreateFileSet(store.FileSet{Name: "docs", Path: "data/docs", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateFileSet(store.FileSet{Name: "pics", Path: "data/pics", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	d := &fakeServiceDocker{inspect: model.Inspect{Name: "/app", Image: "app:latest", Running: true}}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

	// Wiring mirrored from cmd/bombvault/main.go: the scheduled files job suppresses
	// each set's inline off-site copy (so the batch replicates once at the end), and
	// the after-bulk hooks run the batched prune then the batched replication.
	sched := schedule.New(func(string) error { return nil }, st.ListTargets)
	sched.SetFilesJob(func(id string) error {
		ctx := api.WithBulkReplicateSuppressed(
			notify.WithMessagesSuppressed(notify.WithHealthchecksSuppressed(context.Background())))
		_, bErr := svc.BackupFileSet(ctx, id)
		return bErr
	}, st.ListFileSets)
	sched.SetPruneAfterBulkJob(func(domain string) { svc.PruneAfterBulk(context.Background(), domain) })
	sched.SetOffsiteAfterBulkJob(func(domain string) { svc.ReplicateOffsiteAfterBulk(context.Background(), domain) })

	// A stale last-run makes the files entry "missed", so CatchUpMissed fires the
	// SAME wrapped cron job a real 06:00 trigger would run.
	stale := func() (time.Time, error) { return time.Now().Add(-72 * time.Hour), nil }
	if err := sched.ReloadWithDueChecks(mustSettings(t, st), nil, nil, nil, nil, stale); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}
	if ran := sched.CatchUpMissed(time.Now()); len(ran) != 1 || ran[0] != "files" {
		t.Fatalf("expected the scheduled files job to fire, got %v", ran)
	}

	if len(eng.backedUp) != 2 {
		t.Fatalf("want both file sets backed up by the scheduled run, got %d (%v)", len(eng.backedUp), eng.backedUp)
	}
	if len(eng.copied) != 1 {
		t.Fatalf("a scheduled Folders run with a coupled (blank) off-site cadence must replicate the domain exactly ONCE after the loop, got %d copies: %v (#150)", len(eng.copied), eng.copied)
	}

	// ...and it must be recorded as the kind="offsite" run the activity log renders
	// as "Off-site replication done — Folders". The reporter's log had the backups
	// and the prune but never this row.
	runs, err := st.ListRuns(100)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, r := range runs {
		if r.Kind == "offsite" && r.TargetID == "files" {
			found = true
			if r.Status != "success" {
				t.Fatalf("files off-site run recorded as %q (error %q), want success", r.Status, r.Error)
			}
		}
	}
	if !found {
		t.Fatalf("no kind=offsite run recorded for the files domain — the activity log would never show \"Off-site replication done — Folders\" (#150); runs=%+v", runs)
	}
}
