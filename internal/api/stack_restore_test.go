package api_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const stackFolder = "/host/user/stacks/shop"

// folderWatchEngine notes what Docker had been told by the time the project
// folder is written, and can fail that write.
type folderWatchEngine struct {
	*fakeResticEngine
	docker    *fakeServiceDocker
	atWrite   []string
	folderErr error
}

func (e *folderWatchEngine) RestorePath(ctx context.Context, repo, id, p string, m restic.Mode) error {
	if p != stackFolder {
		return e.fakeResticEngine.RestorePath(ctx, repo, id, p, m)
	}
	e.atWrite = slices.Clone(e.docker.calls)
	if e.folderErr != nil {
		return e.folderErr
	}
	return e.fakeResticEngine.RestorePath(ctx, repo, id, p, m)
}

func (e *folderWatchEngine) folderWritten() bool {
	return slices.ContainsFunc(e.restored, func(r string) bool { return strings.HasSuffix(r, ":"+stackFolder) })
}

// shopStack is the compose project "shop" with a backup of its folder, a db that
// app depends on, and both running.
type shopStack struct {
	t      *testing.T
	eng    *folderWatchEngine
	docker *fakeServiceDocker
	svc    *api.Service
	st     *store.Repo
}

func newShopStack(t *testing.T) *shopStack {
	t.Helper()
	d := &fakeServiceDocker{inspects: map[string]model.Inspect{
		"db":  composeInspect("db", "shop", "db", ""),
		"app": composeInspect("app", "shop", "app", "db"),
	}}
	eng := &folderWatchEngine{docker: d, fakeResticEngine: &fakeResticEngine{snaps: []restic.Snapshot{
		memberSnapshot("aaaa1111", "app"),
		memberSnapshot("bbbb2222", "db"),
		{ID: "eeee5555", Time: "2026-11-01T00:00:00Z", Tags: []string{"stack:shop"}, Paths: []string{stackFolder}},
	}}}
	svc, st, mountRoot := stackTestService(t, eng, d)
	seedStackTarget(t, st, mountRoot, "app", "shop", "app", "db")
	seedStackTarget(t, st, mountRoot, "db", "shop", "db", "")
	return &shopStack{t: t, eng: eng, docker: d, svc: svc, st: st}
}

func (s *shopStack) restore() (api.StackRestoreResult, error) {
	s.t.Helper()
	return s.svc.RestoreStack(context.Background(), "shop", "local", "", true, true)
}

func (s *shopStack) lastRestoreRun(name string) store.Run {
	s.t.Helper()
	tg, err := s.st.GetTargetByContainer(name)
	if err != nil {
		s.t.Fatal(err)
	}
	run, err := s.st.LastRestoreRun(tg.ID)
	if err != nil || run == nil {
		s.t.Fatalf("no restore run of %s: %v, %v", name, run, err)
	}
	return *run
}

func (s *shopStack) calledAny(prefix string) bool {
	return slices.ContainsFunc(s.docker.calls, func(c string) bool { return strings.HasPrefix(c, prefix) })
}

func TestAStackRestoreStopsItsMembersBeforeItWritesTheFolder(t *testing.T) {
	s := newShopStack(t)

	if _, err := s.restore(); err != nil {
		t.Fatal(err)
	}

	if !s.eng.folderWritten() {
		t.Fatalf("the project folder was not restored: %v", s.eng.restored)
	}
	var stops []string
	for _, c := range s.eng.atWrite {
		if name, ok := strings.CutPrefix(c, "stop:"); ok {
			stops = append(stops, name)
		}
	}
	if !slices.Equal(stops, []string{"app", "db"}) {
		t.Errorf("stopped before the write: %v, want app, then db", stops)
	}
}

func TestAStackMemberThatKeepsRunningEndsTheRestoreBeforeAnyWrite(t *testing.T) {
	s := newShopStack(t)
	s.docker.stopErr = errors.New("timed out")

	if _, err := s.restore(); err != nil {
		t.Fatal(err)
	}

	if len(s.eng.restored) != 0 || s.eng.atWrite != nil {
		t.Errorf("restic wrote under a running member: %v", s.eng.restored)
	}
	for _, name := range []string{"app", "db"} {
		run := s.lastRestoreRun(name)
		if run.Status != "failed" || !strings.Contains(run.Error, "nothing was restored") {
			t.Errorf("%s: run = %s %q, want a failure that says nothing was restored", name, run.Status, run.Error)
		}
	}
	if !s.calledAny("start:app") {
		t.Errorf("app was stopped and not started again: %v", s.docker.calls)
	}
}

