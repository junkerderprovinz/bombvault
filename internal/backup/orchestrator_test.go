package backup_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
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
	// createdInspect captures the full profile passed to CreateAndStart so tests
	// can assert the security-relevant fields flow through the DI seam unchanged.
	createdInspect model.Inspect

	// allocations is what Allocations returns (restore pre-flight conflict check).
	allocations []model.Allocation
	allocErr    error

	// execErr, when set, fails the NEXT Exec (for hook tests).
	execErr error
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

func (d *fakeDocker) CreateAndStart(_ context.Context, in model.Inspect) error {
	d.log = append(d.log, "createAndStart:"+in.Name)
	d.createdName = in.Name
	d.createdInspect = in
	return d.createErr
}

func (d *fakeDocker) InspectName(_ context.Context, name string) (string, error) {
	d.log = append(d.log, "inspectName:"+name)
	return d.liveName, d.inspectErr
}

func (d *fakeDocker) Allocations(_ context.Context) ([]model.Allocation, error) {
	d.log = append(d.log, "allocations")
	return d.allocations, d.allocErr
}

func (d *fakeDocker) Exec(_ context.Context, name string, cmd []string) error {
	d.log = append(d.log, "exec:"+name+":"+strings.Join(cmd, " "))
	return d.execErr
}

type fakeRestic struct {
	log []string

	backupErr     error
	restoreErr    error
	summary       backup.Summary
	capturedPaths []string
}

func (r *fakeRestic) Backup(_ context.Context, repo string, paths, tags []string) (backup.Summary, error) {
	r.log = append(r.log, "backup:"+repo+":"+strings.Join(paths, ",")+":"+strings.Join(tags, ","))
	if r.backupErr != nil {
		return backup.Summary{}, r.backupErr
	}
	return r.summary, nil
}

func (r *fakeRestic) RestorePaths(_ context.Context, repo, snapshotID string, paths []string) error {
	r.log = append(r.log, "restore:"+repo+":"+snapshotID+":"+strings.Join(paths, ","))
	r.capturedPaths = paths
	return r.restoreErr
}

type fakeTemplates struct {
	log      []string
	readXML  string
	readOK   bool
	readErr  error
	writeErr error
}

