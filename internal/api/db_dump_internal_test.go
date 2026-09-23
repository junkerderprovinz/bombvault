package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dbdump"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestDBDumpPlanFor(t *testing.T) {
	postgres := model.Inspect{Running: true, Config: model.Config{Image: "postgres:16.4", Env: []string{"POSTGRES_PASSWORD=x"}}}
	lookalike := model.Inspect{Running: true, Config: model.Config{Image: "acme/my-postgres:1", Env: []string{"POSTGRES_PASSWORD=x"}}}

	tests := []struct {
		name       string
		enabled    bool
		target     store.Target
		in         model.Inspect
		wantEngine string
		wantImage  string
	}{
		{name: "curated image", enabled: true, in: postgres, wantEngine: "postgres", wantImage: "postgres:16.4"},
		{name: "global switch off", in: postgres},
		{name: "container opted out", enabled: true, target: store.Target{DBDumpOff: true}, in: postgres},
		{name: "stopped container", enabled: true, in: model.Inspect{Config: postgres.Config}},
		{
			name:    "label off",
			enabled: true,
			in: model.Inspect{Running: true, Config: model.Config{
				Image:  "postgres:16.4",
				Labels: map[string]string{"bombvault.dbdump": "false"},
			}},
		},
		{name: "lookalike without a chosen engine", enabled: true, in: lookalike},
		{
			name:       "lookalike with a chosen engine",
			enabled:    true,
			target:     store.Target{DBDumpEngine: "postgres"},
			in:         lookalike,
			wantEngine: "postgres",
			wantImage:  "acme/my-postgres:1",
		},
		{
			name:       "image that cannot be a tag",
			enabled:    true,
			in:         model.Inspect{Running: true, Config: model.Config{Image: "postgres:16,4"}},
			wantEngine: "postgres",
		},
		{name: "not a database", enabled: true, in: model.Inspect{Running: true, Config: model.Config{Image: "plexinc/pms-docker:latest"}}},
	}

	s := &Service{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan := s.dbDumpPlanFor(store.Settings{DBDumpsEnabled: tc.enabled}, tc.target, "pg", tc.in)
			if tc.wantEngine == "" {
				if plan != nil {
					t.Fatalf("plan = %+v, want none", plan)
				}
				return
			}
			if plan == nil {
				t.Fatal("no plan for a container that must be dumped")
			}
			if plan.Engine != tc.wantEngine {
				t.Errorf("engine = %q, want %q", plan.Engine, tc.wantEngine)
			}
			if plan.Identity != "dbdump:pg" {
				t.Errorf("identity = %q", plan.Identity)
			}
			if plan.StdinPath != "/dbdump/pg.sql" {
				t.Errorf("stdin path = %q", plan.StdinPath)
			}
			if plan.Image != tc.wantImage {
				t.Errorf("image = %q, want %q", plan.Image, tc.wantImage)
			}
			if plan.MaxRuntime <= 0 {
				t.Errorf("max runtime = %v", plan.MaxRuntime)
			}
		})
	}
}

func TestDBDumpMaxRuntime(t *testing.T) {
	const noCap = 0

	tests := []struct {
		name string
		raw  string
		cap  time.Duration
		want time.Duration
	}{
		{name: "unset", cap: noCap, want: 6 * time.Hour},
		{name: "hours", raw: "12", cap: noCap, want: 12 * time.Hour},
		{name: "zero", raw: "0", cap: noCap, want: 6 * time.Hour},
		{name: "beyond the range", raw: "49", cap: noCap, want: 6 * time.Hour},
		{name: "not a number", raw: "abc", cap: noCap, want: 6 * time.Hour},
		{name: "backup cap leaves an hour", cap: 4 * time.Hour, want: 3 * time.Hour},
		{name: "backup cap of one hour leaves half of it", cap: time.Hour, want: 30 * time.Minute},
		{name: "backup cap of two hours", cap: 2 * time.Hour, want: time.Hour},
		{name: "backup cap above the default", cap: 48 * time.Hour, want: 6 * time.Hour},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := dbDumpMaxRuntimeFrom(tc.raw, tc.cap); got != tc.want {
				t.Fatalf("dbDumpMaxRuntimeFrom(%q, %v) = %v, want %v", tc.raw, tc.cap, got, tc.want)
			}
		})
	}

	logged := captureLog(t, func() {
		once := sync.OnceValue(func() time.Duration { return dbDumpMaxRuntimeFrom("abc", noCap) })
		for range 3 {
			once()
		}
	})
	if n := strings.Count(logged, "DB_DUMP_MAX_HOURS"); n != 1 {
		t.Fatalf("three reads logged the invalid value %d times:\n%s", n, logged)
	}
}

