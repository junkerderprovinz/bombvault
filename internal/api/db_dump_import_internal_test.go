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

	stopErr  error
	startErr error
	// onStart runs inside Start, so a test can read the data folders at the
	// moment the container comes back up.
	onStart func()

	probeOut string

	tail    string
	exit    int
	execErr error
	fed     string

	calls []string
}

func (f *importFakeDocker) Inspect(_ context.Context, name string) (model.Inspect, error) {
	f.calls = append(f.calls, "inspect:"+name)
	return f.inspect, nil
}

func (f *importFakeDocker) Stop(_ context.Context, name string, _ time.Duration) error {
	f.calls = append(f.calls, "stop:"+name)
	return f.stopErr
}

func (f *importFakeDocker) Start(_ context.Context, name string) error {
	f.calls = append(f.calls, "start:"+name)
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
	return "", 0, nil
}

func (f *importFakeDocker) ExecStdin(_ context.Context, name string, _ []string, stdin io.Reader, _ int) (string, int, error) {
	f.calls = append(f.calls, "import:"+name)
	fed, err := io.ReadAll(stdin)
	if err != nil {
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
			Running: true,
			Config:  model.Config{Image: "postgres:16.4", Env: []string{"POSTGRES_USER=immich"}},
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
	// pg_dumpall's role section always collides with the role a fresh cluster
	// was created for.
	const expected = "ERROR:  role \"immich\" already exists\n"

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

		want := "inspect:pg probe:pg stop:pg start:pg ready:pg import:pg"
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
	})
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
		if got := strings.Count(strings.Join(rig.dock.calls, " "), "start:pg"); got != 2 {
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