func TestAStackRestoreThatCannotWorkTouchesNothing(t *testing.T) {
	s := newShopStack(t)
	gone := composeInspect("db", "shop", "db", "")
	gone.HostConfig.NetworkMode = "shop_gone"
	def, err := marshalDefinition(gone, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.st.UpsertTarget(store.Target{ContainerName: "db", AppdataPaths: []string{"/host/user/appdata/db"}, Definition: string(def)}); err != nil {
		t.Fatal(err)
	}

	_, err = s.restore()

	if err == nil || !strings.Contains(err.Error(), "shop_gone") {
		t.Fatalf("RestoreStack = %v, want a refusal that names the network", err)
	}
	if s.eng.atWrite != nil || len(s.eng.restored) != 0 {
		t.Errorf("restic wrote: %v", s.eng.restored)
	}
	if s.calledAny("stop:") || s.calledAny("remove:") {
		t.Errorf("Docker was told to change something: %v", s.docker.calls)
	}
}

func TestAProjectFolderThatFailsIsOnEveryMembersRunAndNothingStarts(t *testing.T) {
	s := newShopStack(t)
	s.eng.folderErr = errors.New("restic restore failed: chmod [path]: no such file or directory")

	res, err := s.restore()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(res.FolderError, "no such file or directory") {
		t.Errorf("FolderError = %q", res.FolderError)
	}
	for _, m := range res.Members {
		if !m.Restored || m.Started {
			t.Errorf("%s: restored %v, started %v, want restored and left stopped", m.Name, m.Restored, m.Started)
		}
		run := s.lastRestoreRun(m.Name)
		if run.Status != "success" || !strings.HasPrefix(run.Error, store.NoteStackFolderFailed+": ") {
			t.Errorf("%s: run = %s %q, want the note on it", m.Name, run.Status, run.Error)
		}
	}
	if s.calledAny("start:") {
		t.Errorf("a member started on a half-written folder: %v", s.docker.calls)
	}
}

// A database on a named volume that is gone, with dumps and no files backup.
func dumpOnlyStack(t *testing.T) *shopStack {
	t.Helper()
	s := newShopStack(t)
	s.docker.inspects = nil
	s.docker.inspectErr = errors.New("no such container")
	s.eng.snaps = []restic.Snapshot{
		{ID: "dddd0001", Time: "2026-11-01T00:00:00Z", Tags: []string{"dbdump:db"}, Paths: []string{"/dbdump/db.sql"}},
		memberSnapshot("aaaa1111", "app"),
	}
	def, err := marshalDefinition(composeInspect("db", "shop", "db", ""), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.st.UpsertTarget(store.Target{ContainerName: "db", AppdataPaths: []string{}, Definition: string(def)}); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestADumpOnlyDatabaseComesBackStoppedAndHoldsItsAppsBack(t *testing.T) {
	s := dumpOnlyStack(t)

	res, err := s.restore()
	if err != nil {
		t.Fatal(err)
	}

	for _, m := range res.Members {
		if m.Started {
			t.Errorf("%s started on a database without its data", m.Name)
		}
	}
	if s.calledAny("start:") {
		t.Errorf("Docker was told to start: %v", s.docker.calls)
	}
	if run := s.lastRestoreRun("db"); !strings.Contains(run.Error, store.NoteRestoredWithoutData) {
		t.Errorf("db's run says %q, want the note", run.Error)
	}
	if run := s.lastRestoreRun("app"); !strings.HasPrefix(run.Error, store.NoteStackNotStarted+": ") {
		t.Errorf("app's run says %q, want why it stays stopped", run.Error)
	}
}

func TestADumpOnlyDatabaseThatIsStillThereStartsAsBefore(t *testing.T) {
	s := dumpOnlyStack(t)
	s.docker.listOut = []dockercli.ContainerInfo{{Name: "db"}}

	res, err := s.restore()
	if err != nil {
		t.Fatal(err)
	}

	for _, m := range res.Members {
		if !m.Started {
			t.Errorf("%s did not start: %q", m.Name, m.Error)
		}
	}
	if run := s.lastRestoreRun("db"); strings.Contains(run.Error, store.NoteRestoredWithoutData) {
		t.Errorf("db's run says %q, though its data is still on the host", run.Error)
	}
}