func TestDBDumpHelperArgv(t *testing.T) {
	s := &Service{dbDumpHelper: "/usr/local/bin/bombvault"}
	argv := s.dbDumpHelperArgv("pg", backup.DBDumpPlan{Engine: "postgres", MaxRuntime: 6 * time.Hour})

	want := []string{
		"/usr/local/bin/bombvault", "dbdump-stream",
		"--container", "pg", "--engine", "postgres", "--max-seconds", "21600",
	}
	if strings.Join(argv, " ") != strings.Join(want, " ") {
		t.Fatalf("argv = %v, want %v", argv, want)
	}
}

// captureLog collects what fn writes to the standard logger.
func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	out, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(out); log.SetFlags(flags) })
	fn()
	return buf.String()
}

// dumpFakeEngine serves the one restic call a dump makes, plus the forget that
// removes a snapshot the dump must not leave behind.
type dumpFakeEngine struct {
	ResticEngine
	sum   restic.Summary
	lines []string
	err   error
	// onBackup runs inside BackupFromCommand, with its context, so a test can
	// watch the guards that are armed on it.
	onBackup func(ctx context.Context) (restic.Summary, []string, error)

	repo       string
	stdinPath  string
	tags       []string
	command    []string
	forgotten  []string
	forgetErr  error
	backupCall int
}

func (f *dumpFakeEngine) BackupFromCommand(ctx context.Context, repo, stdinPath string, tags, command []string, _ restic.Mode) (restic.Summary, []string, error) {
	f.backupCall++
	f.repo, f.stdinPath, f.tags, f.command = repo, stdinPath, tags, command
	if f.onBackup != nil {
		return f.onBackup(ctx)
	}
	return f.sum, f.lines, f.err
}

func (f *dumpFakeEngine) Forget(_ context.Context, _ string, snapshotIDs []string, prune bool, _ restic.Mode) error {
	if prune {
		return errors.New("a dump snapshot is forgotten without pruning")
	}
	f.forgotten = append(f.forgotten, snapshotIDs...)
	return f.forgetErr
}

// dumpFakeDocker serves the probe exec and the orphan stop.
type dumpFakeDocker struct {
	dockercli.Docker
	probeOut string
	probeErr error
	execErr  error
	// inspect is the dump's container as it is after the dump.
	inspect    model.Inspect
	inspectErr error

	execArgv    [][]string
	execCtxDone []bool
}

func (f *dumpFakeDocker) ExecOutput(_ context.Context, _ string, _ []string, _ int) (string, int, error) {
	if f.probeErr != nil {
		return "", 1, f.probeErr
	}
	return f.probeOut, 0, nil
}

func (f *dumpFakeDocker) Exec(ctx context.Context, _ string, cmd []string) error {
	f.execArgv = append(f.execArgv, cmd)
	f.execCtxDone = append(f.execCtxDone, ctx.Err() != nil)
	return f.execErr
}

func (f *dumpFakeDocker) Inspect(context.Context, string) (model.Inspect, error) {
	return f.inspect, f.inspectErr
}

func (f *dumpFakeDocker) ExecStdin(context.Context, string, []string, io.Reader, int) (string, int, error) {
	return "", 0, nil
}

// dumpAdapter wires an adapter over the two fakes, as Service.Backup does.
func dumpAdapter(eng *dumpFakeEngine, dock *dumpFakeDocker) *dbDumpAdapter {
	return &dbDumpAdapter{
		svc:         &Service{dbDumpHelper: "/usr/local/bin/bombvault"},
		engine:      eng,
		docker:      dock,
		container:   "pg",
		containerID: "c0ffee1d",
		progressKey: "container:pg",
		startedAt:   1700000000,
	}
}

func dumpRequest() backup.DBDumpRequest {
	return backup.DBDumpRequest{
		Repo: "/repo/containers",
		Plan: backup.DBDumpPlan{
			Engine:     "postgres",
			Identity:   "dbdump:pg",
			StdinPath:  "/dbdump/pg.sql",
			MaxRuntime: time.Hour,
		},
		Tags: []string{"dbdump:pg", "p1", "dbengine:postgres", "bvrun:run-1"},
	}
}

