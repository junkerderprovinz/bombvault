package api

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// startTestDocker plays the Docker daemon for start tests. Its embedded
// Docker is nil, so any call that could touch the original container (stop,
// remove, recreate) panics the test.
type startTestDocker struct {
	dockercli.Docker
	mu        sync.Mutex
	states    []dockercli.IsolatedState
	proberRC  int
	networks  []string
	started   []dockercli.IsolatedSpec
	removedC  []string
	removedN  []string
	probers   []dockercli.ProberSpec
	leftoverC []string
	leftoverN []string
}

func (d *startTestDocker) Inspect(context.Context, string) (model.Inspect, error) {
	return model.Inspect{Image: "sha256:bombvault"}, nil
}

func (d *startTestDocker) CreateTestNetwork(_ context.Context, name string) error {
	d.networks = append(d.networks, name)
	return nil
}

func (d *startTestDocker) RemoveTestNetwork(_ context.Context, name string) error {
	d.removedN = append(d.removedN, name)
	return nil
}

func (d *startTestDocker) StartIsolated(_ context.Context, spec dockercli.IsolatedSpec) error {
	d.started = append(d.started, spec)
	return nil
}

func (d *startTestDocker) IsolatedState(context.Context, string) (dockercli.IsolatedState, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	st := d.states[0]
	if len(d.states) > 1 {
		d.states = d.states[1:]
	}
	return st, nil
}

func (d *startTestDocker) RemoveTestContainer(_ context.Context, name string) error {
	d.removedC = append(d.removedC, name)
	return nil
}

func (d *startTestDocker) RunProber(_ context.Context, spec dockercli.ProberSpec) (int, error) {
	d.probers = append(d.probers, spec)
	return d.proberRC, nil
}

func (d *startTestDocker) StartTestLeftovers(context.Context) ([]string, []string, error) {
	return d.leftoverC, d.leftoverN, nil
}

// startTestEngine restores nothing but records where it was asked to.
type startTestEngine struct {
	ResticEngine
	restored []string
	err      error
	calls    []string
}

func (e *startTestEngine) Unlock(_ context.Context, _ string, removeAll bool, _ restic.Mode) error {
	if removeAll {
		e.calls = append(e.calls, "Unlock(all)")
	} else {
		e.calls = append(e.calls, "Unlock")
	}
	return nil
}

func (e *startTestEngine) StatsRestoreSize(context.Context, string, string, restic.Mode) (int, int64, error) {
	return 0, 0, nil
}

func (e *startTestEngine) RestoreAll(_ context.Context, _, snapshotID, target string, _ restic.Mode, _ ...string) error {
	e.restored = append(e.restored, snapshotID+"->"+target)
	e.calls = append(e.calls, "RestoreAll")
	return e.err
}

func init() {
	startTestPoll = time.Millisecond
	startTestSettle = time.Millisecond
}

func newStartTestService(t *testing.T, d *startTestDocker, in model.Inspect) (*Service, store.Target, *startTestEngine) {
	t.Helper()
	st := newTestStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "backups/containers"
	settings.RestoreFolder = "restore"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	mountRoot := t.TempDir()
	def, err := json.Marshal(containerDefinition{Inspect: in, AppdataPaths: []string{path.Clean(mountRoot) + "/appdata/whoami"}})
	if err != nil {
		t.Fatal(err)
	}
	tg, err := st.UpsertTarget(store.Target{ContainerName: "whoami", Definition: string(def)})
	if err != nil {
		t.Fatal(err)
	}
	recordBackup(t, st, tg.ID, "abcd1234")
	eng := &startTestEngine{}
	s := &Service{
		store: st, engine: eng, docker: d,
		cfg:          config.Config{HostMountRoot: mountRoot, HostSourceRoot: "/mnt"},
		repoMu:       map[string]*sync.Mutex{"containers": {}},
		selfName:     "BombVault",
		selfResolved: true,
	}
	return s, tg, eng
}

func whoamiRecipe() model.Inspect {
	return model.Inspect{
		Name:   "/whoami",
		Config: model.Config{Image: "traefik/whoami"},
		HostConfig: model.HostConfig{
			Binds:        []string{"/mnt/appdata/whoami:/data:rw", "/mnt/media:/media", "cache:/cache"},
			PortBindings: map[string][]model.PortBinding{"80/tcp": {{HostPort: "8080"}}},
			NetworkMode:  "bridge",
		},
	}
}

