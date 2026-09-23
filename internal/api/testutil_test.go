package api_test

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// newMemStore opens an in-memory SQLite store, migrates it, and returns a Repo.
func newMemStore(t *testing.T) *store.Repo {
	t.Helper()
	st, _ := newMemStoreDB(t)
	return st
}

// newMemStoreDB is newMemStore with the connection, for a test that has to
// backdate what it seeded so a series reads as history and not as one burst.
func newMemStoreDB(t *testing.T) (*store.Repo, *sql.DB) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store.New(db), db
}

// fakeServiceDocker is a configurable Docker fake satisfying dockercli.Docker.
// It records calls so tests can assert ordering and inputs.
type fakeServiceDocker struct {
	listOut []dockercli.ContainerInfo
	listErr error

	inspect model.Inspect
	// inspects answers per container name, for the tests that list several and
	// care which one was asked about. A name that is not in it falls back to
	// inspect.
	inspects   map[string]model.Inspect
	inspectErr error

	liveName    string
	inspNameErr error

	selfName string // returned by Self (own-container detection); "" = undetected

	allocations []model.Allocation
	allocErr    error

	stopErr        error
	startErr       error
	restartErr     error
	removeErr      error
	pullErr        error
	imageID        string // returned by ImageID after a pull
	imageIDErr     error
	imageRemoveErr error
	createErr      error
	createErrName  string // when set, CreateAndStart fails for this container name

	execOut    string // stdout of ExecOutput
	execStderr string // stderr tail of ExecStdin
	execExit   int    // exit code both report
	execErr    error
	execFed    string // what ExecStdin was fed

	started      bool
	createdIn    model.Inspect
	createdStart bool
	calls        []string
}

var _ dockercli.Docker = (*fakeServiceDocker)(nil)

func (f *fakeServiceDocker) List(_ context.Context) ([]dockercli.ContainerInfo, error) {
	f.calls = append(f.calls, "list")
	return f.listOut, f.listErr
}

func (f *fakeServiceDocker) Inspect(_ context.Context, name string) (model.Inspect, error) {
	f.calls = append(f.calls, "inspect:"+name)
	if f.inspectErr != nil {
		return model.Inspect{}, f.inspectErr
	}
	if in, ok := f.inspects[name]; ok {
		return in, nil
	}
	return f.inspect, nil
}

func (f *fakeServiceDocker) Stop(_ context.Context, name string, _ time.Duration) error {
	f.calls = append(f.calls, "stop:"+name)
	return f.stopErr
}

func (f *fakeServiceDocker) Start(_ context.Context, name string) error {
	f.calls = append(f.calls, "start:"+name)
	f.started = true
	return f.startErr
}

func (f *fakeServiceDocker) Restart(_ context.Context, name string, _ time.Duration) error {
	f.calls = append(f.calls, "restart:"+name)
	return f.restartErr
}

func (f *fakeServiceDocker) WaitRunning(_ context.Context, name string, _ time.Duration) error {
	f.calls = append(f.calls, "waitRunning:"+name)
	return nil
}

func (f *fakeServiceDocker) Health(_ context.Context, name string) (model.Health, error) {
	f.calls = append(f.calls, "health:"+name)
	return model.Health{Running: true}, nil
}

func (f *fakeServiceDocker) Remove(_ context.Context, name string) error {
	f.calls = append(f.calls, "remove:"+name)
	return f.removeErr
}

func (f *fakeServiceDocker) Pull(_ context.Context, image string) error {
	f.calls = append(f.calls, "pull:"+image)
	return f.pullErr
}

// PullWithAuth records the same "pull:" call as Pull, so call-order assertions
// hold for both.
func (f *fakeServiceDocker) PullWithAuth(_ context.Context, image, _ string) error {
	f.calls = append(f.calls, "pull:"+image)
	return f.pullErr
}

func (f *fakeServiceDocker) ImageID(_ context.Context, ref string) (string, error) {
	f.calls = append(f.calls, "imageID:"+ref)
	return f.imageID, f.imageIDErr
}

func (f *fakeServiceDocker) ImageRemove(_ context.Context, id string) error {
	f.calls = append(f.calls, "imageRemove:"+id)
	return f.imageRemoveErr
}

func (f *fakeServiceDocker) Exec(_ context.Context, name string, cmd []string) error {
	f.calls = append(f.calls, "exec:"+name)
	return nil
}

func (f *fakeServiceDocker) ExecOutput(_ context.Context, name string, _ []string, max int) (string, int, error) {
	f.calls = append(f.calls, "execOutput:"+name)
	out := f.execOut
	if len(out) > max {
		out = out[:max]
	}
	return out, f.execExit, f.execErr
}

func (f *fakeServiceDocker) ExecStdin(_ context.Context, name string, _ []string, stdin io.Reader, _ int) (string, int, error) {
	f.calls = append(f.calls, "execStdin:"+name)
	fed, err := io.ReadAll(stdin)
	if err != nil {
		return "", 0, err
	}
	f.execFed = string(fed)
	return f.execStderr, f.execExit, f.execErr
}

func (f *fakeServiceDocker) CreateAndStart(_ context.Context, in model.Inspect, start bool) error {
	f.calls = append(f.calls, "createAndStart:"+in.Name)
	f.createdIn = in
	f.createdStart = start
	if f.createErrName != "" && strings.TrimPrefix(in.Name, "/") == f.createErrName {
		return errors.New("dockercli: create failed for " + f.createErrName)
	}
	return f.createErr
}

func (f *fakeServiceDocker) InspectName(_ context.Context, name string) (string, error) {
	f.calls = append(f.calls, "inspectName:"+name)
	return f.liveName, f.inspNameErr
}

func (f *fakeServiceDocker) Self(_ context.Context) (string, error) {
	f.calls = append(f.calls, "self")
	return f.selfName, nil
}

func (f *fakeServiceDocker) Allocations(_ context.Context) ([]model.Allocation, error) {
	f.calls = append(f.calls, "allocations")
	return f.allocations, f.allocErr
}

// fakeVirsh is a no-op virshcli.Virsh.
type fakeVirsh struct{}

var _ virshcli.Virsh = fakeVirsh{}

func (fakeVirsh) List(_ context.Context) ([]virshcli.VMInfo, error)   { return nil, nil }
func (fakeVirsh) State(_ context.Context, _ string) (string, error)   { return "", nil }
func (fakeVirsh) DumpXML(_ context.Context, _ string) (string, error) { return "<domain/>", nil }
func (fakeVirsh) DumpXMLInactive(_ context.Context, _ string) (string, error) {
	return "<domain/>", nil
}
func (fakeVirsh) Shutdown(_ context.Context, _ string) error          { return nil }
func (fakeVirsh) Destroy(_ context.Context, _ string) error           { return nil }
func (fakeVirsh) Start(_ context.Context, _ string) error             { return nil }
func (fakeVirsh) Define(_ context.Context, _ string) error            { return nil }
func (fakeVirsh) Undefine(_ context.Context, _ string) error          { return nil }
func (fakeVirsh) Autostart(_ context.Context, _ string, _ bool) error { return nil }
func (fakeVirsh) IsActive(_ context.Context, _ string) (bool, error)  { return false, nil }
func (fakeVirsh) SnapshotCreateDiskOnly(_ context.Context, _, _ string, _ bool, _ []string) error {
	return nil
}
func (fakeVirsh) BlockCommitActivePivot(_ context.Context, _, _ string) error { return nil }
func (fakeVirsh) GuestAgentPing(_ context.Context, _ string) bool             { return false }