// resultLine renders the helper's verdict the way restic forwards it.
func resultLine(r dbdump.Result) string {
	return "subprocess /usr/local/bin/bombvault: " + r.Line()
}

func dumpFailure(t *testing.T, res backup.DBDumpResult, err error) *backup.DBDumpError {
	t.Helper()
	if err == nil {
		t.Fatalf("dump succeeded with %+v, want a failure", res)
	}
	var fail *backup.DBDumpError
	if !errors.As(err, &fail) {
		t.Fatalf("error %v is not a DBDumpError", err)
	}
	return fail
}

func TestDBDumpAdapterReasons(t *testing.T) {
	want := map[string]string{
		dbdump.ReasonAuth:          store.ReasonDBDumpAuth,
		dbdump.ReasonPrivileges:    store.ReasonDBDumpPrivileges,
		dbdump.ReasonUnreachable:   store.ReasonDBDumpUnreachable,
		dbdump.ReasonNoClient:      store.ReasonDBDumpNoClient,
		dbdump.ReasonSecret:        store.ReasonDBDumpSecret,
		dbdump.ReasonNoCredentials: store.ReasonDBDumpNoCredentials,
		dbdump.ReasonNotRunning:    store.ReasonDBDumpNotRunning,
		dbdump.ReasonNeedsUpgrade:  store.ReasonDBDumpNeedsUpgrade,
		dbdump.ReasonTimeout:       store.ReasonDBDumpTimeout,
		dbdump.ReasonEmpty:         store.ReasonDBDumpEmpty,
		dbdump.ReasonIncomplete:    store.ReasonDBDumpIncomplete,
		dbdump.ReasonTool:          store.ReasonDBDumpTool,
		dbdump.ReasonDocker:        store.ReasonDBDumpDocker,
		dbdump.ReasonWrite:         store.ReasonDBDumpRepository,
		dbdump.ReasonUsage:         store.ReasonDBDumpHelper,
		dbdump.ReasonCancelled:     store.ReasonCancelled,
	}

	for _, id := range dbdump.ReasonIDs {
		t.Run(id, func(t *testing.T) {
			eng := &dumpFakeEngine{
				lines: []string{resultLine(dbdump.Result{V: 1, Reason: id, Exit: 1, Detail: "tool said so"})},
				err:   errors.New("restic backup: exit status 1"),
			}
			a := dumpAdapter(eng, &dumpFakeDocker{})
			res, err := a.Dump(context.Background(), dumpRequest())
			fail := dumpFailure(t, res, err)

			if fail.Reason != want[id]+": tool said so" {
				t.Fatalf("reason = %q, want %q", fail.Reason, want[id]+": tool said so")
			}
		})
	}

	t.Run("an id nothing maps", func(t *testing.T) {
		if got := dbDumpReasonText("nonsense", ""); got != store.ReasonDBDumpHelper {
			t.Fatalf("reason = %q, want %q", got, store.ReasonDBDumpHelper)
		}
	})
}

func TestDBDumpAdapterNoResultLine(t *testing.T) {
	eng := &dumpFakeEngine{err: errors.New("restic backup: signal: killed")}
	a := dumpAdapter(eng, &dumpFakeDocker{})

	res, err := a.Dump(context.Background(), dumpRequest())
	fail := dumpFailure(t, res, err)

	if !strings.HasPrefix(fail.Reason, store.ReasonDBDumpHelper+": ") {
		t.Fatalf("reason = %q, want the helper reason with the restic error", fail.Reason)
	}
	if !strings.Contains(fail.Reason, "killed") {
		t.Fatalf("reason = %q, want it to carry what restic said", fail.Reason)
	}
}

func TestDBDumpAdapterTrustsResticWithoutResultLine(t *testing.T) {
	eng := &dumpFakeEngine{
		sum:   restic.Summary{SnapshotID: "aaaa1111bbbb2222", TotalBytesProcessed: 500},
		lines: []string{"subprocess /usr/local/bin/bombvault: bombvault-dbdump-scope database"},
	}
	a := dumpAdapter(eng, &dumpFakeDocker{})

	res, err := a.Dump(context.Background(), dumpRequest())
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if res.Summary.SnapshotID != "aaaa1111bbbb2222" || res.Summary.Bytes != 500 {
		t.Fatalf("result = %+v", res)
	}
	if res.Note != store.NoteDBDumpOneDatabase {
		t.Fatalf("note = %q, want the one-database note", res.Note)
	}
	if len(eng.forgotten) != 0 {
		t.Fatalf("a snapshot restic wrote was forgotten: %v", eng.forgotten)
	}
}