func TestStartTestBlockerNamesWhatCannotRunInIsolation(t *testing.T) {
	cases := map[string]model.Inspect{
		"host-network":   {HostConfig: model.HostConfig{NetworkMode: "host"}},
		"privileged":     {HostConfig: model.HostConfig{Privileged: true}},
		"devices":        {HostConfig: model.HostConfig{Devices: []model.DeviceMapping{{PathOnHost: "/dev/dri"}}}},
		"host-namespace": {HostConfig: model.HostConfig{PidMode: "host"}},
		"depends-on":     {HostConfig: model.HostConfig{NetworkMode: "container:vpn"}},
		"":               {HostConfig: model.HostConfig{NetworkMode: "br0.20"}},
	}
	for want, in := range cases {
		if got := startTestBlocker(in); got != want {
			t.Errorf("blocker = %q, want %q for %+v", got, want, in.HostConfig)
		}
	}
	compose := model.Inspect{Config: model.Config{Labels: map[string]string{"com.docker.compose.depends_on": "db:service_started:false"}}}
	if got := startTestBlocker(compose); got != "depends-on" {
		t.Errorf("compose dependency: blocker = %q", got)
	}
}

func TestStartTestBindsOnlyTheRestoredData(t *testing.T) {
	s := &Service{cfg: config.Config{HostMountRoot: "/host/user", HostSourceRoot: "/mnt"}}
	def := containerDefinition{Inspect: whoamiRecipe(), AppdataPaths: []string{"/host/user/appdata/whoami"}}
	got := s.startTestBinds(def, "/host/user/restore/bombvault-starttest-whoami-1")
	want := []string{"/mnt/restore/bombvault-starttest-whoami-1/host/user/appdata/whoami:/data:rw"}
	if !slices.Equal(got, want) {
		t.Fatalf("binds = %v, want %v", got, want)
	}
}

func TestFirstTCPPortPicksTheLowestTCPPort(t *testing.T) {
	if got := firstTCPPort([]string{"8443/tcp", "53/udp", "443/tcp"}); got != "443" {
		t.Fatalf("port = %q", got)
	}
	if got := firstTCPPort([]string{"53/udp"}); got != "" {
		t.Fatalf("port = %q, want none", got)
	}
}

