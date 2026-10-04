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

// A scheduled Folders run with a blank (coupled) off-site cadence replicates the
// domain once after its backup loop and records an offsite run. Service and
// scheduler are wired as in cmd/bombvault/main.go and driven through the cron job.
//
// The primary off-site target row carries a cadence of its own, which the UI
// never shows. The scheduler registers off-site cron entries from the Settings
// column only, so the after-bulk copy must also go by that column; if it
// followed the row, neither side would replicate.
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
	s.FilesOffsiteSchedule = "" // coupled: replicate after each backup
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
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
	// The domain's own repo, as an earlier backup would have left it: a domain
	// whose local repo was never created has nothing to replicate off site.
	filesRepo := filepath.Join(dir, "backups", "files")
	if err := os.MkdirAll(filesRepo, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesRepo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
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

	// The scheduled files job suppresses each set's inline off-site copy so the
	// after-bulk hook replicates the batch once at the end.
	sched := schedule.New(func(string) error { return nil }, st.ListTargets)
	sched.SetFilesJob(func(id string) error {
		ctx := api.WithBulkReplicateSuppressed(
			notify.WithMessagesSuppressed(notify.WithHealthchecksSuppressed(context.Background())))
		_, bErr := svc.BackupFileSet(ctx, id)
		return bErr
	}, st.ListFileSets)
	sched.SetPruneAfterBulkJob(func(domain string) { svc.PruneAfterBulk(context.Background(), domain) })
	sched.SetOffsiteAfterBulkJob(func(domain string) { svc.ReplicateOffsiteAfterBulk(context.Background(), domain) })

	// A stale last run makes the files entry missed, so CatchUpMissed fires the
	// same cron job a real 06:00 trigger would.
	stale := func() (time.Time, error) { return time.Now().Add(-72 * time.Hour), nil }
	if err := sched.ReloadWithDueChecks(mustSettings(t, st), nil, nil, nil, nil, stale, nil); err != nil {
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

	// The activity log shows the replication from this offsite run.
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
		t.Fatalf("no kind=offsite run recorded for the files domain, so the activity log would never show the finished replication; runs=%+v", runs)
	}
}