func (t *fakeTemplates) Read(dir, name string) (string, bool, error) {
	t.log = append(t.log, "readTemplate:"+dir+":"+name)
	return t.readXML, t.readOK, t.readErr
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

// sampleInspect returns a rich captured profile with a couple of
// security-relevant fields set, so restore tests can assert they flow through
// the DI seam (CreateAndStart) unchanged.
func sampleInspect() model.Inspect {
	return model.Inspect{
		Name: "/plex",
		// Top-level Image is the image ID (sha256:…), as real Docker reports it —
		// NOT pullable from a registry. The pullable reference is Config.Image.
		Image: "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		Config: model.Config{
			Image: "lscr.io/linuxserver/plex:latest",
			User:  "1000:1000", // SEC: non-root process user
		},
		HostConfig: model.HostConfig{
			CapDrop:        []string{"ALL"},
			CapAdd:         []string{"NET_BIND_SERVICE"},
			Privileged:     false,
			SecurityOpt:    []string{"no-new-privileges:true"},
			ReadonlyRootfs: true,
			NetworkMode:    "bridge",
		},
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

func TestBackupHooksOrderingAndPreHookAbort(t *testing.T) {
	t.Run("pre-hook runs before stop, post-hook after start", func(t *testing.T) {
		d := &fakeDocker{}
		r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678", Bytes: 1024}}
		_, err := backup.BackupContainer(t.Context(), backup.BackupDeps{
			ContainerRef: "plex", ContainerName: "Plex", RepoPath: "/repo",
			AppdataPaths: []string{"/host/user/appdata/plex"}, TargetID: "t1",
			WasRunning: true,
			PreHook:    "echo pre", PostHook: "echo post",
			Docker: d, Restic: r, Templates: &fakeTemplates{}, Runs: &fakeRuns{},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Expect: exec(pre) → stop → start → exec(post).
		idx := func(s string) int {
			for i, e := range d.log {
				if e == s {
					return i
				}
			}
			return -1
		}
		pre := idx("exec:plex:sh -c echo pre")
		stop := idx("stop:plex")
		start := idx("start:plex")
		post := idx("exec:plex:sh -c echo post")
		ordered := pre >= 0 && pre < stop && stop < start && start < post
		if !ordered {
			t.Fatalf("hook ordering wrong: pre=%d stop=%d start=%d post=%d log=%v", pre, stop, start, post, d.log)
		}
	})

	t.Run("pre-hook failure aborts the backup (no stop, no restic)", func(t *testing.T) {
		d := &fakeDocker{execErr: errors.New("dump failed")}
		r := &fakeRestic{}
		runs := &fakeRuns{}
		_, err := backup.BackupContainer(t.Context(), backup.BackupDeps{
			ContainerRef: "plex", ContainerName: "Plex", RepoPath: "/repo",
			AppdataPaths: []string{"/host/user/appdata/plex"}, TargetID: "t1",
			WasRunning: true,
			PreHook:    "false",
			Docker:  d, Restic: r, Templates: &fakeTemplates{}, Runs: runs,
		})
		if err == nil {
			t.Fatal("expected pre-hook failure to abort the backup")
		}
		if contains(d.log, "stop:plex") {
			t.Fatalf("container must NOT be stopped when pre-hook fails: %v", d.log)
		}
		if len(r.log) != 0 {
			t.Fatalf("restic must NOT run when pre-hook fails: %v", r.log)
		}
		if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
			t.Fatalf("run must be recorded failed: %v", runs.finishes)
		}
	})
}

func TestBackupStoppedContainerStaysStopped(t *testing.T) {
	d := &fakeDocker{}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}
	_, err := backup.BackupContainer(t.Context(), backup.BackupDeps{
		ContainerRef: "plex", ContainerName: "Plex", RepoPath: "/repo",
		AppdataPaths: []string{"/host/user/appdata/plex"}, TargetID: "t1",
		WasRunning: false, // already stopped
		Docker:     d, Restic: r, Templates: &fakeTemplates{}, Runs: &fakeRuns{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contains(d.log, "stop:plex") {
		t.Fatalf("a stopped container must NOT be stopped: %v", d.log)
	}
	if d.started {
		t.Fatalf("a stopped container must NOT be started by a backup: %v", d.log)
	}
	if len(r.log) == 0 {
		t.Fatalf("backup must still run on a stopped container: %v", r.log)
	}
}

func TestBackupRunningContainerIsRestarted(t *testing.T) {
	d := &fakeDocker{}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}
	_, err := backup.BackupContainer(t.Context(), backup.BackupDeps{
		ContainerRef: "plex", ContainerName: "Plex", RepoPath: "/repo",
		AppdataPaths: []string{"/p"}, TargetID: "t1",
		WasRunning: true,
		Docker:     d, Restic: r, Templates: &fakeTemplates{}, Runs: &fakeRuns{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(d.log, "stop:plex") || !d.started {
		t.Fatalf("a running container must be stopped then restarted: %v", d.log)
	}
}

func TestBackupHappyPath(t *testing.T) {
	d := &fakeDocker{}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678", Bytes: 1024}}
	tpl := &fakeTemplates{readXML: "<xml/>", readOK: true}
	runs := &fakeRuns{}

	sum, err := backup.BackupContainer(t.Context(), backup.BackupDeps{
		ContainerRef:         "plex",
		ContainerName:        "Plex",
		RepoPath:             "/repo",
		AppdataPaths:         []string{"/host/user/appdata/plex"},
		StopTimeout:          30 * time.Second,
		TargetID:             "target-1",
		WasRunning:           true,
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

	_, err := backup.BackupContainer(t.Context(), backup.BackupDeps{
		ContainerRef:  "plex",
		ContainerName: "Plex",
		RepoPath:      "/repo",
		AppdataPaths:  []string{"/p"},
		TargetID:      "target-1",
		WasRunning:    true,
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

	_, err := backup.BackupContainer(t.Context(), backup.BackupDeps{
		ContainerRef:  "plex",
		ContainerName: "Plex",
		RepoPath:      "/repo",
		AppdataPaths:  []string{"/p"},
		TargetID:      "t",
		WasRunning:    true,
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
		AppdataPaths:      []string{"/host/user/user/appdata/plex"},
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

	err := backup.RestoreContainer(t.Context(), deps)
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

	err := backup.RestoreContainer(t.Context(), deps)
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

	err := backup.RestoreContainer(t.Context(), restoreDeps(d, r, tpl, runs))
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

// restoreDepsWithNet gives the restored container a static IP + a published
// host port so the pre-flight conflict check has something to compare.
func restoreDepsWithNet(d *fakeDocker, r *fakeRestic, tpl *fakeTemplates, runs *fakeRuns) backup.RestoreDeps {
	deps := restoreDeps(d, r, tpl, runs)
	in := deps.Inspect
	in.Network = model.NetworkEndpoint{Name: "br0.20", IPv4Address: "192.168.20.51"}
	in.HostConfig.PortBindings = map[string][]model.PortBinding{
		"80/tcp": {{HostIP: "0.0.0.0", HostPort: "8080"}},
	}
	deps.Inspect = in
	return deps
}

func TestRestoreAbortsOnIPConflict(t *testing.T) {
	d := &fakeDocker{liveName: "", allocations: []model.Allocation{
		{Name: "featherdrop", IPv4: "192.168.20.51"},
	}}
	r := &fakeRestic{}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	err := backup.RestoreContainer(t.Context(), restoreDepsWithNet(d, r, tpl, runs))
	if !errors.Is(err, backup.ErrRestoreConflict) {
		t.Fatalf("expected ErrRestoreConflict, got %v", err)
	}
	if !strings.Contains(err.Error(), "192.168.20.51") || !strings.Contains(err.Error(), "featherdrop") {
		t.Fatalf("error must name the IP and conflicting container: %v", err)
	}
	// Reported BEFORE anything is pulled or destroyed.
	if contains(d.log, "pull:") || contains(d.log, "stop:") || contains(d.log, "remove:") || contains(r.log, "restore:") {
		t.Fatalf("no pull/destructive op allowed on conflict: docker=%v restic=%v", d.log, r.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
}

func TestRestoreAbortsOnPortConflict(t *testing.T) {
	d := &fakeDocker{liveName: "", allocations: []model.Allocation{
		{Name: "nginx", HostPorts: []string{"8080/tcp"}},
	}}
	r := &fakeRestic{}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	err := backup.RestoreContainer(t.Context(), restoreDepsWithNet(d, r, tpl, runs))
	if !errors.Is(err, backup.ErrRestoreConflict) {
		t.Fatalf("expected ErrRestoreConflict, got %v", err)
	}
	if !strings.Contains(err.Error(), "8080/tcp") || !strings.Contains(err.Error(), "nginx") {
		t.Fatalf("error must name the port and conflicting container: %v", err)
	}
	if contains(d.log, "pull:") || contains(d.log, "remove:") {
		t.Fatalf("no pull/destructive op allowed on conflict: %v", d.log)
	}
}

// The container being restored may still exist and hold its own IP/port — that
// is NOT a conflict (it frees up on remove), so the restore must proceed.
func TestRestoreIgnoresOwnAllocation(t *testing.T) {
	d := &fakeDocker{liveName: "/plex", allocations: []model.Allocation{
		{Name: "plex", IPv4: "192.168.20.51", HostPorts: []string{"8080/tcp"}},
	}}
	r := &fakeRestic{}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	err := backup.RestoreContainer(t.Context(), restoreDepsWithNet(d, r, tpl, runs))
	if err != nil {
		t.Fatalf("own allocation must not block restore: %v", err)
	}
	if !contains(d.log, "createAndStart:") {
		t.Fatalf("restore should complete through recreate: %v", d.log)
	}
}

func TestRestoreHappyPathOrder(t *testing.T) {
	d := &fakeDocker{liveName: "/plex"}
	r := &fakeRestic{}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	err := backup.RestoreContainer(t.Context(), restoreDeps(d, r, tpl, runs))
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
	// restic restored each appdata path back to its origin (per-path subtree).
	if len(r.capturedPaths) != 1 || r.capturedPaths[0] != "/host/user/user/appdata/plex" {
		t.Fatalf("restored paths = %v, want [/host/user/user/appdata/plex]", r.capturedPaths)
	}
	// template written to flash dir
	if !contains(tpl.log, "writeTemplate:/boot/templates:plex") {
		t.Fatalf("template not flashed: %v", tpl.log)
	}
	// success recorded
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("run finishes = %v, want [success]", runs.finishes)
	}

	// SEC §8 / C1: the FULL captured profile (incl. security-relevant fields)
	// must reach CreateAndStart unchanged — proving the DI seam no longer drops
	// User/Cap*/Privileged/SecurityOpt/ReadonlyRootfs/NetworkMode.
	sent := d.createdInspect
	orig := sampleInspect()
	if sent.Config.User != orig.Config.User {
		t.Fatalf("User dropped at seam: got %q want %q", sent.Config.User, orig.Config.User)
	}
	if !equalStrings(sent.HostConfig.CapDrop, orig.HostConfig.CapDrop) {
		t.Fatalf("CapDrop dropped at seam: got %v want %v", sent.HostConfig.CapDrop, orig.HostConfig.CapDrop)
	}
	if !equalStrings(sent.HostConfig.CapAdd, orig.HostConfig.CapAdd) {
		t.Fatalf("CapAdd dropped at seam: got %v want %v", sent.HostConfig.CapAdd, orig.HostConfig.CapAdd)
	}
	if !equalStrings(sent.HostConfig.SecurityOpt, orig.HostConfig.SecurityOpt) {
		t.Fatalf("SecurityOpt dropped at seam: got %v want %v", sent.HostConfig.SecurityOpt, orig.HostConfig.SecurityOpt)
	}
	if sent.HostConfig.Privileged != orig.HostConfig.Privileged {
		t.Fatalf("Privileged dropped at seam: got %v want %v", sent.HostConfig.Privileged, orig.HostConfig.Privileged)
	}
	if sent.HostConfig.ReadonlyRootfs != orig.HostConfig.ReadonlyRootfs {
		t.Fatalf("ReadonlyRootfs dropped at seam: got %v want %v", sent.HostConfig.ReadonlyRootfs, orig.HostConfig.ReadonlyRootfs)
	}
	if sent.HostConfig.NetworkMode != orig.HostConfig.NetworkMode {
		t.Fatalf("NetworkMode dropped at seam: got %q want %q", sent.HostConfig.NetworkMode, orig.HostConfig.NetworkMode)
	}
}

// equalStrings reports whether two string slices are element-wise equal.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRestoreProceedsWhenLiveAbsent(t *testing.T) {
	d := &fakeDocker{liveName: ""} // absent
	r := &fakeRestic{}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	if err := backup.RestoreContainer(t.Context(), restoreDeps(d, r, tpl, runs)); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !contains(r.log, "restore:") {
		t.Fatalf("restore must proceed for a fresh restore: %v", r.log)
	}
}

func TestRestoreSkipsEmptyTemplate(t *testing.T) {
	d := &fakeDocker{liveName: "/plex"}
	r := &fakeRestic{}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	deps := restoreDeps(d, r, tpl, runs)
	deps.TemplateXML = "" // no template captured (e.g. backup before flash mount)

	if err := backup.RestoreContainer(t.Context(), deps); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	// restore still proceeds, but no (empty) template is flashed.
	if !contains(r.log, "restore:") {
		t.Fatalf("restore must proceed: %v", r.log)
	}
	for _, e := range tpl.log {
		if strings.HasPrefix(e, "writeTemplate:") {
			t.Fatalf("must NOT write an empty template: %v", tpl.log)
		}
	}
}

func TestRestoreIgnoresStopRemoveErrors(t *testing.T) {
	d := &fakeDocker{liveName: "/plex", stopErr: errors.New("no such container"), removeErr: errors.New("no such container")}
	r := &fakeRestic{}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	if err := backup.RestoreContainer(t.Context(), restoreDeps(d, r, tpl, runs)); err != nil {
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

	err := backup.RestoreContainer(t.Context(), restoreDeps(d, r, tpl, runs))
	if err == nil {
		t.Fatal("expected error")
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
	_ = combinedLog(d, r, tpl, runs) // touch helper
}

// I1 / SEC: an unsafe appdata path (traversal / non-absolute) must be rejected
// before any destructive step runs, and the run recorded failed.
func TestRestoreRejectsUnsafePath(t *testing.T) {
	d := &fakeDocker{liveName: "/plex"}
	r := &fakeRestic{}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	deps := restoreDeps(d, r, tpl, runs)
	deps.AppdataPaths = []string{"/host/user/user/appdata/../../etc"}

	err := backup.RestoreContainer(t.Context(), deps)
	if err == nil || !strings.Contains(err.Error(), "SEC") {
		t.Fatalf("expected unsafe-path rejection, got %v", err)
	}
	// nothing destructive may have run
	if contains(d.log, "stop:") || contains(d.log, "remove:") || contains(r.log, "restore:") || contains(d.log, "createAndStart:") {
		t.Fatalf("destructive op ran for a non-root target: docker=%v restic=%v", d.log, r.log)
	}
	// a run was started (guard is inside runRestore) and recorded failed
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
}

// I2: even when Stop fails, the container must still be Started and the error
// returned/recorded — Backup ALWAYS restarts the container.
func TestBackupAlwaysStartsOnStopFailure(t *testing.T) {
	d := &fakeDocker{stopErr: errors.New("stop boom")}
	r := &fakeRestic{}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	_, err := backup.BackupContainer(t.Context(), backup.BackupDeps{
		ContainerRef:  "plex",
		ContainerName: "Plex",
		RepoPath:      "/repo",
		AppdataPaths:  []string{"/p"},
		TargetID:      "target-1",
		WasRunning:    true,
		Docker:        d,
		Restic:        r,
		Templates:     tpl,
		Runs:          runs,
	})
	if err == nil || !strings.Contains(err.Error(), "stop container") {
		t.Fatalf("expected the stop error to be returned, got %v", err)
	}
	if !d.started {
		t.Fatal("container must be restarted even when Stop fails")
	}
	// restic backup must NOT have run after a failed stop
	if contains(r.log, "backup:") {
		t.Fatalf("backup must not run after a failed stop: %v", r.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
}

// I3: a real template-read error must NOT fail the backup (the snapshot is
// valid) but also must NOT be silently swallowed — the backup still succeeds,
// no template is persisted, and the run is recorded success.
func TestBackupSucceedsWhenTemplateReadErrors(t *testing.T) {
	d := &fakeDocker{}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "abcd1234", Bytes: 1}}
	tpl := &fakeTemplates{readErr: errors.New("permission denied")}
	runs := &fakeRuns{}

	sum, err := backup.BackupContainer(t.Context(), backup.BackupDeps{
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
	if err != nil {
		t.Fatalf("template read error must not fail the backup: %v", err)
	}
	if sum.SnapshotID != "abcd1234" {
		t.Fatalf("snapshot id = %q", sum.SnapshotID)
	}
	// no template persisted when the read failed
	if contains(tpl.log, "writeTemplate:") {
		t.Fatalf("no template must be persisted on a read error: %v", tpl.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("run finishes = %v, want [success]", runs.finishes)
	}
}

// M2: the restore guards return sentinel errors the API layer can match with
// errors.Is.
func TestRestoreGuardSentinels(t *testing.T) {
	d := &fakeDocker{liveName: "/plex"}
	r := &fakeRestic{}
	tpl := &fakeTemplates{}
	runs := &fakeRuns{}

	notConfirmed := restoreDeps(d, r, tpl, runs)
	notConfirmed.Confirmed = false
	if err := backup.RestoreContainer(t.Context(), notConfirmed); !errors.Is(err, backup.ErrNotConfirmed) {
		t.Fatalf("expected ErrNotConfirmed, got %v", err)
	}

	badID := restoreDeps(d, r, tpl, runs)
	badID.SnapshotID = "; rm -rf /"
	if err := backup.RestoreContainer(t.Context(), badID); !errors.Is(err, backup.ErrInvalidSnapshotID) {
		t.Fatalf("expected ErrInvalidSnapshotID, got %v", err)
	}
}
