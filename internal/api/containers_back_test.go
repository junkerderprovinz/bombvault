package api_test

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// sharingDocker keeps a run state per container and refuses to start one whose
// network owner is down, as Docker does.
type sharingDocker struct {
	*fakeServiceDocker
	byID    map[string]string
	running map[string]bool
	order   []string
}

func newSharingDocker(containers ...model.Inspect) *sharingDocker {
	d := &sharingDocker{
		fakeServiceDocker: &fakeServiceDocker{inspects: map[string]model.Inspect{}},
		byID:              map[string]string{},
		running:           map[string]bool{},
	}
	for _, in := range containers {
		name := strings.TrimPrefix(in.Name, "/")
		d.inspects[name], d.inspects[in.ID] = in, in
		d.byID[in.ID], d.byID[name] = name, name
		d.running[name] = in.Running
		state := "exited"
		if in.Running {
			state = "running"
		}
		d.listOut = append(d.listOut, dockercli.ContainerInfo{ID: in.ID, Name: name, State: state})
	}
	return d
}

func (d *sharingDocker) Inspect(ctx context.Context, name string) (model.Inspect, error) {
	in, err := d.fakeServiceDocker.Inspect(ctx, name)
	in.Running = d.running[d.byID[name]]
	return in, err
}

func (d *sharingDocker) Stop(ctx context.Context, name string, x time.Duration) error {
	d.running[d.byID[name]] = false
	return d.fakeServiceDocker.Stop(ctx, name, x)
}

func (d *sharingDocker) Start(ctx context.Context, ref string) error {
	name := d.byID[ref]
	d.order = append(d.order, name)
	if err := d.fakeServiceDocker.Start(ctx, ref); err != nil {
		return err
	}
	owner, shared := strings.CutPrefix(d.inspects[name].HostConfig.NetworkMode, "container:")
	if shared && !d.running[owner] {
		return errors.New("cannot join network namespace of a non running container")
	}
	d.running[name] = true
	return nil
}

func (d *sharingDocker) Health(ctx context.Context, ref string) (model.Health, error) {
	_, _ = d.fakeServiceDocker.Health(ctx, ref)
	return model.Health{Running: d.running[d.byID[ref]]}, nil
}

func sharingContainer(name, owner string, running bool) model.Inspect {
	in := model.Inspect{Name: "/" + name, ID: "id-" + name, Image: "sha256:" + name, Running: running, Config: model.Config{Image: name + ":1"}}
	if owner != "" {
		in.HostConfig.NetworkMode = "container:" + owner
	}
	return in
}

func sharingService(t *testing.T, d *sharingDocker) (*api.Service, *store.Repo) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root, HostSourceRoot: "/mnt", DataRootSegments: []string{"appdata"}}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	// These tests pin the order of the starts, and each wait would cost the
	// four second grace of a container without a healthcheck.
	s.RestartHealthWait = false
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	return api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{}), st
}

