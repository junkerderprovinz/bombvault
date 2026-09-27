package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const (
	importDumpID   = "3f9c2a1be0d4aaaa"
	importDumpTime = "2026-09-17T02:14:03.123456789+02:00"
)

// importFakeDocker serves the container calls an import makes: the version
// probe, the stop and start around the data folder swap, the readiness poll and
// the import exec itself.
type importFakeDocker struct {
	dockercli.Docker
	inspect model.Inspect

	stopErr error
	// stopPanic makes the stop of the database itself panic, once the apps
	// around it are already down.
	stopPanic bool
	// others answers Inspect for the containers around the database.
	others map[string]model.Inspect
	// rollbackStopErr answers every stop of the database after the first one.
	rollbackStopErr error
	stops           int
	startErr        error
	readyExit       int
	// onStart runs inside Start, so a test can read the data folders at the
	// moment the container comes back up.
	onStart func()
	// startErrs answers Start for the containers around the database.
	startErrs map[string]error

	probeOut string

	tail    string
	exit    int
	execErr error
	fed     string
	// swallowInputErr makes the import exit cleanly on a truncated input, as
	// psql does when it reads a script that simply stops.
	swallowInputErr bool

	calls []string
}

func (f *importFakeDocker) Inspect(_ context.Context, name string) (model.Inspect, error) {
	f.calls = append(f.calls, "inspect:"+name)
	if other, ok := f.others[name]; ok {
		return other, nil
	}
	return f.inspect, nil
}

func (f *importFakeDocker) Stop(_ context.Context, name string, _ time.Duration) error {
	f.calls = append(f.calls, "stop:"+name)
	if name != f.inspect.ID {
		return nil
	}
	if f.stopPanic {
		panic("boom while stopping the database")
	}
	if f.stops++; f.stops > 1 {
		return f.rollbackStopErr
	}
	return f.stopErr
}

func (f *importFakeDocker) Start(_ context.Context, name string) error {
	f.calls = append(f.calls, "start:"+name)
	if err, ok := f.startErrs[name]; ok {
		return err
	}
	if f.onStart != nil {
		f.onStart()
	}
	return f.startErr
}

func (f *importFakeDocker) ExecOutput(_ context.Context, name string, cmd []string, _ int) (string, int, error) {
	if cmd[len(cmd)-1] == "bombvault-dbdump-probe" {
		f.calls = append(f.calls, "probe:"+name)
		return f.probeOut, 0, nil
	}
	f.calls = append(f.calls, "ready:"+name)
	return "", f.readyExit, nil
}

func (f *importFakeDocker) ExecStdin(_ context.Context, name string, _ []string, stdin io.Reader, _ int) (string, int, error) {
	f.calls = append(f.calls, "import:"+name)
	fed, err := io.ReadAll(stdin)
	if err != nil && !f.swallowInputErr {
		return "", 0, err
	}
	f.fed = string(fed)
	return f.tail, f.exit, f.execErr
}

// importRig is a service with one importable PostgreSQL dump of "pg" whose data
// folder lies under the host mount.
type importRig struct {
	svc     *Service
	dock    *importFakeDocker
	eng     *httpDumpEngine
	dataDir string
	payload string
}

func newImportRig(t *testing.T) *importRig {
	t.Helper()
	payload := "-- PostgreSQL database cluster dump\nCREATE ROLE immich;\n"
	eng := &httpDumpEngine{
		raw: []byte(payload),
		snaps: []restic.Snapshot{
			dumpSnapshot(importDumpID, importDumpTime, uint64(len(payload)), "dbengine:postgres", "dbversion:16.4"),
		},
	}
	svc := serviceWithContainersRepo(t, eng)
	svc.cfg.HostSourceRoot = "/data"
	if _, err := svc.store.UpsertTarget(store.Target{ContainerName: "pg"}); err != nil {
		t.Fatal(err)
	}

	dataDir := filepath.Join(svc.cfg.HostMountRoot, "pg")
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "PG_VERSION"), []byte("16\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	dock := &importFakeDocker{
		inspect: model.Inspect{
			ID:      "c0ffee1d",
			Name:    "/pg",
			Running: true,
			Config:  model.Config{Image: "postgres:16.4", Env: []string{"POSTGRES_USER=immich", "POSTGRES_DB=immich_db"}},
			Mounts:  []model.Mount{{Source: "/data/pg", Destination: "/var/lib/postgresql/data"}},
		},
		probeOut: "bombvault-dbdump-version pg_dumpall (PostgreSQL) 16.4\n",
	}
	svc.docker = dock
	return &importRig{svc: svc, dock: dock, eng: eng, dataDir: dataDir, payload: payload}
}