func TestDBDumpAdapterByteMismatchForgets(t *testing.T) {
	eng := &dumpFakeEngine{
		sum:   restic.Summary{SnapshotID: "aaaa1111bbbb2222", TotalBytesProcessed: 10},
		lines: []string{resultLine(dbdump.Result{V: 1, OK: true, Bytes: 11})},
	}
	a := dumpAdapter(eng, &dumpFakeDocker{})

	res, err := a.Dump(context.Background(), dumpRequest())
	fail := dumpFailure(t, res, err)

	if fail.Reason != store.ReasonDBDumpMismatch {
		t.Fatalf("reason = %q", fail.Reason)
	}
	if len(eng.forgotten) != 1 || eng.forgotten[0] != "aaaa1111bbbb2222" {
		t.Fatalf("forgotten = %v, want the mismatching snapshot", eng.forgotten)
	}
	if fail.SnapshotID != "" {
		t.Fatalf("snapshot id = %q, want none once it is gone", fail.SnapshotID)
	}
}

func TestDBDumpAdapterPartialSnapshotForgets(t *testing.T) {
	eng := &dumpFakeEngine{err: &restic.CommandSnapshotPartialError{SnapshotID: "aaaa1111bbbb2222"}}
	a := dumpAdapter(eng, &dumpFakeDocker{})

	res, err := a.Dump(context.Background(), dumpRequest())
	fail := dumpFailure(t, res, err)

	if fail.Reason != store.ReasonDBDumpIncomplete {
		t.Fatalf("reason = %q", fail.Reason)
	}
	if len(eng.forgotten) != 1 || eng.forgotten[0] != "aaaa1111bbbb2222" {
		t.Fatalf("forgotten = %v", eng.forgotten)
	}
}

func TestDBDumpAdapterForgetFailureIsLeftover(t *testing.T) {
	eng := &dumpFakeEngine{
		err:       &restic.CommandSnapshotPartialError{SnapshotID: "aaaa1111bbbb2222"},
		forgetErr: errors.New("repository is append-only"),
	}
	a := dumpAdapter(eng, &dumpFakeDocker{})

	res, err := a.Dump(context.Background(), dumpRequest())
	fail := dumpFailure(t, res, err)

	if fail.Reason != store.ReasonDBDumpLeftover+": aaaa1111" {
		t.Fatalf("reason = %q, want the leftover reason with the short id", fail.Reason)
	}
	if fail.SnapshotID != "aaaa1111bbbb2222" {
		t.Fatalf("snapshot id = %q, want the snapshot the dump list must mark damaged", fail.SnapshotID)
	}
}

func TestDBDumpAdapterStallCancelsOnlyTheDump(t *testing.T) {
	eng := &dumpFakeEngine{onBackup: func(ctx context.Context) (restic.Summary, []string, error) {
		<-ctx.Done()
		return restic.Summary{}, nil, ctx.Err()
	}}
	a := dumpAdapter(eng, &dumpFakeDocker{})
	a.guard = func(ctx context.Context, cancel context.CancelFunc, _ string) context.Context {
		time.AfterFunc(10*time.Millisecond, cancel)
		return ctx
	}

	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	res, err := a.Dump(parent, dumpRequest())
	fail := dumpFailure(t, res, err)

	if fail.Reason != store.ReasonDBDumpStalled {
		t.Fatalf("reason = %q, want the stall reason", fail.Reason)
	}
	if parent.Err() != nil {
		t.Fatalf("the backup around the dump was cancelled too: %v", parent.Err())
	}
}

