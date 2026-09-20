package backup_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// fakeDumper writes into the shared fakeDocker log, so the dump's place in the
// stop/start sequence can be read off one list.
type fakeDumper struct {
	docker *fakeDocker
	res    backup.DBDumpResult
	err    error
	// onDump runs inside Dump, e.g. to cancel the backup's context mid-dump.
	onDump func()

	reqs []backup.DBDumpRequest
}

func (f *fakeDumper) Dump(_ context.Context, req backup.DBDumpRequest) (backup.DBDumpResult, error) {
	f.docker.log = append(f.docker.log, "dbdump")
	f.reqs = append(f.reqs, req)
	if f.onDump != nil {
		f.onDump()
	}
	if f.err != nil {
		return backup.DBDumpResult{}, f.err
	}
	return f.res, nil
}

func dumpPlan() backup.DBDumpPlan {
	return backup.DBDumpPlan{
		Engine:     "postgres",
		Identity:   "dbdump:pg",
		StdinPath:  "/dbdump/pg.sql",
		Image:      "postgres:16",
		MaxRuntime: 6 * time.Hour,
	}
}

// dumpDeps is a running database container whose backup takes a dump.
func dumpDeps(d *fakeDocker, r *fakeRestic, runs *fakeRuns, dumper backup.DBDumper) backup.BackupDeps {
	plan := dumpPlan()
	return backup.BackupDeps{
		ContainerRef:  "pg",
		ContainerName: "pg",
		RepoPath:      "/repo",
		AppdataPaths:  []string{"/host/user/appdata/pg"},
		TargetID:      "t1",
		WasRunning:    true,
		DBDump:        &plan,
		DBDumper:      dumper,
		Docker:        d,
		Restic:        r,
		Templates:     &fakeTemplates{},
		Runs:          runs,
	}
}

func dumpFakes(t *testing.T) (*fakeDocker, *fakeRestic, *fakeRuns, *fakeDumper) {
	t.Helper()
	d := &fakeDocker{}
	return d,
		&fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678", Bytes: 1024}},
		&fakeRuns{numbered: true},
		&fakeDumper{docker: d, res: backup.DBDumpResult{SnapshotID: "abc123def456", Bytes: 4096}}
}

func TestDBDumpRunsAfterPreHookAndBeforeStop(t *testing.T) {
	d, r, runs, dumper := dumpFakes(t)
	deps := dumpDeps(d, r, runs, dumper)
	deps.PreHook = "echo pre"

	if _, err := backup.BackupContainer(t.Context(), deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"exec:pg:sh -c echo pre", "dbdump", "stop:pg", "start:pg", "waitRunning:pg"}
	if len(d.log) != len(want) {
		t.Fatalf("docker log = %v, want %v", d.log, want)
	}
	for i := range want {
		if d.log[i] != want[i] {
			t.Fatalf("docker log[%d] = %q, want %q (full %v)", i, d.log[i], want[i], d.log)
		}
	}
}