func importRequest(name, id string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/containers/"+name+"/dbdumps/"+id+"/import", nil)
	r.SetPathValue("name", name)
	r.SetPathValue("id", id)
	return r
}

// siblingsOf lists the folders next to the data folder, which is where both the
// kept and the failed one appear.
func siblingsOf(t *testing.T, dataDir, suffix string) []string {
	t.Helper()
	found, err := filepath.Glob(dataDir + suffix)
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func entryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func runsOfKind(t *testing.T, st *store.Repo, kind string) []store.Run {
	t.Helper()
	runs, err := st.ListRuns(20)
	if err != nil {
		t.Fatal(err)
	}
	var out []store.Run
	for _, r := range runs {
		if r.Kind == kind {
			out = append(out, r)
		}
	}
	return out
}

func TestImportRefusesBeforeTouchingAnything(t *testing.T) {
	refusals := []struct {
		name    string
		id      string
		prepare func(t *testing.T, rig *importRig)
		want    string
		numbers []float64
	}{
		{
			name: "another backup or restore is running",
			id:   importDumpID,
			prepare: func(_ *testing.T, rig *importRig) {
				rig.svc.batchActive.Store(true)
			},
			want: "busy",
		},
		{
			name: "the container is stopped",
			id:   importDumpID,
			prepare: func(_ *testing.T, rig *importRig) {
				rig.dock.inspect.Running = false
			},
			want: "notRunning",
		},
		{
			name: "the dump comes from another engine",
			id:   importDumpID,
			prepare: func(_ *testing.T, rig *importRig) {
				rig.eng.snaps = []restic.Snapshot{
					dumpSnapshot(importDumpID, importDumpTime, 64, "dbengine:mariadb", "dbversion:11.4"),
				}
			},
			want: "engineMismatch",
		},
		{
			name: "the dump carries no engine",
			id:   importDumpID,
			prepare: func(_ *testing.T, rig *importRig) {
				rig.eng.snaps = []restic.Snapshot{dumpSnapshot(importDumpID, importDumpTime, 64)}
			},
			want: "engineMismatch",
		},
		{
			name: "the data folder is out of reach",
			id:   importDumpID,
			prepare: func(_ *testing.T, rig *importRig) {
				rig.dock.inspect.Mounts = nil
			},
			want: "noDataMount",
		},
		{
			name: "the server is older than the dump",
			id:   importDumpID,
			prepare: func(_ *testing.T, rig *importRig) {
				rig.dock.probeOut = "bombvault-dbdump-version pg_dumpall (PostgreSQL) 14.19\n"
			},
			want:    "version",
			numbers: []float64{14, 16},
		},
		{
			name: "a failed dump left the snapshot behind",
			id:   importDumpID,
			prepare: func(t *testing.T, rig *importRig) {
				tg, err := rig.svc.store.GetTargetByContainer("pg")
				if err != nil {
					t.Fatal(err)
				}
				finishedRun(t, rig.svc.store, tg.ID, "dbdump", "failed", importDumpID)
			},
			want: "damaged",
		},
		{
			name: "the snapshot holds the container's volumes",
			id:   "eeee5555eeee5555",
			prepare: func(_ *testing.T, rig *importRig) {
				rig.eng.snaps = append(rig.eng.snaps, snapWithTags("eeee5555eeee5555", "container:pg"))
			},
			want: "notADump",
		},
		{
			// A removed container's dumps are still listed, and Docker answers
			// an unknown name with the container whose id starts with it.
			name: "the name resolves to another container's id",
			id:   importDumpID,
			prepare: func(_ *testing.T, rig *importRig) {
				rig.dock.inspect.Name = "/immich_postgres"
			},
		},
	}

	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			rig := newImportRig(t)
			tc.prepare(t, rig)

			w := httptest.NewRecorder()
			dumpHandler(rig.svc).handleImportDBDump(w, importRequest("pg", tc.id))

			var resp struct {
				OK     bool    `json:"ok"`
				Code   string  `json:"code"`
				Error  string  `json:"error"`
				Server float64 `json:"server"`
				Dump   float64 `json:"dump"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v (%s)", err, w.Body.String())
			}
			if resp.OK || resp.Code != tc.want {
				t.Fatalf("answer = %+v, want a refusal with the reason id %q", resp, tc.want)
			}
			if tc.numbers != nil && (resp.Server != tc.numbers[0] || resp.Dump != tc.numbers[1]) {
				t.Errorf("server/dump = %v/%v, want %v: the page builds the sentence from the numbers",
					resp.Server, resp.Dump, tc.numbers)
			}

			if got := strings.Join(rig.dock.calls, " "); strings.Contains(got, "stop:") {
				t.Errorf("calls = %q, want the container untouched", got)
			}
			if got := entryNames(t, rig.dataDir); len(got) != 1 || got[0] != "PG_VERSION" {
				t.Errorf("the data folder holds %v, want it untouched", got)
			}
			if got := siblingsOf(t, rig.dataDir, ".bombvault-*"); len(got) != 0 {
				t.Errorf("a refusal moved the data folder: %v", got)
			}
			if got := runsOfKind(t, rig.svc.store, "dbimport"); len(got) != 0 {
				t.Errorf("a refusal recorded %d import runs", len(got))
			}
		})
	}
}

func TestImportStepsInOrder(t *testing.T) {
	// A cluster the image initialised from POSTGRES_USER and POSTGRES_DB holds
	// both before the dump's own CREATE statements run.
	const expected = "ERROR:  role \"immich\" already exists\n" +
		"ERROR:  database \"immich_db\" already exists\n"

	t.Run("the fresh database is in place before the container starts", func(t *testing.T) {
		rig := newImportRig(t)
		rig.dock.tail = expected
		var atStart []string
		var keptAtStart int
		rig.dock.onStart = func() {
			atStart = entryNames(t, rig.dataDir)
			keptAtStart = len(siblingsOf(t, rig.dataDir, ".bombvault-before-import-*"))
		}

		started, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID)
		if err != nil || !started {
			t.Fatalf("started=%v err=%v", started, err)
		}
		waitForDetachedRun(t, rig.svc)

		want := "inspect:pg probe:c0ffee1d stop:c0ffee1d start:c0ffee1d ready:c0ffee1d import:c0ffee1d"
		if got := strings.Join(rig.dock.calls, " "); got != want {
			t.Fatalf("calls = %q, want %q", got, want)
		}
		if len(atStart) != 0 || keptAtStart != 1 {
			t.Errorf("at start the data folder held %v and %d kept folders existed, want an empty folder next to the kept one",
				atStart, keptAtStart)
		}
		if rig.dock.fed != rig.payload {
			t.Errorf("the import was fed %q, want the dump", rig.dock.fed)
		}

		kept := siblingsOf(t, rig.dataDir, ".bombvault-before-import-*")
		if len(kept) != 1 {
			t.Fatalf("kept folders = %v, want exactly one", kept)
		}
		if got, err := os.ReadFile(filepath.Join(kept[0], "PG_VERSION")); err != nil || string(got) != "16\n" {
			t.Errorf("the kept folder holds %q (%v), want the old data", got, err)
		}
		if runtime.GOOS != "windows" {
			info, sErr := os.Stat(rig.dataDir)
			if sErr != nil {
				t.Fatal(sErr)
			}
			if info.Mode().Perm() != 0o750 {
				t.Errorf("the fresh folder's mode = %v, want the old 0750", info.Mode().Perm())
			}
		}

		runs := runsOfKind(t, rig.svc.store, "dbimport")
		if len(runs) != 1 || runs[0].Status != "success" {
			t.Fatalf("runs = %+v, want one successful import", runs)
		}
		if !strings.HasPrefix(runs[0].Error, store.NoteDBImportKeptOld+": ") || !strings.HasSuffix(runs[0].Error, filepath.Base(kept[0])) {
			t.Errorf("note = %q, want %q naming the kept folder %q", runs[0].Error, store.NoteDBImportKeptOld, kept[0])
		}
		if want := store.NoteDBImportKeptOld + ": /data/pg.bombvault-before-import-"; !strings.HasPrefix(runs[0].Error, want) {
			t.Errorf("note = %q, want the kept folder as the host names it, %q...", runs[0].Error, want)
		}
	})

	t.Run("an error the import reports itself is counted", func(t *testing.T) {
		rig := newImportRig(t)
		rig.dock.tail = expected + "ERROR:  relation \"users\" does not exist\n"

		if _, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil {
			t.Fatal(err)
		}
		waitForDetachedRun(t, rig.svc)

		runs := runsOfKind(t, rig.svc.store, "dbimport")
		if len(runs) != 1 || runs[0].Status != "success" {
			t.Fatalf("runs = %+v, want one successful import", runs)
		}
		if !strings.HasPrefix(runs[0].Error, store.NoteDBImportErrors) || !strings.Contains(runs[0].Error, "1") {
			t.Errorf("note = %q, want %q with the count", runs[0].Error, store.NoteDBImportErrors)
		}
	})

	t.Run("an import the tool refuses fails and names the kept folder", func(t *testing.T) {
		rig := newImportRig(t)
		rig.dock.exit = 1
		rig.dock.tail = "psql: error: connection to server failed\n"

		if _, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil {
			t.Fatal(err)
		}
		waitForDetachedRun(t, rig.svc)

		kept := siblingsOf(t, rig.dataDir, ".bombvault-before-import-*")
		runs := runsOfKind(t, rig.svc.store, "dbimport")
		if len(runs) != 1 || runs[0].Status != "failed" {
			t.Fatalf("runs = %+v, want one failed import", runs)
		}
		if !strings.HasPrefix(runs[0].Error, store.ReasonDBImportFailed) || !strings.Contains(runs[0].Error, filepath.Base(kept[0])) {
			t.Errorf("reason = %q, want %q naming the kept folder %q", runs[0].Error, store.ReasonDBImportFailed, kept[0])
		}
		if !strings.Contains(runs[0].Error, " kept at /data/pg.bombvault-before-import-") {
			t.Errorf("reason = %q, want the kept folder as the host names it", runs[0].Error)
		}
	})
}

func TestImportFailsWhenTheDumpCannotBeRead(t *testing.T) {
	rig := newImportRig(t)
	rig.eng.rawErr = errors.New("restic dump: pack 5e1f not found")
	rig.dock.swallowInputErr = true

	if _, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil {
		t.Fatal(err)
	}
	waitForDetachedRun(t, rig.svc)

	kept := siblingsOf(t, rig.dataDir, ".bombvault-before-import-*")
	if len(kept) != 1 {
		t.Fatalf("kept folders = %v, want exactly one", kept)
	}
	runs := runsOfKind(t, rig.svc.store, "dbimport")
	if len(runs) != 1 || runs[0].Status != "failed" {
		t.Fatalf("runs = %+v, want one failed import", runs)
	}
	reason := runs[0].Error
	if !strings.HasPrefix(reason, store.ReasonDBImportFailed) || !strings.Contains(reason, filepath.Base(kept[0])) ||
		!strings.Contains(reason, "pack 5e1f not found") {
		t.Errorf("reason = %q, want %q naming the kept folder and the read error", reason, store.ReasonDBImportFailed)
	}
}

func TestImportRollsBackWhenStartFails(t *testing.T) {
	t.Run("the old data folder goes back", func(t *testing.T) {
		rig := newImportRig(t)
		rig.dock.startErr = errors.New("dockercli: start pg: address already in use")

		if _, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil {
			t.Fatal(err)
		}
		waitForDetachedRun(t, rig.svc)

		if got, err := os.ReadFile(filepath.Join(rig.dataDir, "PG_VERSION")); err != nil || string(got) != "16\n" {
			t.Errorf("the data folder holds %q (%v), want the old data back", got, err)
		}
		failed := siblingsOf(t, rig.dataDir, ".bombvault-import-failed-*")
		if len(failed) != 1 {
			t.Fatalf("failed folders = %v, want the fresh one kept aside", failed)
		}
		if got := siblingsOf(t, rig.dataDir, ".bombvault-before-import-*"); len(got) != 0 {
			t.Errorf("the kept folder is still aside: %v", got)
		}
		if got := strings.Count(strings.Join(rig.dock.calls, " "), "start:c0ffee1d"); got != 2 {
			t.Errorf("the container was started %d times, want a second try after the rollback", got)
		}

		runs := runsOfKind(t, rig.svc.store, "dbimport")
		if len(runs) != 1 || runs[0].Status != "failed" {
			t.Fatalf("runs = %+v, want one failed import", runs)
		}
		if !strings.HasPrefix(runs[0].Error, store.ReasonDBImportPrepare) {
			t.Errorf("reason = %q, want %q", runs[0].Error, store.ReasonDBImportPrepare)
		}
	})

	t.Run("a rollback that cannot put the data back names both folders", func(t *testing.T) {
		rig := newImportRig(t)
		rig.dock.startErr = errors.New("dockercli: start pg: address already in use")
		rig.dock.onStart = func() {
			kept := siblingsOf(t, rig.dataDir, ".bombvault-before-import-*")
			if len(kept) == 1 {
				if err := os.Rename(kept[0], kept[0]+".gone"); err != nil {
					t.Error(err)
				}
			}
		}

		if _, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil {
			t.Fatal(err)
		}
		waitForDetachedRun(t, rig.svc)

		failed := siblingsOf(t, rig.dataDir, ".bombvault-import-failed-*")
		gone := siblingsOf(t, rig.dataDir, ".bombvault-before-import-*.gone")
		if len(failed) != 1 || len(gone) != 1 {
			t.Fatalf("failed=%v gone=%v, want both folders on disk", failed, gone)
		}

		runs := runsOfKind(t, rig.svc.store, "dbimport")
		if len(runs) != 1 || runs[0].Status != "failed" {
			t.Fatalf("runs = %+v, want one failed import", runs)
		}
		reason := runs[0].Error
		if !strings.HasPrefix(reason, store.ReasonDBImportRollback) {
			t.Fatalf("reason = %q, want %q", reason, store.ReasonDBImportRollback)
		}
		if !strings.Contains(reason, filepath.Base(failed[0])) || !strings.Contains(reason, filepath.Base(strings.TrimSuffix(gone[0], ".gone"))) {
			t.Errorf("reason = %q, want both folder paths in it", reason)
		}
	})
}

func TestImportRollsBackWhenTheDatabaseNeverComesUp(t *testing.T) {
	every, bound := dbImportReadyEvery, dbImportReadyFor
	dbImportReadyEvery, dbImportReadyFor = time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { dbImportReadyEvery, dbImportReadyFor = every, bound })

	rig := newImportRig(t)
	rig.dock.readyExit = 2

	if _, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil {
		t.Fatal(err)
	}
	waitForDetachedRun(t, rig.svc)

	if got, err := os.ReadFile(filepath.Join(rig.dataDir, "PG_VERSION")); err != nil || string(got) != "16\n" {
		t.Errorf("the data folder holds %q (%v), want the old data back", got, err)
	}
	if got := siblingsOf(t, rig.dataDir, ".bombvault-import-failed-*"); len(got) != 1 {
		t.Errorf("failed folders = %v, want the fresh one kept aside", got)
	}
	calls := strings.Join(rig.dock.calls, " ")
	if strings.Contains(calls, "import:") {
		t.Errorf("calls = %q, want no import into a server that never answered", calls)
	}
	if got := strings.Count(calls, "start:c0ffee1d"); got != 2 {
		t.Errorf("the container was started %d times, want a second start after the rollback", got)
	}

	runs := runsOfKind(t, rig.svc.store, "dbimport")
	if len(runs) != 1 || runs[0].Status != "failed" {
		t.Fatalf("runs = %+v, want one failed import", runs)
	}
	if !strings.HasPrefix(runs[0].Error, store.ReasonDBImportPrepare) || !strings.Contains(runs[0].Error, "not ready") {
		t.Errorf("reason = %q, want %q saying the database was not ready", runs[0].Error, store.ReasonDBImportPrepare)
	}
}

func TestImportRollbackLeavesTheFoldersWhenTheServerCannotBeStopped(t *testing.T) {
	every, bound := dbImportReadyEvery, dbImportReadyFor
	dbImportReadyEvery, dbImportReadyFor = time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { dbImportReadyEvery, dbImportReadyFor = every, bound })

	rig := newImportRig(t)
	rig.dock.readyExit = 2
	rig.dock.rollbackStopErr = errors.New("dockercli: stop pg: context deadline exceeded")

	if _, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil {
		t.Fatal(err)
	}
	waitForDetachedRun(t, rig.svc)

	if got := entryNames(t, rig.dataDir); len(got) != 0 {
		t.Errorf("the running server's folder holds %v, want it left as the fresh one", got)
	}
	if got := siblingsOf(t, rig.dataDir, ".bombvault-import-failed-*"); len(got) != 0 {
		t.Errorf("the folder under the running server was moved to %v", got)
	}
	kept := siblingsOf(t, rig.dataDir, ".bombvault-before-import-*")
	if len(kept) != 1 {
		t.Fatalf("kept folders = %v, want the old data still aside", kept)
	}

	runs := runsOfKind(t, rig.svc.store, "dbimport")
	if len(runs) != 1 || runs[0].Status != "failed" {
		t.Fatalf("runs = %+v, want one failed import", runs)
	}
	reason := runs[0].Error
	if !strings.HasPrefix(reason, store.ReasonDBImportRollback) || !strings.Contains(reason, filepath.Base(kept[0])) {
		t.Errorf("reason = %q, want %q naming the kept folder", reason, store.ReasonDBImportRollback)
	}
}

func TestImportRollbackReasonKeepsBothFoldersBehindALongCause(t *testing.T) {
	rig := newImportRig(t)
	nested := filepath.Join("appdata", "immich", strings.Repeat("postgres-cluster-", 4), "pg")
	deepDir := filepath.Join(rig.svc.cfg.HostMountRoot, nested)
	if err := os.MkdirAll(filepath.Dir(deepDir), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(rig.dataDir, deepDir); err != nil {
		t.Fatal(err)
	}
	rig.dataDir = deepDir
	rig.dock.inspect.Mounts = []model.Mount{{Source: "/data/" + filepath.ToSlash(nested), Destination: "/var/lib/postgresql/data"}}
	rig.dock.startErr = errors.New("dockercli: start pg: driver failed programming external connectivity on endpoint pg (" +
		strings.Repeat("0123456789abcdef", 4) + "): " + strings.Repeat("Bind for 0.0.0.0:5432 failed: port is already allocated; ", 5))
	rig.dock.onStart = func() {
		kept := siblingsOf(t, rig.dataDir, ".bombvault-before-import-*")
		if len(kept) == 1 {
			if err := os.Rename(kept[0], kept[0]+".gone"); err != nil {
				t.Error(err)
			}
		}
	}

	if _, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil {
		t.Fatal(err)
	}
	waitForDetachedRun(t, rig.svc)

	failed := siblingsOf(t, rig.dataDir, ".bombvault-import-failed-*")
	gone := siblingsOf(t, rig.dataDir, ".bombvault-before-import-*.gone")
	if len(failed) != 1 || len(gone) != 1 {
		t.Fatalf("failed=%v gone=%v, want both folders on disk", failed, gone)
	}
	runs := runsOfKind(t, rig.svc.store, "dbimport")
	if len(runs) != 1 || !strings.HasPrefix(runs[0].Error, store.ReasonDBImportRollback) {
		t.Fatalf("runs = %+v, want one import that could not roll back", runs)
	}
	for _, folder := range []string{failed[0], strings.TrimSuffix(gone[0], ".gone")} {
		onHost := "/data/" + filepath.ToSlash(filepath.Join(filepath.Dir(nested), filepath.Base(folder)))
		if !strings.Contains(runs[0].Error, onHost) {
			t.Errorf("reason = %q, want the whole host path %s in it", runs[0].Error, onHost)
		}
	}
}

func TestImportStopsTheAppsOfTheDatabaseUntilItIsDone(t *testing.T) {
	appsOf := func(t *testing.T, rig *importRig) {
		t.Helper()
		if err := rig.svc.store.SetStopContainers("pg", []string{"immich_server", "immich_ml", "old_app", "cafe"}); err != nil {
			t.Fatal(err)
		}
		rig.dock.others = map[string]model.Inspect{
			"immich_server": {ID: "5e7e7e01", Name: "/immich_server", Running: true, Config: model.Config{Labels: map[string]string{
				"com.docker.compose.service":    "immich-server",
				"com.docker.compose.depends_on": "immich-machine-learning:service_started:false",
			}}},
			"immich_ml": {ID: "3a1a1a02", Name: "/immich_ml", Running: true, Config: model.Config{Labels: map[string]string{
				"com.docker.compose.service": "immich-machine-learning",
			}}},
			"old_app": {ID: "01d0a903", Name: "/old_app"},
			// Docker answers a name no container has with the container whose id
			// starts with it.
			"cafe": {ID: "cafe0b0e", Name: "/unrelated", Running: true},
		}
	}
	importFails := func(t *testing.T, rig *importRig) store.Run {
		t.Helper()
		if _, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil {
			t.Fatal(err)
		}
		waitForDetachedRun(t, rig.svc)
		runs := runsOfKind(t, rig.svc.store, "dbimport")
		if len(runs) != 1 || runs[0].Status != "failed" {
			t.Fatalf("runs = %+v, want one failed import", runs)
		}
		return runs[0]
	}

	t.Run("an import that goes through", func(t *testing.T) {
		rig := newImportRig(t)
		appsOf(t, rig)

		if _, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil {
			t.Fatal(err)
		}
		waitForDetachedRun(t, rig.svc)

		want := "inspect:pg probe:c0ffee1d inspect:immich_server stop:5e7e7e01 inspect:immich_ml stop:3a1a1a02 inspect:old_app inspect:cafe " +
			"stop:c0ffee1d start:c0ffee1d ready:c0ffee1d import:c0ffee1d start:3a1a1a02 start:5e7e7e01"
		if got := strings.Join(rig.dock.calls, " "); got != want {
			t.Errorf("calls = %q, want %q", got, want)
		}
	})

	t.Run("an import that is rolled back", func(t *testing.T) {
		rig := newImportRig(t)
		appsOf(t, rig)
		rig.dock.startErr = errors.New("dockercli: start pg: address already in use")
		rig.dock.startErrs = map[string]error{"5e7e7e01": nil, "3a1a1a02": nil}

		importFails(t, rig)

		calls := strings.Join(rig.dock.calls, " ")
		if !strings.HasSuffix(calls, "start:3a1a1a02 start:5e7e7e01") {
			t.Errorf("calls = %q, want the apps started again after the rollback", calls)
		}
		if strings.Contains(calls, "start:01d0a903") {
			t.Errorf("calls = %q, want the app that was stopped left stopped", calls)
		}
	})

	t.Run("an import the tool gives up on", func(t *testing.T) {
		rig := newImportRig(t)
		appsOf(t, rig)
		rig.dock.exit = 1
		rig.dock.tail = "ERROR 1273 (HY000) at line 40: Unknown collation: 'utf8mb4_uca1400_ai_ci'\n"

		run := importFails(t, rig)

		if calls := strings.Join(rig.dock.calls, " "); strings.Contains(calls, "start:3a1a1a02") || strings.Contains(calls, "start:5e7e7e01") {
			t.Errorf("calls = %q, want the apps kept away from a half-imported database", calls)
		}
		if !strings.HasSuffix(run.Error, "; these apps stay stopped until the data folder is sorted out: immich_server, immich_ml") {
			t.Errorf("reason = %q, want the apps that stay stopped named", run.Error)
		}
	})

	t.Run("an import whose rollback cannot stop the server", func(t *testing.T) {
		every, bound := dbImportReadyEvery, dbImportReadyFor
		dbImportReadyEvery, dbImportReadyFor = time.Millisecond, 20*time.Millisecond
		t.Cleanup(func() { dbImportReadyEvery, dbImportReadyFor = every, bound })

		rig := newImportRig(t)
		appsOf(t, rig)
		rig.dock.readyExit = 2
		rig.dock.rollbackStopErr = errors.New("dockercli: stop pg: context deadline exceeded")

		run := importFails(t, rig)

		if calls := strings.Join(rig.dock.calls, " "); strings.Contains(calls, "start:3a1a1a02") || strings.Contains(calls, "start:5e7e7e01") {
			t.Errorf("calls = %q, want the apps kept away from the empty database", calls)
		}
		if !strings.HasPrefix(run.Error, store.ReasonDBImportRollback) || !strings.Contains(run.Error, "immich_server, immich_ml") {
			t.Errorf("reason = %q, want the apps that stay stopped named", run.Error)
		}
	})

	t.Run("an app that does not start again", func(t *testing.T) {
		rig := newImportRig(t)
		appsOf(t, rig)
		rig.dock.startErrs = map[string]error{"5e7e7e01": errors.New("dockercli: start immich_server: port is already allocated")}

		if _, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil {
			t.Fatal(err)
		}
		waitForDetachedRun(t, rig.svc)

		runs := runsOfKind(t, rig.svc.store, "dbimport")
		if len(runs) != 1 || runs[0].Status != "success" {
			t.Fatalf("runs = %+v, want one successful import", runs)
		}
		if !strings.HasPrefix(runs[0].Error, store.NoteDBImportKeptOld) || !strings.HasSuffix(runs[0].Error, "; could not start these apps again: immich_server") {
			t.Errorf("note = %q, want the app that stayed down named", runs[0].Error)
		}
	})
}

func TestImportPanicNamesTheAppsItLeftStopped(t *testing.T) {
	rig := newImportRig(t)
	if err := rig.svc.store.SetStopContainers("pg", []string{"immich_server"}); err != nil {
		t.Fatal(err)
	}
	rig.dock.others = map[string]model.Inspect{
		"immich_server": {ID: "5e7e7e01", Name: "/immich_server", Running: true},
	}
	rig.dock.stopPanic = true

	var run store.Run
	captureLog(t, func() {
		if _, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil {
			t.Fatal(err)
		}
		run = waitForImportRunClosed(t, rig.svc.store)
	})

	if run.Status != "failed" {
		t.Fatalf("run = %+v, want a failed import", run)
	}
	if !strings.Contains(run.Error, "recovered panic") {
		t.Errorf("reason = %q, want the panic recorded", run.Error)
	}
	if !strings.HasSuffix(run.Error, "; "+store.ImportTailAppsStopped+": immich_server") {
		t.Errorf("reason = %q, want the app that stays stopped named, as a normal failure does", run.Error)
	}
}

// waitForImportRunClosed polls until the import's run row leaves "running".
// The panic path closes it after the single-flight guard is already clear.
func waitForImportRunClosed(t *testing.T, st *store.Repo) store.Run {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if runs := runsOfKind(t, st, "dbimport"); len(runs) == 1 && runs[0].Status != "running" {
			return runs[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the import's run to be closed")
	return store.Run{}
}

func TestImportFailureLogLeavesOutTheToolsMessage(t *testing.T) {
	rig := newImportRig(t)
	rig.dock.exit = 1
	rig.dock.tail = "ERROR:  duplicate key value violates unique constraint \"users_email_key\"\nDETAIL:  Key (email)=(alice@example.com) already exists.\n"

	logged := captureLog(t, func() {
		if _, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil {
			t.Fatal(err)
		}
		waitForDetachedRun(t, rig.svc)
	})

	if !strings.Contains(logged, store.ReasonDBImportFailed) {
		t.Errorf("the log does not say the import failed:\n%s", logged)
	}
	if strings.Contains(logged, "alice@example.com") {
		t.Errorf("the log carries a row the import tool quoted:\n%s", logged)
	}
	if runs := runsOfKind(t, rig.svc.store, "dbimport"); len(runs) != 1 || !strings.Contains(runs[0].Error, "alice@example.com") {
		t.Errorf("runs = %+v, want the run history to keep the tool's message", runs)
	}
}

func TestImportFailureNamesTheLineTheToolEndedOn(t *testing.T) {
	rig := newImportRig(t)
	rig.dock.exit = 2
	rig.dock.tail = "ERROR:  role \"immich\" already exists\n" +
		strings.Repeat("ERROR:  relation \"asset_faces\" already exists\nCONTEXT:  Ausführung der Zeile 4711\n", 12) +
		"psql: error: server closed the connection unexpectedly\n"

	if _, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil {
		t.Fatal(err)
	}
	waitForDetachedRun(t, rig.svc)

	runs := runsOfKind(t, rig.svc.store, "dbimport")
	if len(runs) != 1 || runs[0].Status != "failed" {
		t.Fatalf("runs = %+v, want one failed import", runs)
	}
	reason := runs[0].Error
	if !strings.Contains(reason, "exit 2: ") || !strings.Contains(reason, "server closed the connection unexpectedly") {
		t.Errorf("reason = %q, want the exit code and the line the tool ended on", reason)
	}
	if strings.Contains(reason, "role \"immich\"") {
		t.Errorf("reason = %q, want the start of a long output left out", reason)
	}
	if !utf8.ValidString(reason) {
		t.Errorf("reason = %q, want valid UTF-8", reason)
	}
}

func TestImportCauseIsCutBetweenCharacters(t *testing.T) {
	cause := errors.New(strings.Repeat("a", dbImportDetailMax-1) + "äöü")
	msg := importPrepareFailure(cause).Error()
	if !utf8.ValidString(msg) || !strings.HasSuffix(msg, strings.Repeat("a", dbImportDetailMax-1)) {
		t.Errorf("cause cut to %q, want it to end before the character the limit falls into", msg[len(msg)-8:])
	}
}

func TestACreatedFolderSwappedForALinkIsLeftAlone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("links need privileges on Windows")
	}
	root := t.TempDir()
	elsewhere := filepath.Join(root, "config")
	if err := os.Mkdir(elsewhere, 0o700); err != nil {
		t.Fatal(err)
	}
	created := filepath.Join(root, "pg")
	if err := os.Symlink(elsewhere, created); err != nil {
		t.Fatal(err)
	}

	if d, err := openCreatedDir(created); err == nil {
		_ = d.Close()
		t.Fatal("a folder swapped for a link was opened for its mode and owner")
	}
}