func TestDBDumpAdapterStopsOrphanFromForwardedPid(t *testing.T) {
	pidLine := "subprocess /usr/local/bin/bombvault: bombvault-dbdump-pid 4242"
	wantArgv, err := dbdump.OrphanStopArgv(4242)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("a cancelled dump", func(t *testing.T) {
		eng := &dumpFakeEngine{lines: []string{pidLine}, err: context.Canceled}
		dock := &dumpFakeDocker{}
		a := dumpAdapter(eng, dock)

		parent, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := a.Dump(parent, dumpRequest()); err == nil {
			t.Fatal("a cancelled dump reported success")
		}

		if len(dock.execArgv) != 1 {
			t.Fatalf("%d orphan stops, want exactly one", len(dock.execArgv))
		}
		if strings.Join(dock.execArgv[0], "\x00") != strings.Join(wantArgv, "\x00") {
			t.Fatalf("argv = %v, want %v", dock.execArgv[0], wantArgv)
		}
		if dock.execCtxDone[0] {
			t.Fatal("the orphan stop ran on a context that was already done")
		}
	})

	t.Run("a finished dump", func(t *testing.T) {
		eng := &dumpFakeEngine{
			sum:   restic.Summary{SnapshotID: "aaaa1111bbbb2222", TotalBytesProcessed: 7},
			lines: []string{pidLine},
		}
		dock := &dumpFakeDocker{}
		a := dumpAdapter(eng, dock)

		if _, err := a.Dump(context.Background(), dumpRequest()); err != nil {
			t.Fatalf("Dump: %v", err)
		}
		if len(dock.execArgv) != 0 {
			t.Fatalf("a finished dump stopped something: %v", dock.execArgv)
		}
	})

	t.Run("no pid to stop", func(t *testing.T) {
		eng := &dumpFakeEngine{err: errors.New("restic backup: exit status 1")}
		dock := &dumpFakeDocker{}
		a := dumpAdapter(eng, dock)

		if _, err := a.Dump(context.Background(), dumpRequest()); err == nil {
			t.Fatal("a dump that restic refused reported success")
		}

		if len(dock.execArgv) != 0 {
			t.Fatalf("something was stopped without a pid: %v", dock.execArgv)
		}
	})
}

func TestDBDumpAdapterNamesAnOrphanItCouldNotStop(t *testing.T) {
	pidLine := "subprocess /usr/local/bin/bombvault: bombvault-dbdump-pid 4242"
	stopErr := errors.New("dockercli: exec create: container is paused")
	paused := model.Inspect{ID: "c0ffee1d", Name: "/pg", Running: true}

	t.Run("behind the reason alone", func(t *testing.T) {
		eng := &dumpFakeEngine{lines: []string{pidLine}, err: context.Canceled}
		a := dumpAdapter(eng, &dumpFakeDocker{execErr: stopErr, inspect: paused})
		parent, cancel := context.WithCancel(context.Background())
		cancel()

		res, err := a.Dump(parent, dumpRequest())
		fail := dumpFailure(t, res, err)
		if want := store.ReasonCancelled + ": orphan stop failed"; fail.Reason != want {
			t.Fatalf("reason = %q, want %q", fail.Reason, want)
		}
	})

	t.Run("behind the tool's own message", func(t *testing.T) {
		eng := &dumpFakeEngine{
			lines: []string{pidLine, resultLine(dbdump.Result{V: 1, Reason: dbdump.ReasonTool, Exit: 1, Detail: "tool said so"})},
			err:   errors.New("restic backup: exit status 1"),
		}
		a := dumpAdapter(eng, &dumpFakeDocker{execErr: stopErr, inspect: paused})

		res, err := a.Dump(context.Background(), dumpRequest())
		fail := dumpFailure(t, res, err)
		if want := store.ReasonDBDumpTool + ": tool said so; orphan stop failed"; fail.Reason != want {
			t.Fatalf("reason = %q, want %q", fail.Reason, want)
		}
		if head := dbDumpReasonHead(fail.Reason); head != store.ReasonDBDumpTool {
			t.Errorf("head = %q, want the remedy and the debounce to still see %q", head, store.ReasonDBDumpTool)
		}
	})

	// Every process a dump left in a container ends with the container.
	gone := map[string]*dumpFakeDocker{
		"stopped":   {execErr: stopErr, inspect: model.Inspect{ID: "c0ffee1d", Name: "/pg"}},
		"recreated": {execErr: stopErr, inspect: model.Inspect{ID: "5eed0001", Name: "/pg", Running: true}},
		"removed":   {execErr: stopErr, inspectErr: errors.New("dockercli: inspect: Error response from daemon: No such container: pg")},
	}
	for name, dock := range gone {
		t.Run("not in a container that was "+name, func(t *testing.T) {
			eng := &dumpFakeEngine{lines: []string{pidLine}, err: context.Canceled}
			parent, cancel := context.WithCancel(context.Background())
			cancel()

			res, err := dumpAdapter(eng, dock).Dump(parent, dumpRequest())
			if fail := dumpFailure(t, res, err); fail.Reason != store.ReasonCancelled {
				t.Errorf("reason = %q, want %q alone", fail.Reason, store.ReasonCancelled)
			}
		})
	}
}

