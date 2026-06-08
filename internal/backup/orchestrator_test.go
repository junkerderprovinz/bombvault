package backup_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

// ---------------------------------------------------------------------------
// Fakes (DI seam — no real docker.sock, restic binary, or DB)
// ---------------------------------------------------------------------------

type fakeDocker struct {
	log []string

	stopErr   error
	startErr  error
	pullErr   error
	createErr error
	removeErr error

	// liveName is what InspectName returns; "" means absent (no such container).
	liveName    string
	inspectErr  error
	started     bool
	createdName string
}

func (d *fakeDocker) Stop(_ context.Context, name string, _ time.Duration) error {
	d.log = append(d.log, "stop:"+name)
	return d.stopErr
}

func (d *fakeDocker) Start(_ context.Context, name string) error {
	d.log = append(d.log, "start:"+name)
	d.started = true
	return d.startErr
}

func (d *fakeDocker) Remove(_ context.Context, name string) error {
	d.log = append(d.log, "remove:"+name)
	return d.removeErr
}

func (d *fakeDocker) Pull(_ context.Context, image string) error {
	d.log = append(d.log, "pull:"+image)
	return d.pullErr
}

func (d *fakeDocker) CreateAndStart(_ context.Context, in backup.Inspect) error {
	d.log = append(d.log, "createAndStart:"+in.Name)
	d.createdName = in.Name
	return d.createErr
}

func (d *fakeDocker) InspectName(_ context.Context, name string) (string, error) {
	d.log = append(d.log, "inspectName:"+name)
	return d.liveName, d.inspectErr
}

type fakeRestic struct {
	log []string

	backupErr      error
	restoreErr     error
	summary        backup.Summary
	capturedTarget string
}

func (r *fakeRestic) Backup(_ context.Context, repo string, paths, tags []string) (backup.Summary, error) {
	r.log = append(r.log, "backup:"+repo+":"+strings.Join(paths, ",")+":"+strings.Join(tags, ","))
	if r.backupErr != nil {
		return backup.Summary{}, r.backupErr
	}
	return r.summary, nil
}

func (r *fakeRestic) Restore(_ context.Context, repo, snapshotID, target string) error {
	r.log = append(r.log, "restore:"+repo+":"+snapshotID+":"+target)
	r.capturedTarget = target
	return r.restoreErr
}

type fakeTemplates struct {
	log      []string
	readXML  string
	readOK   bool
	writeErr error
}

func (t *fakeTemplates) Read(dir, name string) (string, bool) {
	t.log = append(t.log, "readTemplate:"+dir+":"+name)
	return t.readXML, t.readOK
}

func (t *fakeTemplates) Write(dir, name, xml string) error {
	t.log = append(t.log, "writeTemplate:"+dir+":"+name)
	return t.writeErr
}

type fakeRuns struct {
	log       []string
	startErr  error
	finishErr error
	lastRunID string
	finishes  []string // recorded "status" values
}

func (r *fakeRuns) Start(targetID, kind string) (string, error) {
	r.log = append(r.log, "runStart:"+targetID+":"+kind)
	r.lastRunID = "run-1"
	return r.lastRunID, r.startErr
}