func TestStartTestPassesOnAHealthyCopyAndCleansUp(t *testing.T) {
	d := &startTestDocker{states: []dockercli.IsolatedState{
		{Running: true, Health: "starting"},
		{Running: true, Health: "healthy"},
	}}
	s, tg, eng := newStartTestService(t, d, whoamiRecipe())

	rec, err := s.RunStartTest(context.Background(), tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !rec.OK || rec.Method != "health" || rec.Trigger != "manual" {
		t.Fatalf("start test = %+v, want a pass by healthcheck", rec)
	}
	if len(eng.restored) != 1 || !strings.HasPrefix(eng.restored[0], "abcd1234->") {
		t.Fatalf("restores = %v", eng.restored)
	}
	spec := d.started[0]
	if !strings.HasPrefix(spec.Name, dockercli.StartTestPrefix) || spec.Network != d.networks[0] {
		t.Fatalf("copy %q on %q, networks %v", spec.Name, spec.Network, d.networks)
	}
	if spec.NanoCPUs == 0 || spec.MemoryBytes == 0 {
		t.Fatal("the copy must run with limits")
	}
	if len(spec.Binds) != 1 || strings.Contains(spec.Binds[0], "/mnt/appdata/whoami:") {
		t.Fatalf("binds = %v, want only the sandbox copy", spec.Binds)
	}
	if !slices.Equal(d.removedC, []string{spec.Name}) || !slices.Equal(d.removedN, []string{spec.Network}) {
		t.Fatalf("removed containers %v networks %v", d.removedC, d.removedN)
	}
	sandbox := strings.TrimPrefix(eng.restored[0], "abcd1234->")
	if _, err := os.Stat(sandbox); !os.IsNotExist(err) {
		t.Fatalf("the restored data must be gone, stat=%v", err)
	}
}

func TestStartTestRecordsACopyThatStops(t *testing.T) {
	d := &startTestDocker{states: []dockercli.IsolatedState{{Running: false, ExitCode: 3}}}
	s, tg, _ := newStartTestService(t, d, whoamiRecipe())
	rec, err := s.RunStartTest(context.Background(), tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.OK || !strings.Contains(rec.Detail, "exit code 3") {
		t.Fatalf("start test = %+v", rec)
	}
	if len(d.removedC) != 1 {
		t.Fatal("a failed copy is removed too")
	}
}

func TestStartTestChecksThePortWithoutAHealthcheck(t *testing.T) {
	d := &startTestDocker{states: []dockercli.IsolatedState{{Running: true, ExposedPorts: []string{"80/tcp"}}}}
	s, tg, _ := newStartTestService(t, d, whoamiRecipe())
	rec, err := s.RunStartTest(context.Background(), tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !rec.OK || rec.Method != "tcp" {
		t.Fatalf("start test = %+v", rec)
	}
	p := d.probers[0]
	if p.Network != d.networks[0] || p.Image != "sha256:bombvault" || p.Cmd[0] != "tcp-probe" || !strings.HasSuffix(p.Cmd[1], ":80") {
		t.Fatalf("prober = %+v", p)
	}

	d.states = []dockercli.IsolatedState{{Running: true, ExposedPorts: []string{"80/tcp"}}}
	d.proberRC = 1
	rec, _ = s.RunStartTest(context.Background(), tg.ID)
	if rec.OK || !strings.Contains(rec.Detail, "port 80") {
		t.Fatalf("start test = %+v, want a failure naming the port", rec)
	}
}

func TestStartTestRefusesAContainerThatCannotRunIsolated(t *testing.T) {
	in := whoamiRecipe()
	in.HostConfig.NetworkMode = "host"
	d := &startTestDocker{}
	s, tg, _ := newStartTestService(t, d, in)
	_, err := s.RunStartTest(context.Background(), tg.ID)
	var blocked startTestBlocked
	if !errors.As(err, &blocked) || blocked.Code != "host-network" {
		t.Fatalf("err = %v, want the host-network refusal", err)
	}
	if len(d.networks)+len(d.started) != 0 {
		t.Fatal("nothing may be created for a container that cannot be tested")
	}
}

func TestScheduledStartTestTakesTheLongestUntested(t *testing.T) {
	d := &startTestDocker{states: []dockercli.IsolatedState{{Running: true, Health: "healthy"}}}
	s, tested, _ := newStartTestService(t, d, whoamiRecipe())
	if err := s.store.AddStartTest(store.StartTest{TargetID: tested.ID, Container: "whoami", At: 100, OK: true}); err != nil {
		t.Fatal(err)
	}
	def, _ := json.Marshal(containerDefinition{Inspect: whoamiRecipe()})
	fresh, err := s.store.UpsertTarget(store.Target{ContainerName: "zeta", Definition: string(def)})
	if err != nil {
		t.Fatal(err)
	}
	recordBackup(t, s.store, fresh.ID, "ef567890")

	if err := s.runScheduledStartTest(context.Background()); err != nil {
		t.Fatal(err)
	}
	latest, _ := s.store.LatestStartTests()
	if latest[fresh.ID].Trigger != "schedule" {
		t.Fatalf("the never-tested container must go first, got %+v", latest)
	}
}

func TestCleanupRemovesOnlyStartTestLeftovers(t *testing.T) {
	d := &startTestDocker{leftoverC: []string{"bombvault-test-whoami-1"}, leftoverN: []string{"bombvault-test-net-1"}}
	s, _, _ := newStartTestService(t, d, whoamiRecipe())
	restore := filepath.Join(s.cfg.HostMountRoot, "restore")
	marked := filepath.Join(restore, "bombvault-starttest-whoami-1")
	unmarked := filepath.Join(restore, "bombvault-starttest-mine")
	for _, dir := range []string{marked, unmarked} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(marked, drillMarkerName), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	s.CleanupStartTestLeftovers(context.Background())

	if !slices.Equal(d.removedC, d.leftoverC) || !slices.Equal(d.removedN, d.leftoverN) {
		t.Fatalf("removed %v %v", d.removedC, d.removedN)
	}
	if _, err := os.Stat(marked); !os.IsNotExist(err) {
		t.Fatal("a marked leftover sandbox must go")
	}
	if _, err := os.Stat(unmarked); err != nil {
		t.Fatal("a folder without the marker must stay")
	}
}

func TestStartTestClearsStaleLocksBeforeItRestores(t *testing.T) {
	d := &startTestDocker{states: []dockercli.IsolatedState{{Running: true, Health: "healthy"}}}
	s, tg, eng := newStartTestService(t, d, whoamiRecipe())
	if _, err := s.RunStartTest(context.Background(), tg.ID); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(eng.calls, []string{"Unlock", "RestoreAll"}) {
		t.Fatalf("calls = %v, want stale locks cleared before the restore", eng.calls)
	}
}