func TestDBDumpAdapterForgetsASnapshotTheHelperDisowns(t *testing.T) {
	disowned := func() *dumpFakeEngine {
		return &dumpFakeEngine{
			sum:   restic.Summary{SnapshotID: "aaaa1111bbbb2222", TotalBytesProcessed: 4096},
			lines: []string{resultLine(dbdump.Result{V: 1, Reason: dbdump.ReasonIncomplete, Exit: 1, Bytes: 4096})},
		}
	}

	t.Run("the snapshot goes", func(t *testing.T) {
		eng := disowned()
		res, err := dumpAdapter(eng, &dumpFakeDocker{}).Dump(context.Background(), dumpRequest())
		fail := dumpFailure(t, res, err)

		if fail.Reason != store.ReasonDBDumpIncomplete {
			t.Fatalf("reason = %q, want %q", fail.Reason, store.ReasonDBDumpIncomplete)
		}
		if len(eng.forgotten) != 1 || eng.forgotten[0] != "aaaa1111bbbb2222" {
			t.Fatalf("forgotten = %v, want the snapshot the helper called incomplete", eng.forgotten)
		}
	})

	t.Run("a snapshot that cannot go is the leftover", func(t *testing.T) {
		eng := disowned()
		eng.forgetErr = errors.New("repository is append-only")
		res, err := dumpAdapter(eng, &dumpFakeDocker{}).Dump(context.Background(), dumpRequest())
		fail := dumpFailure(t, res, err)

		if fail.Reason != store.ReasonDBDumpLeftover+": aaaa1111" || fail.SnapshotID != "aaaa1111bbbb2222" {
			t.Fatalf("failure = %+v, want the leftover reason naming the snapshot", fail)
		}
	})
}

func TestDBDumpAdapterParentEnds(t *testing.T) {
	blockUntilDone := func(ctx context.Context) (restic.Summary, []string, error) {
		<-ctx.Done()
		return restic.Summary{}, nil, ctx.Err()
	}

	t.Run("cancelled", func(t *testing.T) {
		a := dumpAdapter(&dumpFakeEngine{onBackup: blockUntilDone}, &dumpFakeDocker{})
		parent, cancel := context.WithCancel(context.Background())
		cancel()

		res, err := a.Dump(parent, dumpRequest())
		fail := dumpFailure(t, res, err)
		if fail.Reason != store.ReasonCancelled {
			t.Fatalf("reason = %q, want %q", fail.Reason, store.ReasonCancelled)
		}
	})

	t.Run("past the backup's own cap", func(t *testing.T) {
		a := dumpAdapter(&dumpFakeEngine{onBackup: blockUntilDone}, &dumpFakeDocker{})
		parent, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()

		res, err := a.Dump(parent, dumpRequest())
		fail := dumpFailure(t, res, err)
		if fail.Reason != store.ReasonDBDumpBackupCap {
			t.Fatalf("reason = %q, want %q", fail.Reason, store.ReasonDBDumpBackupCap)
		}
	})
}