func (r *fakeRuns) Finish(runID, status, snapshotID string, bytes int64, errMsg string) error {
	entry := "runFinish:" + runID + ":" + status
	if errMsg != "" {
		entry += ":" + errMsg
	}
	r.log = append(r.log, entry)
	r.finishes = append(r.finishes, status)
	return r.finishErr
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

var ctx = context.Background()

func sampleInspect() backup.Inspect {
	return backup.Inspect{
		Name:  "/plex",
		Image: "lscr.io/linuxserver/plex:latest",
	}
}

func combinedLog(d *fakeDocker, r *fakeRestic, tpl *fakeTemplates, runs *fakeRuns) []string {
	// Not order-preserving across fakes; callers use per-fake logs for ordering.
	var all []string
	all = append(all, runs.log...)
	all = append(all, d.log...)
	all = append(all, r.log...)
	all = append(all, tpl.log...)
	return all
}

func contains(log []string, prefix string) bool {
	for _, e := range log {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// BackupContainer
// ---------------------------------------------------------------------------

func TestBackupHappyPath(t *testing.T) {
	d := &fakeDocker{}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678", Bytes: 1024}}
	tpl := &fakeTemplates{readXML: "<xml/>", readOK: true}
	runs := &fakeRuns{}

	sum, err := backup.BackupContainer(ctx, backup.BackupDeps{
		ContainerRef:         "plex",
		ContainerName:        "Plex",
		RepoPath:             "/repo",
		AppdataPaths:         []string{"/host/user/appdata/plex"},
		StopTimeout:          30 * time.Second,
		TargetID:             "target-1",
		SnapshotTemplatesDir: "/data/templates",
		FlashTemplatesDir:    "/boot/templates",
		Docker:               d,
		Restic:               r,
		Templates:            tpl,
		Runs:                 runs,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum.SnapshotID != "deadbeef12345678" {
		t.Fatalf("snapshot id = %q", sum.SnapshotID)
	}

	// Order within docker fake: stop precedes start.
	if d.log[0] != "stop:plex" {
		t.Fatalf("docker log[0] = %q, want stop:plex", d.log[0])
	}
	if !d.started {
		t.Fatalf("container must be started")
	}
	// restic backup tagged container:plex
	if !contains(r.log, "backup:/repo") || !strings.Contains(r.log[0], "container:plex") {
		t.Fatalf("restic backup tag missing: %v", r.log)
	}
	// template read + write
	if !contains(tpl.log, "readTemplate:") || !contains(tpl.log, "writeTemplate:") {
		t.Fatalf("template not captured: %v", tpl.log)
	}
	// run recorded success
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("run finishes = %v, want [success]", runs.finishes)
	}
}

func TestBackupAlwaysStarts(t *testing.T) {
	d := &fakeDocker{}
	r := &fakeRestic{backupErr: errors.New("boom")}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	_, err := backup.BackupContainer(ctx, backup.BackupDeps{
		ContainerRef:  "plex",
		ContainerName: "Plex",
		RepoPath:      "/repo",
		AppdataPaths:  []string{"/p"},
		TargetID:      "target-1",
		Docker:        d,
		Restic:        r,
		Templates:     tpl,
		Runs:          runs,
	})
	if err == nil {
		t.Fatal("expected error to be re-thrown")
	}
	if !d.started {
		t.Fatal("container must be restarted even on backup failure")
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
}

func TestBackupNoTemplateWriteWhenAbsent(t *testing.T) {
	d := &fakeDocker{}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "abcd1234", Bytes: 1}}
	tpl := &fakeTemplates{readOK: false} // template absent
	runs := &fakeRuns{}

	_, err := backup.BackupContainer(ctx, backup.BackupDeps{
		ContainerRef:  "plex",
		ContainerName: "Plex",
		RepoPath:      "/repo",
		AppdataPaths:  []string{"/p"},
		TargetID:      "t",
		Docker:        d,
		Restic:        r,
		Templates:     tpl,
		Runs:          runs,
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if contains(tpl.log, "writeTemplate:") {
		t.Fatalf("writeTemplate must not be called when template absent: %v", tpl.log)
	}
}

// ---------------------------------------------------------------------------
// RestoreContainer
// ---------------------------------------------------------------------------

func restoreDeps(d *fakeDocker, r *fakeRestic, tpl *fakeTemplates, runs *fakeRuns) backup.RestoreDeps {
	return backup.RestoreDeps{
		Confirmed:         true,
		ContainerRef:      "plex",
		ContainerName:     "plex",
		RepoPath:          "/repo",
		SnapshotID:        "deadbeef12345678",
		RestoreTargetDir:  "/",
		TemplateXML:       "<xml>restored</xml>",
		FlashTemplatesDir: "/boot/templates",
		Inspect:           sampleInspect(),
		TargetID:          "target-1",
		Docker:            d,
		Restic:            r,
		Templates:         tpl,
		Runs:              runs,
	}
}

func TestRestoreAbortsWhenNotConfirmed(t *testing.T) {
	d := &fakeDocker{liveName: "/plex"}
	r := &fakeRestic{}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	deps := restoreDeps(d, r, tpl, runs)
	deps.Confirmed = false

	err := backup.RestoreContainer(ctx, deps)
	if err == nil || !strings.Contains(err.Error(), "confirm") {
		t.Fatalf("expected confirm error, got %v", err)
	}
	// No run started, nothing destructive happened.
	if contains(runs.log, "runStart:") {
		t.Fatal("runStart must NOT be called when not confirmed")
	}
	if contains(d.log, "stop:") || contains(d.log, "remove:") {
		t.Fatalf("no destructive op allowed when not confirmed: %v", d.log)
	}
}

func TestRestoreRejectsBadSnapshotID(t *testing.T) {
	d := &fakeDocker{liveName: "/plex"}
	r := &fakeRestic{}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	deps := restoreDeps(d, r, tpl, runs)
	deps.SnapshotID = "; rm -rf /" // not hex

	err := backup.RestoreContainer(ctx, deps)
	if err == nil || !strings.Contains(err.Error(), "snapshot") {
		t.Fatalf("expected snapshot-id validation error, got %v", err)
	}
	if contains(d.log, "stop:") || contains(d.log, "remove:") || contains(r.log, "restore:") {
		t.Fatalf("nothing destructive allowed for a bad snapshot id: docker=%v restic=%v", d.log, r.log)
	}
}

func TestRestoreAbortsOnLiveNameMismatch(t *testing.T) {
	d := &fakeDocker{liveName: "/some-other-container"}
	r := &fakeRestic{}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	err := backup.RestoreContainer(ctx, restoreDeps(d, r, tpl, runs))
	if err == nil || !strings.Contains(err.Error(), "does not match target") {
		t.Fatalf("expected wrong-target abort, got %v", err)
	}
	// destructive steps must NOT have run
	if contains(d.log, "stop:") || contains(d.log, "remove:") || contains(r.log, "restore:") {
		t.Fatalf("destructive op ran despite mismatch: docker=%v restic=%v", d.log, r.log)
	}
	// the run was recorded failed
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
}

func TestRestoreHappyPathOrder(t *testing.T) {
	d := &fakeDocker{liveName: "/plex"}
	r := &fakeRestic{}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	err := backup.RestoreContainer(ctx, restoreDeps(d, r, tpl, runs))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// runStart first
	if runs.log[0] != "runStart:target-1:restore" {
		t.Fatalf("runs.log[0] = %q", runs.log[0])
	}
	// docker order: inspectName → pull → stop → remove → createAndStart
	want := []string{"inspectName:plex", "pull:lscr.io/linuxserver/plex:latest", "stop:plex", "remove:plex", "createAndStart:/plex"}
	if len(d.log) != len(want) {
		t.Fatalf("docker log = %v, want %v", d.log, want)
	}
	for i := range want {
		if d.log[i] != want[i] {
			t.Fatalf("docker log[%d] = %q, want %q (full %v)", i, d.log[i], want[i], d.log)
		}
	}
	// restic restore target is "/"
	if r.capturedTarget != "/" {
		t.Fatalf("restore target = %q, want /", r.capturedTarget)
	}
	// template written to flash dir
	if !contains(tpl.log, "writeTemplate:/boot/templates:plex") {
		t.Fatalf("template not flashed: %v", tpl.log)
	}
	// success recorded
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("run finishes = %v, want [success]", runs.finishes)
	}
}

func TestRestoreProceedsWhenLiveAbsent(t *testing.T) {
	d := &fakeDocker{liveName: ""} // absent
	r := &fakeRestic{}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	if err := backup.RestoreContainer(ctx, restoreDeps(d, r, tpl, runs)); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !contains(r.log, "restore:") {
		t.Fatalf("restore must proceed for a fresh restore: %v", r.log)
	}
}

func TestRestoreIgnoresStopRemoveErrors(t *testing.T) {
	d := &fakeDocker{liveName: "/plex", stopErr: errors.New("no such container"), removeErr: errors.New("no such container")}
	r := &fakeRestic{}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	if err := backup.RestoreContainer(ctx, restoreDeps(d, r, tpl, runs)); err != nil {
		t.Fatalf("stop/remove errors must be ignored, got %v", err)
	}
	if !contains(r.log, "restore:") {
		t.Fatalf("restore must proceed after stop/remove failure: %v", r.log)
	}
}

func TestRestoreRecordsFailedWhenResticThrows(t *testing.T) {
	d := &fakeDocker{liveName: "/plex"}
	r := &fakeRestic{restoreErr: errors.New("restic restore failed")}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	err := backup.RestoreContainer(ctx, restoreDeps(d, r, tpl, runs))
	if err == nil {
		t.Fatal("expected error")
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
	_ = combinedLog(d, r, tpl, runs) // touch helper
}