func lastBackupRun(t *testing.T, st *store.Repo, name string) *store.Run {
	t.Helper()
	tg, err := st.GetTargetByContainer(name)
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.LastRunOfKind(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestATargetJoiningAContainerOnItsStopListComesBackAfterIt(t *testing.T) {
	d := newSharingDocker(sharingContainer("qbit", "vpn", true), sharingContainer("vpn", "", true))
	svc, st := sharingService(t, d)
	if err := svc.SetStopContainers(context.Background(), "qbit", []string{"vpn"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Backup(context.Background(), "qbit"); err != nil {
		t.Fatalf("Backup: %v, starts %v", err, d.order)
	}
	if !d.running["qbit"] || !d.running["vpn"] {
		t.Fatalf("running = %v, starts %v", d.running, d.order)
	}
	if run := lastBackupRun(t, st, "qbit"); run.Status != "success" || run.Error != "" {
		t.Fatalf("run = %q %q", run.Status, run.Error)
	}
}

func TestContainersSharingATargetsNetworkComeBackOnItsNewOne(t *testing.T) {
	d := newSharingDocker(
		sharingContainer("gluetun", "", true),
		sharingContainer("peer", "gluetun", true),
		sharingContainer("off", "gluetun", false),
		sharingContainer("other", "", true),
	)
	svc, st := sharingService(t, d)
	if _, err := svc.Backup(context.Background(), "gluetun"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(d.calls, "stop:id-peer") {
		t.Fatalf("the sharing container must go down with its network: %v", d.calls)
	}
	if !slices.Equal(d.order, []string{"gluetun", "peer"}) {
		t.Fatalf("starts = %v, want the owner, then its running sharer, and nothing else", d.order)
	}
	if run := lastBackupRun(t, st, "gluetun"); run.Status != "success" || run.Error != "" {
		t.Fatalf("run = %q %q", run.Status, run.Error)
	}
	tg, err := st.GetTargetByContainer("gluetun")
	if err != nil {
		t.Fatal(err)
	}
	if len(tg.StopContainers) != 0 {
		t.Fatalf("the user's stop list must stay theirs: %v", tg.StopContainers)
	}
}

func TestContainersSharingTheNetworkOfAStoppedContainerComeBackWithIt(t *testing.T) {
	d := newSharingDocker(
		sharingContainer("app", "", true),
		sharingContainer("vpn", "", true),
		sharingContainer("zzz", "vpn", true),
		sharingContainer("aaa", "zzz", true),
	)
	svc, _ := sharingService(t, d)
	if err := svc.SetStopContainers(context.Background(), "app", []string{"vpn"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Backup(context.Background(), "app"); err != nil {
		t.Fatal(err)
	}
	if !d.running["vpn"] || !d.running["zzz"] || !d.running["aaa"] {
		t.Fatalf("running = %v, starts %v", d.running, d.order)
	}
}

func TestBombVaultSharingATargetsNetworkIsNotStoppedAndTheRunSaysSo(t *testing.T) {
	d := newSharingDocker(sharingContainer("gluetun", "", true), sharingContainer("bombvault", "gluetun", true))
	d.selfName = "bombvault"
	svc, st := sharingService(t, d)
	if _, err := svc.Backup(context.Background(), "gluetun"); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(d.calls, "stop:id-bombvault") {
		t.Fatalf("BombVault must not stop itself: %v", d.calls)
	}
	if run := lastBackupRun(t, st, "gluetun"); run.Error != store.NoteNoNetworkUntilRestart+": bombvault" {
		t.Fatalf("note = %q", run.Error)
	}
}

// A database in the network of the target cannot start before it, whatever
// its kind says.
func TestADatabaseOnTheTargetsNetworkStartsAfterTheTarget(t *testing.T) {
	db := sharingContainer("db", "app", true)
	db.Config.Image = "postgres:16"
	db.Config.Env = []string{"POSTGRES_PASSWORD=x"}
	d := newSharingDocker(sharingContainer("app", "", true), db)
	svc, _ := sharingService(t, d)
	if err := svc.SetStopContainers(context.Background(), "app", []string{"db"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Backup(context.Background(), "app"); err != nil {
		t.Fatal(err)
	}
	if !d.running["db"] || !slices.Equal(d.order, []string{"app", "db"}) {
		t.Fatalf("starts = %v, running %v", d.order, d.running)
	}
}

// What a container that starts before the target joins has to start first too.
func TestTheOwnerOfAContainerStartingBeforeTheTargetStartsBeforeItToo(t *testing.T) {
	db := sharingContainer("db", "vpn", true)
	db.Config.Image = "postgres:16"
	db.Config.Env = []string{"POSTGRES_PASSWORD=x"}
	d := newSharingDocker(sharingContainer("app", "", true), db, sharingContainer("vpn", "", true))
	svc, _ := sharingService(t, d)
	if err := svc.SetStopContainers(context.Background(), "app", []string{"db", "vpn"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Backup(context.Background(), "app"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(d.order, []string{"vpn", "db", "app"}) {
		t.Fatalf("starts = %v", d.order)
	}
}

func TestAStopListRefusesBombVaultsOwnContainer(t *testing.T) {
	d := newSharingDocker(sharingContainer("app", "", true), sharingContainer("bombvault", "", true))
	d.selfName = "bombvault"
	svc, st := sharingService(t, d)
	if err := svc.SetStopContainers(context.Background(), "app", []string{"db", "bombvault"}); err == nil {
		t.Fatal("want a refusal")
	}
	if tg, err := st.GetTargetByContainer("app"); err == nil && len(tg.StopContainers) != 0 {
		t.Fatalf("stored = %v", tg.StopContainers)
	}
}

func TestABackupSkipsBombVaultInAnOlderStopList(t *testing.T) {
	d := newSharingDocker(sharingContainer("app", "", true), sharingContainer("bombvault", "", true))
	d.selfName = "bombvault"
	svc, st := sharingService(t, d)
	if _, err := st.UpsertTarget(store.Target{ContainerName: "app"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetStopContainers("app", []string{"bombvault"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Backup(context.Background(), "app"); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(d.calls, "stop:id-bombvault") {
		t.Fatalf("BombVault must not stop itself: %v", d.calls)
	}
}

func TestABootStartsANetworkOwnerBeforeTheContainersJoiningIt(t *testing.T) {
	d := newSharingDocker(sharingContainer("aaa-app", "vpn", false), sharingContainer("vpn", "", false))
	svc, st := sharingService(t, d)
	if err := st.MarkContainersStopped([]string{"aaa-app", "vpn"}, 1); err != nil {
		t.Fatal(err)
	}
	svc.RecoverContainerRestarts(context.Background())
	if !d.running["aaa-app"] || !d.running["vpn"] {
		t.Fatalf("starts = %v, running %v", d.order, d.running)
	}
	if left, _ := st.ListContainersStopped(); len(left) != 0 {
		t.Fatalf("still recorded: %v", left)
	}
}