func TestDBDumpAdapterPublishesStage(t *testing.T) {
	eng := &dumpFakeEngine{
		sum: restic.Summary{SnapshotID: "aaaa1111bbbb2222", TotalBytesProcessed: 900},
		onBackup: func(ctx context.Context) (restic.Summary, []string, error) {
			watch := restic.WatcherFrom(ctx)
			if watch == nil {
				t.Error("no watcher on the dump's context")
				return restic.Summary{SnapshotID: "aaaa1111bbbb2222", TotalBytesProcessed: 900}, nil, nil
			}
			watch(restic.Progress{BytesDone: 100})
			time.Sleep(dbDumpProgressEvery + 50*time.Millisecond)
			watch(restic.Progress{BytesDone: 900})
			return restic.Summary{SnapshotID: "aaaa1111bbbb2222", TotalBytesProcessed: 900}, nil, nil
		},
	}
	a := dumpAdapter(eng, &dumpFakeDocker{})
	a.svc.progress = progress.NewStore()
	events, stop := a.svc.progress.Subscribe()
	defer stop()

	percents := 0
	ctx := progress.WithSink(context.Background(), func(float64) { percents++ })
	if _, err := a.Dump(ctx, dumpRequest()); err != nil {
		t.Fatalf("Dump: %v", err)
	}

	var bytes []int64
	var stages []string
	for len(stages) < 3 {
		select {
		case e := <-events:
			stages = append(stages, e.Stage)
			bytes = append(bytes, e.Bytes)
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d events: stages %v", len(stages), stages)
		}
	}
	if stages[0] != "dbdump" || stages[1] != "dbdump" || stages[2] != "" {
		t.Fatalf("stages = %v, want two dump stages and then the volume phase", stages)
	}
	if bytes[0] != 100 || bytes[1] != 900 {
		t.Fatalf("bytes = %v, want them growing with the stream", bytes)
	}
	if percents != 0 {
		t.Fatalf("the dump reported %d percentages to the backup's sink", percents)
	}
}

func TestDBDumpAdapterProbeTags(t *testing.T) {
	probeOut := "bombvault-dbdump-version pg_dumpall (PostgreSQL) 16.4\n" +
		"bombvault-dbdump-db postgres\n" +
		"bombvault-dbdump-db immich\n"

	t.Run("a probe that answers", func(t *testing.T) {
		eng := &dumpFakeEngine{sum: restic.Summary{SnapshotID: "aaaa1111bbbb2222", TotalBytesProcessed: 1}}
		a := dumpAdapter(eng, &dumpFakeDocker{probeOut: probeOut})

		if _, err := a.Dump(context.Background(), dumpRequest()); err != nil {
			t.Fatalf("Dump: %v", err)
		}
		tags := strings.Join(eng.tags, " ")
		for _, want := range []string{"dbversion:16.4", "dbname:postgres", "dbname:immich"} {
			if !strings.Contains(tags, want) {
				t.Errorf("tags %v are missing %q", eng.tags, want)
			}
		}
	})

	t.Run("a probe that fails", func(t *testing.T) {
		eng := &dumpFakeEngine{sum: restic.Summary{SnapshotID: "aaaa1111bbbb2222", TotalBytesProcessed: 1}}
		a := dumpAdapter(eng, &dumpFakeDocker{probeErr: errors.New("no such container")})

		if _, err := a.Dump(context.Background(), dumpRequest()); err != nil {
			t.Fatalf("Dump: %v", err)
		}
		if tags := strings.Join(eng.tags, " "); strings.Contains(tags, "dbversion:") || strings.Contains(tags, "dbname:") {
			t.Fatalf("tags = %v, want none from a failed probe", eng.tags)
		}
		if eng.backupCall != 1 {
			t.Fatalf("%d dumps after a failed probe, want one", eng.backupCall)
		}
	})
}

func TestIdentityTagsIncludeDBDump(t *testing.T) {
	snaps := []restic.Snapshot{{Tags: []string{
		"dbdump:pg", "p1", "bvrun:x", "dbengine:postgres", "dbimage:postgres:16", "dbversion:16.4", "dbname:immich",
	}}}

	got := identityTags(snaps)
	if len(got) != 1 || got[0] != "dbdump:pg" {
		t.Fatalf("identityTags = %v, want only the dump identity: every other dump tag describes\n"+
			"the snapshot and would get a retention series of its own", got)
	}
}

// serviceWithContainersRepo builds a Service over a real store whose containers
// repository exists on disk, so a snapshot listing reaches the engine.
func serviceWithContainersRepo(t *testing.T, eng ResticEngine) *Service {
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
	dir := t.TempDir()
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return &Service{store: st, engine: eng, cfg: config.Config{HostMountRoot: dir, AppKey: strings.Repeat("a", 64)}}
}

func TestContainerHasBackupsCountsDumps(t *testing.T) {
	eng := &previewEngine{snaps: []restic.Snapshot{snapWithTags("aaaa1111", "dbdump:pg", "p1")}}
	s := serviceWithContainersRepo(t, eng)

	has, err := s.containerHasBackups(context.Background(), "pg")
	if err != nil {
		t.Fatalf("containerHasBackups: %v", err)
	}
	if !has {
		t.Error("a container whose only backups are database dumps must count as backed up:\n" +
			"a repository override would otherwise orphan those dumps without a word")
	}
	if eng.snapsCalls != 1 {
		t.Errorf("the repository was listed %d times, want once: Discover and the repository\n"+
			"override ask this per container", eng.snapsCalls)
	}
}