func TestDBDumpSkippedWhenContainerStopped(t *testing.T) {
	d, r, runs, dumper := dumpFakes(t)
	deps := dumpDeps(d, r, runs, dumper)
	deps.WasRunning = false

	if _, err := backup.BackupContainer(t.Context(), deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(dumper.reqs) != 0 {
		t.Fatalf("a stopped container must not be dumped: %v", dumper.reqs)
	}
	for _, kind := range runs.kinds {
		if kind == "dbdump" {
			t.Fatalf("no dump run may be started for a stopped container: %v", runs.kinds)
		}
	}
}

func TestDBDumpFailureNeverFailsTheBackup(t *testing.T) {
	d, r, runs, dumper := dumpFakes(t)
	dumper.err = &backup.DBDumpError{Reason: store.ReasonDBDumpAuth}
	var outcome backup.DBDumpOutcome
	deps := dumpDeps(d, r, runs, dumper)
	deps.OnDBDumpDone = func(o backup.DBDumpOutcome) { outcome = o }

	if _, err := backup.BackupContainer(t.Context(), deps); err != nil {
		t.Fatalf("a failed dump must not fail the backup: %v", err)
	}

	for _, want := range []string{"stop:pg", "start:pg"} {
		if !contains(d.log, want) {
			t.Fatalf("docker log = %v, want %q in it", d.log, want)
		}
	}
	if len(r.log) == 0 {
		t.Fatalf("the volume backup must still run: %v", r.log)
	}
	if got := runs.finishOf(t, "run-2"); got.status != "failed" || got.note != store.ReasonDBDumpAuth {
		t.Fatalf("dump run finished %q/%q, want failed/%q", got.status, got.note, store.ReasonDBDumpAuth)
	}
	if got := runs.finishOf(t, "run-1"); got.status != "success" {
		t.Fatalf("backup run finished %q, want success", got.status)
	}
	if outcome.Status != "failed" || outcome.RunID != "run-2" || outcome.Reason != store.ReasonDBDumpAuth {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestDBDumpSuccessRecordsItsOwnRun(t *testing.T) {
	d, r, runs, dumper := dumpFakes(t)
	dumper.res.Note = store.NoteDBDumpOneDatabase
	var outcome backup.DBDumpOutcome
	deps := dumpDeps(d, r, runs, dumper)
	deps.OnDBDumpDone = func(o backup.DBDumpOutcome) { outcome = o }

	if _, err := backup.BackupContainer(t.Context(), deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runs.kinds[1] != "dbdump" {
		t.Fatalf("run kinds = %v, want the second one to be dbdump", runs.kinds)
	}
	got := runs.finishOf(t, "run-2")
	want := runFinish{runID: "run-2", status: "success", snapshotID: "abc123def456", bytes: 4096, note: store.NoteDBDumpOneDatabase}
	if got != want {
		t.Fatalf("dump run finished %+v, want %+v", got, want)
	}
	if outcome.SnapshotID != "abc123def456" || outcome.Bytes != 4096 || outcome.Status != "success" {
		t.Fatalf("outcome = %+v", outcome)
	}
	if note := runs.finishOf(t, "run-1").note; note != "" {
		t.Fatalf("backup run note = %q, want empty", note)
	}
}

func TestDBDumpTags(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  []string
	}{
		{
			name:  "with an image reference",
			image: "postgres:16",
			want:  []string{"dbdump:pg", "p1", "dbengine:postgres", "bvrun:run-1", "dbimage:postgres:16"},
		},
		{
			name:  "without one",
			image: "",
			want:  []string{"dbdump:pg", "p1", "dbengine:postgres", "bvrun:run-1"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d, r, runs, dumper := dumpFakes(t)
			deps := dumpDeps(d, r, runs, dumper)
			deps.DBDump.Image = tc.image

			if _, err := backup.BackupContainer(t.Context(), deps); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(dumper.reqs) != 1 {
				t.Fatalf("dump calls = %d, want 1", len(dumper.reqs))
			}
			req := dumper.reqs[0]
			if !equalStrings(req.Tags, tc.want) {
				t.Fatalf("tags = %v, want %v", req.Tags, tc.want)
			}
			if req.Repo != "/repo" {
				t.Fatalf("repo = %q, want /repo", req.Repo)
			}
			if req.Plan != *deps.DBDump {
				t.Fatalf("plan = %+v, want %+v", req.Plan, *deps.DBDump)
			}
		})
	}
}

func TestDBDumpLeavesVolumeTagsAlone(t *testing.T) {
	d, r, runs, dumper := dumpFakes(t)

	if _, err := backup.BackupContainer(t.Context(), dumpDeps(d, r, runs, dumper)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "backup:/repo:/host/user/appdata/pg:container:pg,p1"
	if len(r.log) != 1 || r.log[0] != want {
		t.Fatalf("restic log = %v, want [%s]", r.log, want)
	}
}

func TestNilDBDumpIsByteIdentical(t *testing.T) {
	run := func(t *testing.T, withDumper bool) ([]string, []string, []string) {
		t.Helper()
		d := &fakeDocker{}
		r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678", Bytes: 1024}}
		runs := &fakeRuns{}
		deps := backup.BackupDeps{
			ContainerRef:  "pg",
			ContainerName: "pg",
			RepoPath:      "/repo",
			AppdataPaths:  []string{"/host/user/appdata/pg"},
			TargetID:      "t1",
			WasRunning:    true,
			PreHook:       "echo pre",
			PostHook:      "echo post",
			Docker:        d,
			Restic:        r,
			Templates:     &fakeTemplates{},
			Runs:          runs,
		}
		if withDumper {
			deps.DBDumper = &fakeDumper{docker: d}
			deps.OnDBDumpDone = func(backup.DBDumpOutcome) {
				t.Error("OnDBDumpDone fired without a dump plan")
			}
		}
		if _, err := backup.BackupContainer(t.Context(), deps); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return d.log, r.log, runs.log
	}

	plainDocker, plainRestic, plainRuns := run(t, false)
	dockerLog, resticLog, runsLog := run(t, true)

	if !equalStrings(plainDocker, dockerLog) {
		t.Fatalf("docker log = %v, want %v", dockerLog, plainDocker)
	}
	if !equalStrings(plainRestic, resticLog) {
		t.Fatalf("restic log = %v, want %v", resticLog, plainRestic)
	}
	if !equalStrings(plainRuns, runsLog) {
		t.Fatalf("runs log = %v, want %v", runsLog, plainRuns)
	}
}

func TestCancelDuringDBDumpStopsBeforeStop(t *testing.T) {
	tests := []struct {
		name    string
		ctx     func(t *testing.T) (context.Context, func())
		wantErr error
		wantMsg string
	}{
		{
			name: "the user cancels",
			ctx: func(t *testing.T) (context.Context, func()) {
				t.Helper()
				ctx, cancel := context.WithCancel(t.Context())
				return ctx, cancel
			},
			wantErr: context.Canceled,
			wantMsg: "backup: stopped after database dump: context canceled",
		},
		{
			name: "the backup runs out of time",
			ctx: func(t *testing.T) (context.Context, func()) {
				t.Helper()
				ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
				t.Cleanup(cancel)
				return ctx, func() { <-ctx.Done() }
			},
			wantErr: context.DeadlineExceeded,
			wantMsg: "backup: stopped after database dump: context deadline exceeded",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d, r, runs, dumper := dumpFakes(t)
			ctx, endCtx := tc.ctx(t)
			dumper.onDump = endCtx

			_, err := backup.BackupContainer(ctx, dumpDeps(d, r, runs, dumper))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			for _, unwanted := range []string{"stop:", "start:"} {
				if contains(d.log, unwanted) {
					t.Fatalf("nothing may be stopped or started: %v", d.log)
				}
			}
			if len(r.log) != 0 {
				t.Fatalf("restic must not run: %v", r.log)
			}
			got := runs.finishOf(t, "run-1")
			if got.status != "failed" || got.note != tc.wantMsg {
				t.Fatalf("backup run finished %q/%q, want failed/%q", got.status, got.note, tc.wantMsg)
			}
		})
	}
}

func TestDBDumpRunStartFailureSkipsTheDump(t *testing.T) {
	d, r, runs, dumper := dumpFakes(t)
	runs.startErr = errors.New("database is locked")
	runs.startErrKind = "dbdump"

	if _, err := backup.BackupContainer(t.Context(), dumpDeps(d, r, runs, dumper)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(dumper.reqs) != 0 {
		t.Fatalf("no dump may run without a run row: %v", dumper.reqs)
	}
	got := runs.finishOf(t, "run-1")
	if got.status != "success" || got.note != store.NoteDBDumpNotRecorded {
		t.Fatalf("backup run finished %q/%q, want success/%q", got.status, got.note, store.NoteDBDumpNotRecorded)
	}
}

func TestDBDumpLeftoverRecordsSnapshotOnFailedRun(t *testing.T) {
	d, r, runs, dumper := dumpFakes(t)
	const snapshot = "3f9c2a1b7d5e4068"
	dumper.err = &backup.DBDumpError{
		Reason:     store.ReasonDBDumpLeftover + ": 3f9c2a1b",
		SnapshotID: snapshot,
	}

	if _, err := backup.BackupContainer(t.Context(), dumpDeps(d, r, runs, dumper)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := runs.finishOf(t, "run-2")
	if got.status != "failed" || got.snapshotID != snapshot {
		t.Fatalf("dump run finished %+v, want failed with snapshot %q", got, snapshot)
	}
}

func TestUnexpectedDumperErrorIsRecordedTruncated(t *testing.T) {
	d, r, runs, dumper := dumpFakes(t)
	dumper.err = errors.New(strings.Repeat("x", 600))

	if _, err := backup.BackupContainer(t.Context(), dumpDeps(d, r, runs, dumper)); err != nil {
		t.Fatalf("a dumper error must not fail the backup: %v", err)
	}

	got := runs.finishOf(t, "run-2")
	if got.status != "failed" || got.note != strings.Repeat("x", 500) {
		t.Fatalf("dump run finished %q with a %d character reason", got.status, len(got.note))
	}
}

func TestPostHookFailureDoesNotTouchTheDumpRun(t *testing.T) {
	d, r, runs, dumper := dumpFakes(t)
	d.execErr = errors.New("post hook exploded")
	deps := dumpDeps(d, r, runs, dumper)
	deps.PostHook = "false"

	if _, err := backup.BackupContainer(t.Context(), deps); err != nil {
		t.Fatalf("a failing post hook must not fail the backup: %v", err)
	}

	if len(runs.finishCalls) != 2 {
		t.Fatalf("finishes = %+v, want one per run", runs.finishCalls)
	}
	if got := runs.finishOf(t, "run-2"); got.status != "success" {
		t.Fatalf("dump run finished %q, want success", got.status)
	}
	if got := runs.finishOf(t, "run-1"); got.status != "success" {
		t.Fatalf("backup run finished %q, want success", got.status)
	}
}

// TestRestoreNeverExecsForDatabaseImage pins that a restore of a database
// container runs no command inside it: the volume snapshot is authoritative and
// a dump is never replayed.
func TestRestoreNeverExecsForDatabaseImage(t *testing.T) {
	d := &fakeDocker{liveName: "/plex"}
	runs := &fakeRuns{}
	deps := restoreDeps(d, &fakeRestic{}, &fakeTemplates{}, runs)
	in := deps.Inspect
	in.Config = model.Config{Image: "postgres:16"}
	deps.Inspect = in

	if err := backup.RestoreContainer(t.Context(), deps); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	want := []string{"inspectName:plex", "pull:postgres:16", "stop:plex", "remove:plex", "createAndStart:/plex"}
	if len(d.log) != len(want) {
		t.Fatalf("docker log = %v, want %v", d.log, want)
	}
	for i := range want {
		if d.log[i] != want[i] {
			t.Fatalf("docker log[%d] = %q, want %q (full %v)", i, d.log[i], want[i], d.log)
		}
	}
}
