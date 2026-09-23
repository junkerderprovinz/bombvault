package api_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Deleting old backups is the one thing that cannot be undone, so a finding
// that says the source lost its data pauses it for that series alone, until
// the user has seen the finding.

const (
	holdGiB = 1 << 30
	holdMiB = 1 << 20
)

// measuredSummary is what restic reports for a backup that read sourceBytes and
// stored added of it.
func measuredSummary(sourceBytes uint64, added float64) restic.Summary {
	secs := 60.0
	return restic.Summary{
		SnapshotID:          "deadbeef12345678",
		BytesAdded:          added,
		FilesNew:            4,
		TotalBytesProcessed: sourceBytes,
		TotalFilesProcessed: 900,
		TotalDuration:       &secs,
	}
}

func holdDocker(names ...string) *fakeServiceDocker {
	d := &fakeServiceDocker{inspects: map[string]model.Inspect{}}
	for _, name := range names {
		d.inspects[name] = model.Inspect{
			Name: "/" + name, Image: name + ":latest",
			Mounts: []model.Mount{{Type: "bind", Source: "/host/appdata/" + name, Destination: "/config"}},
		}
	}
	return d
}

// holdService is a containers domain with a keep-policy and an appdata folder
// per container, so a backup has a source and the retention pass after it has
// something to forget.
func holdService(t *testing.T, eng *fakeResticEngine, names ...string) (*api.Service, *store.Repo, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		if err := os.MkdirAll(filepath.Join(dir, "appdata", name), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Config{
		AppKey: strings.Repeat("a", 64), DataDir: dir,
		HostMountRoot: filepath.ToSlash(dir), DataRootSegments: []string{"appdata"},
	}
	st, db := newMemStoreDB(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	s.RetentionKeepLast = 5
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	return api.NewService(cfg, st, holdDocker(names...), fakeVirsh{}, eng), st, db
}

// seedSteadySeries writes n daily backups of one size, the newest a day back,
// and returns the container's target id.
func seedSteadySeries(t *testing.T, st *store.Repo, db *sql.DB, name string, n int, sourceBytes int64) string {
	t.Helper()
	target, err := st.UpsertTarget(store.Target{ContainerName: name, IncludeInSchedule: true})
	if err != nil {
		t.Fatal(err)
	}
	parent := true
	now := time.Now().Unix()
	for i := n; i >= 1; i-- {
		id, sErr := st.StartRun(target.ID, "backup")
		if sErr != nil {
			t.Fatal(sErr)
		}
		m := store.RunMetrics{SourceBytes: sourceBytes, SourceFiles: 900, FilesNew: 4, ResticMS: 60_000, HasParent: &parent}
		if fErr := st.FinishRunMeasured(id, "success", "snap-"+id, sourceBytes/100, "", &m, ""); fErr != nil {
			t.Fatal(fErr)
		}
		at := now - int64(i)*86400
		if _, eErr := db.Exec(`UPDATE runs SET started_at = ?, finished_at = ? WHERE id = ?`, at, at+60, id); eErr != nil {
			t.Fatal(eErr)
		}
	}
	return target.ID
}

func backupOnce(t *testing.T, svc *api.Service, name string) {
	t.Helper()
	if _, err := svc.Backup(context.Background(), name); err != nil {
		t.Fatalf("backup %s: %v", name, err)
	}
}

// forgotSince reports whether tag was forgotten after the mark, so a test can
// ask about one retention pass instead of the whole run of them.
func forgotSince(eng *fakeResticEngine, mark int, tag string) bool {
	for _, group := range eng.forgetTags[mark:] {
		if slices.Contains(strings.Split(group, ","), tag) {
			return true
		}
	}
	return false
}

func heldRows(t *testing.T, svc *api.Service) []api.AnomalyView {
	t.Helper()
	page, err := svc.ListAnomalies(context.Background(), store.AnomalyFilter{})
	if err != nil {
		t.Fatalf("list anomalies: %v", err)
	}
	var out []api.AnomalyView
	for _, row := range page.Anomalies {
		if row.RetentionHeld {
			out = append(out, row)
		}
	}
	return out
}

func onlyHeldRow(t *testing.T, svc *api.Service) api.AnomalyView {
	t.Helper()
	rows := heldRows(t, svc)
	if len(rows) != 1 {
		t.Fatalf("want one finding holding retention, got %d: %+v", len(rows), rows)
	}
	return rows[0]
}

func TestRetentionSkipsItemWithOpenSourceCollapse(t *testing.T) {
	eng := &fakeResticEngine{backupSummaries: []restic.Summary{
		measuredSummary(40*holdGiB, 400*holdMiB),
		measuredSummary(20*holdMiB, 200*1024),
		measuredSummary(20*holdMiB, 200*1024),
		measuredSummary(40*holdGiB, 400*holdMiB),
		measuredSummary(20*holdMiB, 200*1024),
	}}
	svc, _, _ := holdService(t, eng, "plex")

	backupOnce(t, svc, "plex")
	if !forgotSince(eng, 0, "container:plex") {
		t.Fatal("the first backup must forget as usual, or nothing below is proved")
	}

	mark := len(eng.forgetTags)
	backupOnce(t, svc, "plex")
	if forgotSince(eng, mark, "container:plex") {
		t.Fatalf("a collapsed source must pause retention, forgot %v", eng.forgetTags[mark:])
	}
	row := onlyHeldRow(t, svc)
	if row.Metric != "source_bytes_shrink" || row.Severity != "critical" {
		t.Fatalf("want a critical source_bytes_shrink, got %s/%s", row.Metric, row.Severity)
	}

	if _, _, err := svc.AcknowledgeAnomalies(context.Background(), []string{row.ID}, "clearing it out"); err != nil {
		t.Fatalf("acknowledge: %v", err)
	}
	mark = len(eng.forgetTags)
	backupOnce(t, svc, "plex")
	if !forgotSince(eng, mark, "container:plex") {
		t.Fatal("an acknowledged episode holds nothing, so retention has to run again")
	}
	if rows := heldRows(t, svc); len(rows) != 0 {
		t.Fatalf("the same collapse must not open a second episode, got %+v", rows)
	}

	backupOnce(t, svc, "plex") // the source is back
	mark = len(eng.forgetTags)
	backupOnce(t, svc, "plex") // and gone again
	if forgotSince(eng, mark, "container:plex") {
		t.Fatalf("a new collapse is a new episode and holds again, forgot %v", eng.forgetTags[mark:])
	}
	if row := onlyHeldRow(t, svc); row.Metric != "source_bytes_shrink" {
		t.Fatalf("want the collapse raised again, got %s", row.Metric)
	}
}

func TestRewriteHoldsRetentionUntilTheUserActs(t *testing.T) {
	settle := map[string]func(*api.Service, []string) error{
		"acknowledge": func(svc *api.Service, ids []string) error {
			_, _, err := svc.AcknowledgeAnomalies(context.Background(), ids, "")
			return err
		},
		"mark as expected": func(svc *api.Service, ids []string) error {
			_, _, err := svc.MarkAnomaliesExpected(context.Background(), ids, "")
			return err
		},
	}
	for name, act := range settle {
		t.Run(name, func(t *testing.T) {
			eng := &fakeResticEngine{backupSummaries: []restic.Summary{
				measuredSummary(40*holdGiB, 30*holdGiB),
				measuredSummary(40*holdGiB, 400*holdMiB),
				measuredSummary(40*holdGiB, 400*holdMiB),
			}}
			svc, st, db := holdService(t, eng, "plex", "sonarr")
			seedSteadySeries(t, st, db, "plex", 12, 40*holdGiB)

			mark := len(eng.forgetTags)
			backupOnce(t, svc, "plex")
			backupOnce(t, svc, "sonarr")

			if forgotSince(eng, mark, "container:plex") {
				t.Fatalf("a backup that rewrote most of the source must pause retention, forgot %v", eng.forgetTags[mark:])
			}
			if !forgotSince(eng, mark, "container:sonarr") {
				t.Fatal("the hold belongs to one series: another container keeps forgetting")
			}
			row := onlyHeldRow(t, svc)
			if row.Metric != "new_data_rewrite" || row.Severity != "critical" {
				t.Fatalf("want a critical new_data_rewrite, got %s/%s", row.Metric, row.Severity)
			}
			if row.LastGoodRun == "" {
				t.Fatal("a data-loss finding has to name the backup to restore from")
			}

			if err := act(svc, []string{row.ID}); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			mark = len(eng.forgetTags)
			backupOnce(t, svc, "plex")
			if !forgotSince(eng, mark, "container:plex") {
				t.Fatalf("%s releases the hold, so the next retention pass has to run", name)
			}
		})
	}
}

// A rewrite the user called normal fixes a ceiling, so the same amount written
// again is no longer a finding.
func TestExpectedRewriteIsNotRaisedAgain(t *testing.T) {
	rewrite := measuredSummary(40*holdGiB, 30*holdGiB)
	eng := &fakeResticEngine{backupSummaries: []restic.Summary{rewrite, rewrite}}
	svc, st, db := holdService(t, eng, "plex")
	seedSteadySeries(t, st, db, "plex", 12, 40*holdGiB)

	backupOnce(t, svc, "plex")
	row := onlyHeldRow(t, svc)
	if _, _, err := svc.MarkAnomaliesExpected(context.Background(), []string{row.ID}, ""); err != nil {
		t.Fatalf("mark as expected: %v", err)
	}

	time.Sleep(time.Second) // the second rewrite has to be a run of its own
	backupOnce(t, svc, "plex")
	if rows := heldRows(t, svc); len(rows) != 0 {
		t.Fatalf("a rewrite under the accepted ceiling must raise nothing, got %+v", rows)
	}
}

func TestRetentionHoldOffOrDetectionOff(t *testing.T) {
	for name, tc := range map[string]struct {
		off         func(*store.Settings)
		wantForgets bool
	}{
		"the hold is switched off":     {func(s *store.Settings) { s.AnomalyRetentionHold = false }, true},
		"detection is switched off":    {func(s *store.Settings) { s.AnomalyEnabled = false }, true},
		"both switches are left alone": {func(*store.Settings) {}, false},
	} {
		t.Run(name, func(t *testing.T) {
			eng := &fakeResticEngine{backupSummaries: []restic.Summary{
				measuredSummary(40*holdGiB, 400*holdMiB),
				measuredSummary(20*holdMiB, 200*1024),
			}}
			svc, st, _ := holdService(t, eng, "plex")
			if _, err := st.MutateSettings(func(s *store.Settings) error {
				tc.off(s)
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			backupOnce(t, svc, "plex")
			mark := len(eng.forgetTags)
			backupOnce(t, svc, "plex")

			if forgot := forgotSince(eng, mark, "container:plex"); forgot != tc.wantForgets {
				t.Fatalf("forgot container:plex = %v, want %v", forgot, tc.wantForgets)
			}
		})
	}
}

// heldRepo is a files domain on one repository that also holds another
// domain's snapshots, which is what a named repository looks like (#204).
func heldRepo(t *testing.T, eng *fakeResticEngine) (*api.Service, *store.Repo, http.Handler) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.FilesPath = "backups/shared"
	s.ContainersPath = "backups/shared"
	s.RetentionKeepLast = 5
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "backups", "shared")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	eng.snaps = []restic.Snapshot{
		snapTagged("a1", "fileset:docs"),
		snapTagged("b2", "container:plex"),
		snapTagged("c3", "vm:win11"),
		snapTagged("d4", "vm:win11:zvol:vda"),
	}

	if _, err := st.CreateFileSet(store.FileSet{Name: "docs", Path: "docs", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	target, err := st.UpsertTarget(store.Target{ContainerName: "plex", IncludeInSchedule: true})
	if err != nil {
		t.Fatal(err)
	}
	vm, err := st.UpsertVMTarget(store.VMTarget{Name: "win11", IncludeInSchedule: true})
	if err != nil {
		t.Fatal(err)
	}
	seedHold(t, st, "item", target.ID, "container")
	seedHold(t, st, "item", vm.ID, "vm")

	d := &fakeServiceDocker{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)
	sched := schedule.New(func(string) error { return nil }, st.ListTargets)
	h := api.NewHandler(cfg, st, d, svc, sched, spike.DefaultProbes())
	return svc, st, h.Router()
}

func snapTagged(id string, tags ...string) restic.Snapshot {
	return restic.Snapshot{ID: id, Time: "2026-09-15T02:00:00.000000000+02:00", Tags: tags}
}

// seedHold writes the one finding that pauses retention for a series, so a
// test about the pass itself does not have to drive a whole backup history.
func seedHold(t *testing.T, st *store.Repo, scopeKind, targetID, domain string) {
	t.Helper()
	now := time.Now().Unix()
	if _, err := st.ApplyAnomalyChanges(store.AnomalyChanges{Now: now, Insert: []store.Anomaly{{
		Fingerprint: "source|" + scopeKind + "|" + targetID + "|source_bytes_shrink",
		Detector:    "source", Metric: "source_bytes_shrink", Severity: "critical",
		ScopeKind: scopeKind, ScopeID: targetID, TargetID: targetID, Domain: domain,
		FirstSeenAt: now, LastSeenAt: now, LastRunAt: now,
	}}}); err != nil {
		t.Fatalf("seed a held finding: %v", err)
	}
}

func TestPruneSkipsHeldIdentitiesAcrossDomains(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, _, _ := heldRepo(t, eng)

	paused, err := svc.PruneDomain(context.Background(), "files", "")
	if err != nil {
		t.Fatalf("PruneDomain: %v", err)
	}
	if !forgotSince(eng, 0, "fileset:docs") {
		t.Fatalf("the folder set is not held and has to be forgotten, got %v", eng.forgetTags)
	}
	for _, tag := range []string{"container:plex", "vm:win11", "vm:win11:zvol:vda"} {
		if forgotSince(eng, 0, tag) {
			t.Fatalf("%s is held and must not be forgotten, got %v", tag, eng.forgetTags)
		}
		if !slices.Contains(paused, tag) {
			t.Fatalf("the prune has to name %s as paused, got %v", tag, paused)
		}
	}
}

// Without the identity tags the pass falls back to forgetting the whole
// repository, which would take the held items with it.
func TestPruneRefusesTheRepoWidePassWhileAnythingIsHeld(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, _, _ := heldRepo(t, eng)
	eng.snaps = nil

	if _, err := svc.PruneDomain(context.Background(), "files", ""); err == nil {
		t.Fatal("want an error naming the pause")
	}
	if len(eng.forgetTags) != 0 {
		t.Fatalf("nothing may be forgotten, got %v", eng.forgetTags)
	}
}

func TestManualPruneNamesPausedItems(t *testing.T) {
	eng := &fakeResticEngine{}
	_, _, h := heldRepo(t, eng)

	w, m := doJSON(t, h, http.MethodPost, "/api/prune/files", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("status %d body %v", w.Code, m)
	}
	raw, err := json.Marshal(m["paused"])
	if err != nil {
		t.Fatal(err)
	}
	var paused []string
	if err := json.Unmarshal(raw, &paused); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(paused, "container:plex") {
		t.Fatalf("the response has to name the paused items, got %v", m)
	}
}