func TestDBDataCoverage(t *testing.T) {
	toContainer := func(host string) (string, bool) {
		if rest, ok := strings.CutPrefix(host, "/mnt/"); ok {
			return "/host/" + rest, true
		}
		return "", false
	}
	stackMount := []model.Mount{{Source: "/mnt/user/stacks/immich/pgdata", Destination: "/var/lib/postgresql/data"}}

	tests := []struct {
		name       string
		engine     dbdump.Engine
		env        []string
		mounts     []model.Mount
		stackDir   string
		effective  []string
		folderSets []string
		want       string
	}{
		{
			name:     "datadir inside the compose project directory",
			engine:   dbdump.EnginePostgres,
			mounts:   stackMount,
			stackDir: "/host/user/stacks/immich",
			want:     "live",
		},
		{
			name:      "datadir under a backed-up path",
			engine:    dbdump.EnginePostgres,
			mounts:    stackMount,
			stackDir:  "/host/user/stacks/immich",
			effective: []string{"/host/user/stacks/immich/pgdata"},
			want:      "stopped",
		},
		{
			name:       "datadir under a folder set",
			engine:     dbdump.EnginePostgres,
			mounts:     stackMount,
			folderSets: []string{"/host/user/stacks"},
			want:       "live",
		},
		{
			name:   "datadir outside every backup path",
			engine: dbdump.EnginePostgres,
			mounts: []model.Mount{{Source: "/mnt/user/databases/pg", Destination: "/var/lib/postgresql/data"}},
			want:   "none",
		},
		{
			name:      "volume on the parent of PGDATA",
			engine:    dbdump.EnginePostgres,
			env:       []string{"PGDATA=/var/lib/postgresql/data/pgdata"},
			mounts:    []model.Mount{{Source: "/mnt/user/appdata/pg", Destination: "/var/lib/postgresql/data"}},
			effective: []string{"/host/user/appdata/pg"},
			want:      "stopped",
		},
		{
			name:   "bind below the mount holding PGDATA",
			engine: dbdump.EnginePostgres,
			env:    []string{"PGDATA=/data/pg"},
			mounts: []model.Mount{{Source: "/mnt/user/databases/x", Destination: "/data"}},
			want:   "none",
		},
		{
			name:   "no data mount",
			engine: dbdump.EngineMariaDB,
			mounts: []model.Mount{{Source: "/mnt/user/appdata/x", Destination: "/logs"}},
			want:   "unknown",
		},
		{
			name:   "datadir outside the host mount",
			engine: dbdump.EnginePostgres,
			mounts: []model.Mount{{Source: "/srv/pg", Destination: "/var/lib/postgresql/data"}},
			want:   "none",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := model.Inspect{Config: model.Config{Env: tc.env}, Mounts: tc.mounts}
			got := dbDataCoverage(in, tc.engine, toContainer, tc.stackDir, tc.effective, tc.folderSets)
			if got != tc.want {
				t.Fatalf("coverage = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDBDumpAdapterLogLeavesOutTheToolsMessageWhenTheSnapshotStays(t *testing.T) {
	eng := &dumpFakeEngine{
		sum:       restic.Summary{SnapshotID: "aaaa1111bbbb2222", TotalBytesProcessed: 4096},
		lines:     []string{resultLine(dbdump.Result{V: 1, Reason: dbdump.ReasonPrivileges, Exit: 1, Detail: "permission denied for table users, row (alice@example.com)"})},
		forgetErr: errors.New("repository is append-only"),
	}

	logged := captureLog(t, func() {
		_, _ = dumpAdapter(eng, &dumpFakeDocker{}).Dump(context.Background(), dumpRequest())
	})

	if !strings.Contains(logged, store.ReasonDBDumpPrivileges) {
		t.Errorf("the log does not say why the snapshot had to go:\n%s", logged)
	}
	if strings.Contains(logged, "alice@example.com") {
		t.Errorf("the log carries what the database tool said:\n%s", logged)
	}
}
