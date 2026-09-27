package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// checkEngine answers the reads a restore check makes.
type checkEngine struct {
	ResticEngine
	openErr error
	snaps   []restic.Snapshot
	preview map[string][]restic.PreviewItem
	hook    func(ctx context.Context) error
	steps   []restic.PreviewStep
	size    int64
}

func (e *checkEngine) RepoOpensErr(context.Context, string, restic.Mode) error { return e.openErr }

func (e *checkEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return e.snaps, nil
}

func (e *checkEngine) StatsRestoreSize(context.Context, string, string, restic.Mode) (int, int64, error) {
	return 1, e.size, nil
}

func (e *checkEngine) RestorePreview(ctx context.Context, _ string, st restic.PreviewStep, _ restic.Mode, onItem func(restic.PreviewItem)) error {
	e.steps = append(e.steps, st)
	if e.hook != nil {
		return e.hook(ctx)
	}
	for _, it := range e.preview[st.Target] {
		onItem(it)
	}
	return nil
}

// checkDocker lists containers with their binds and inspects the live one.
type checkDocker struct {
	dockercli.Docker
	list []dockercli.ContainerInfo
	live model.Inspect
	self string
}

func (d *checkDocker) List(context.Context) ([]dockercli.ContainerInfo, error) { return d.list, nil }

func (d *checkDocker) Inspect(context.Context, string) (model.Inspect, error) { return d.live, nil }

func (d *checkDocker) Self(context.Context) (string, error) { return d.self, nil }

// checkFixture is a container "plex" backed up from <mount>/appdata/plex, whose
// host path is /mnt/user/appdata/plex.
type checkFixture struct {
	svc     *Service
	eng     *checkEngine
	docker  *checkDocker
	appdata string
	free    uint64
}

func newCheckFixture(t *testing.T) *checkFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("an in-place restore needs a slash-absolute host mount (paths.Within needs a leading /)")
	}
	dir := filepath.ToSlash(t.TempDir())
	st := newTestStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	repo := dir + "/backups/containers"
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repo+"/config", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	appdata := dir + "/appdata/plex"
	if err := os.MkdirAll(appdata, 0o750); err != nil {
		t.Fatal(err)
	}
	backedUp := model.Inspect{Name: "plex", Config: model.Config{Image: "plexinc/pms:1.40", Env: []string{"TZ=Europe/Vienna", "PLEX_CLAIM=secret-a"}}}
	backedUp.HostConfig.Binds = []string{"/mnt/user/appdata/plex:/config"}
	backedUp.HostConfig.PortBindings = map[string][]model.PortBinding{"32400/tcp": {{HostPort: "32400"}}}
	def, err := json.Marshal(containerDefinition{Inspect: backedUp})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{appdata}, Definition: string(def)}); err != nil {
		t.Fatal(err)
	}
	live := backedUp
	live.Config.Image = "plexinc/pms:1.41"
	live.Config.Env = []string{"TZ=Europe/Vienna", "PLEX_CLAIM=secret-b", "EXTRA=1"}
	live.HostConfig.PortBindings = nil
	f := &checkFixture{
		eng: &checkEngine{snaps: []restic.Snapshot{
			{ID: "aaaa1111", Tags: []string{"container:plex"}, Paths: []string{appdata}},
			{ID: "bbbb2222", Tags: []string{"container:sonarr"}, Paths: []string{dir + "/appdata/sonarr"}},
		}},
		docker: &checkDocker{live: live, self: "BombVault", list: []dockercli.ContainerInfo{
			{Name: "plex", Mounts: []dockercli.MountPoint{{Source: "/mnt/user/appdata/plex", Destination: "/config"}}},
			{Name: "nextcloud-db", Mounts: []dockercli.MountPoint{{Source: "/mnt/cache/appdata/plex/transcode", Destination: "/t"}}},
			{Name: "krusader", Mounts: []dockercli.MountPoint{{Source: "/mnt/user", Destination: "/media"}}},
			{Name: "BombVault", Mounts: []dockercli.MountPoint{{Source: "/mnt/user/appdata/plex", Destination: "/x"}}},
			{Name: "sonarr", Mounts: []dockercli.MountPoint{{Source: "/mnt/user/appdata/plexamp", Destination: "/config"}}},
		}},
		appdata: appdata,
		free:    1 << 30,
	}
	f.svc = &Service{
		cfg:    config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir, HostSourceRoot: "/mnt/user"},
		store:  st,
		docker: f.docker,
		engine: f.eng,
	}
	f.svc.diskStat = func(string) (diskStatResult, error) {
		return diskStatResult{Free: f.free, Volume: "dev:1"}, nil
	}
	return f
}

func (f *checkFixture) check(t *testing.T, req RestoreCheckRequest) RestoreCheck {
	t.Helper()
	if req.Source == "" {
		req.Source = "local"
	}
	res, err := f.svc.CheckRestore(context.Background(), req)
	if err != nil {
		t.Fatalf("CheckRestore: %v", err)
	}
	return res
}

func lineOf(t *testing.T, res RestoreCheck, id string) CheckLine {
	t.Helper()
	for _, c := range res.Checks {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no %s line in %+v", id, res.Checks)
	return CheckLine{}
}

func TestCheckRestoreReportsWhatAnInPlaceRestoreChanges(t *testing.T) {
	f := newCheckFixture(t)
	if err := os.WriteFile(f.appdata+"/grown.db", make([]byte, 10), 0o600); err != nil {
		t.Fatal(err)
	}
	f.eng.preview = map[string][]restic.PreviewItem{f.appdata: {
		{Action: "restored", Item: "/new.conf", Size: 100},
		{Action: "updated", Item: "/grown.db", Size: 50},
		{Action: "unchanged", Item: "/same", Size: 7},
		{Action: "deleted", Item: "/leftover.log"},
		{Action: "restored", Item: "/"},
	}}
	res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111"})

	if !res.Ready {
		t.Fatalf("a restore that fits must be ready: %+v", res.Checks)
	}
	for _, id := range []string{lineRepository, lineKey, lineSnapshot, lineSpace} {
		if c := lineOf(t, res, id); c.Status != lineOK {
			t.Errorf("%s = %+v, want ok", id, c)
		}
	}
	// The new file whole, the replaced one only by what it grows.
	if c := lineOf(t, res, lineSpace); c.Need != 140 {
		t.Errorf("need = %d, want 140", c.Need)
	}
	p := res.Plan
	if p == nil || p.Added != 1 || p.Changed != 1 || p.Unchanged != 1 || p.Extra != 1 {
		t.Fatalf("plan counts = %+v", p)
	}
	want := []PlanFile{
		{Path: "/mnt/user/appdata/plex/new.conf", Change: changeAdded},
		{Path: "/mnt/user/appdata/plex/grown.db", Change: changeChanged},
		{Path: "/mnt/user/appdata/plex/leftover.log", Change: changeExtra},
	}
	if !slices.Equal(p.Files, want) {
		t.Errorf("files = %+v, want %+v", p.Files, want)
	}
	if len(f.eng.steps) != 1 || f.eng.steps[0].Subtree != f.appdata || f.eng.steps[0].Target != f.appdata {
		t.Errorf("the dry run must replay the restore's own call, got %+v", f.eng.steps)
	}
}

func TestCheckRestoreComparesTheDefinitionWithoutVariableValues(t *testing.T) {
	f := newCheckFixture(t)
	res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111"})
	want := []DefinitionChange{
		{Field: "image", Change: changeChanged, Backup: "plexinc/pms:1.40", Now: "plexinc/pms:1.41"},
		{Field: "port", Change: changeAdded, Backup: "32400 -> 32400/tcp"},
		{Field: "env", Name: "EXTRA", Change: changeRemoved},
		{Field: "env", Name: "PLEX_CLAIM", Change: changeChanged},
	}
	if !slices.Equal(res.Plan.Definition, want) {
		t.Fatalf("definition = %+v, want %+v", res.Plan.Definition, want)
	}
	blob, _ := json.Marshal(res.Plan.Definition)
	if strings.Contains(string(blob), "secret") {
		t.Fatalf("a variable's value reached the plan: %s", blob)
	}
}

func TestCheckRestoreSaysWhenTheContainerIsGone(t *testing.T) {
	f := newCheckFixture(t)
	f.docker.list = nil
	res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111"})
	if !res.Plan.Missing || len(res.Plan.Definition) != 0 {
		t.Fatalf("plan = %+v", res.Plan)
	}
}

func TestCheckRestoreNamesContainersSharingTheTarget(t *testing.T) {
	f := newCheckFixture(t)
	res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111"})
	// nextcloud-db reaches in through the cache pool; the owner, BombVault
	// itself, a whole-share bind and a lookalike name do not count.
	want := []SharedFolder{{Path: "/mnt/user/appdata/plex", Containers: []string{"nextcloud-db"}}}
	if len(res.Plan.Shared) != 1 || res.Plan.Shared[0].Path != want[0].Path || !slices.Equal(res.Plan.Shared[0].Containers, want[0].Containers) {
		t.Fatalf("shared = %+v, want %+v", res.Plan.Shared, want)
	}
	if !res.Ready {
		t.Fatal("a shared folder warns and does not block")
	}
}

func TestCheckRestoreBlocksWhenTheTargetIsTooSmall(t *testing.T) {
	f := newCheckFixture(t)
	f.free = 50
	f.eng.preview = map[string][]restic.PreviewItem{f.appdata: {{Action: "restored", Item: "/big", Size: 100}}}
	res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111"})
	c := lineOf(t, res, lineSpace)
	if res.Ready || c.Status != lineFail || c.Reason != reasonShort || c.Need != 100 || c.Free != 50 {
		t.Fatalf("ready=%v space=%+v", res.Ready, c)
	}
}

func TestCheckRestoreWrongKeyStillReachesTheRepository(t *testing.T) {
	f := newCheckFixture(t)
	f.eng.openErr = errors.New("Fatal: wrong password or no key found")
	res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111"})
	if res.Ready || lineOf(t, res, lineRepository).Status != lineOK || lineOf(t, res, lineKey).Status != lineFail {
		t.Fatalf("checks = %+v", res.Checks)
	}
	if lineOf(t, res, lineSnapshot).Status != lineSkip || len(f.eng.steps) != 0 || res.Plan != nil {
		t.Fatal("nothing past an unreadable repository may run")
	}
}

func TestCheckRestoreUnreachableRepository(t *testing.T) {
	f := newCheckFixture(t)
	f.eng.openErr = errors.New("Fatal: unable to open config file: dial tcp: connection refused")
	res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111"})
	if res.Ready || lineOf(t, res, lineRepository).Status != lineFail || lineOf(t, res, lineKey).Status != lineSkip {
		t.Fatalf("checks = %+v", res.Checks)
	}
}

func TestCheckRestoreRefusesAnotherContainersSnapshot(t *testing.T) {
	f := newCheckFixture(t)
	res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "bbbb2222"})
	c := lineOf(t, res, lineSnapshot)
	if res.Ready || c.Status != lineFail || !strings.Contains(c.Detail, "does not belong") {
		t.Fatalf("snapshot = %+v", c)
	}
}

func TestCheckRestoreToFolderCreatesNothing(t *testing.T) {
	f := newCheckFixture(t)
	res := f.check(t, RestoreCheckRequest{Kind: checkContainerTo, Name: "plex", SnapshotID: "aaaa1111", TargetPath: "restore/plex"})
	if !res.Ready {
		t.Fatalf("checks = %+v", res.Checks)
	}
	if _, err := os.Stat(f.svc.cfg.HostMountRoot + "/restore/plex"); !os.IsNotExist(err) {
		t.Fatalf("a check created the target folder: %v", err)
	}
	if res.Plan.InPlace || len(res.Plan.Shared) != 0 {
		t.Fatalf("a restore into a new folder writes over nothing: %+v", res.Plan)
	}
}

func TestCheckRestoreStopsAtTheBudgetAndSaysSo(t *testing.T) {
	f := newCheckFixture(t)
	old := previewBudget
	previewBudget = 20 * time.Millisecond
	t.Cleanup(func() { previewBudget = old })
	f.eng.hook = func(ctx context.Context) error {
		<-ctx.Done()
		return fmt.Errorf("restic restore: %w", ctx.Err())
	}
	res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111"})
	if !res.Plan.Partial {
		t.Fatalf("plan = %+v", res.Plan)
	}
	// A measurement that stopped early proves no fit, and no shortfall either.
	if c := lineOf(t, res, lineSpace); c.Status != lineSkip || c.Reason != reasonUnmeasured || !res.Ready {
		t.Fatalf("space = %+v ready=%v", c, res.Ready)
	}
}

func TestCheckRestoreCapsTheListNotTheCounts(t *testing.T) {
	f := newCheckFixture(t)
	var items []restic.PreviewItem
	for i := range planListCap + 50 {
		items = append(items, restic.PreviewItem{Action: "restored", Item: fmt.Sprintf("/f%d", i), Size: 1})
	}
	f.eng.preview = map[string][]restic.PreviewItem{f.appdata: items}
	res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111"})
	if res.Plan.Added != planListCap+50 || len(res.Plan.Files) != planListCap || !res.Plan.ListCapped {
		t.Fatalf("added=%d listed=%d capped=%v", res.Plan.Added, len(res.Plan.Files), res.Plan.ListCapped)
	}
}

func TestCheckRestoreFlashOnlyDownloads(t *testing.T) {
	f := newCheckFixture(t)
	settings, _ := f.svc.store.GetSettings()
	settings.FlashPath = "backups/containers"
	if err := f.svc.store.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	res := f.check(t, RestoreCheckRequest{Kind: checkFlash, SnapshotID: "latest"})
	if c := lineOf(t, res, lineSpace); c.Status != lineSkip || c.Reason != reasonDownload {
		t.Fatalf("space = %+v", c)
	}
	if res.Plan != nil || !res.Ready {
		t.Fatalf("a download has no plan and nothing to block: %+v", res)
	}
}

func TestCheckRestoreRejectsAnUnknownKind(t *testing.T) {
	f := newCheckFixture(t)
	if _, err := f.svc.CheckRestore(context.Background(), RestoreCheckRequest{Kind: "stack"}); err == nil {
		t.Fatal("an unknown kind must be an error")
	}
}

func TestBindShares(t *testing.T) {
	for _, tc := range []struct {
		target, bind string
		want         bool
	}{
		{"/mnt/user/appdata/plex", "/mnt/user/appdata/plex/transcode", true},
		{"/mnt/user/appdata/plex/db", "/mnt/user/appdata/plex", true},
		{"/mnt/user/appdata/plex", "/mnt/cache/appdata/plex", true},
		{"/mnt/cache/appdata/plex", "/mnt/user0/appdata/plex/x", true},
		{"/mnt/user/appdata/plex", "/mnt/user/appdata/plexamp", false},
		{"/mnt/user/appdata/plex", "/mnt/user/appdata", false},
		{"/mnt/user/appdata/plex", "/mnt/user", false},
		{"/mnt/cache/appdata/plex", "/mnt/zfs/appdata/plex", false},
		{"/srv/data/app", "/srv/data/app/sub", true},
		{"/srv/data/app", "/srv", false},
	} {
		if got := bindShares(tc.target, tc.bind); got != tc.want {
			t.Errorf("bindShares(%q, %q) = %v, want %v", tc.target, tc.bind, got, tc.want)
		}
	}
}

func TestVMChanges(t *testing.T) {
	backup := `<domain><memory unit='KiB'>8388608</memory><vcpu>4</vcpu><devices>
<disk device='disk'><source file='/mnt/user/domains/win/vdisk1.img'/><target dev='hdc'/></disk>
<interface type='bridge'><mac address='52:54:00:aa:bb:cc'/><source bridge='br0'/></interface></devices></domain>`
	live := `<domain><memory unit='GiB'>16</memory><vcpu>4</vcpu><devices>
<disk device='disk'><source file='/mnt/user/domains/win/vdisk1.img'/><target dev='hdc'/></disk>
<disk device='disk'><source file='/mnt/user/domains/win/vdisk2.img'/><target dev='hdd'/></disk>
<interface type='bridge'><mac address='52:54:00:aa:bb:cc'/><source bridge='br0.20'/></interface></devices></domain>`
	want := []DefinitionChange{
		{Field: "memory", Change: changeChanged, Backup: "8192 MiB", Now: "16384 MiB"},
		{Field: "disk", Change: changeRemoved, Now: "hdd: /mnt/user/domains/win/vdisk2.img"},
		{Field: "network", Change: changeAdded, Backup: "52:54:00:aa:bb:cc (br0)"},
		{Field: "network", Change: changeRemoved, Now: "52:54:00:aa:bb:cc (br0.20)"},
	}
	if got := vmChanges(backup, live); !slices.Equal(got, want) {
		t.Fatalf("vmChanges = %+v, want %+v", got, want)
	}
}

// zfsCheckEngine adds the check's reads to the ZFS restore fake.
type zfsCheckEngine struct {
	*zfsFakeEngine
	steps []restic.PreviewStep
}

func (e *zfsCheckEngine) RepoOpensErr(context.Context, string, restic.Mode) error { return nil }

func (e *zfsCheckEngine) RestorePreview(_ context.Context, _ string, st restic.PreviewStep, _ restic.Mode, _ func(restic.PreviewItem)) error {
	e.steps = append(e.steps, st)
	return nil
}

func TestCheckRestoreZFSTakesNoSafetySnapshot(t *testing.T) {
	s, st, host, eng := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	check := &zfsCheckEngine{zfsFakeEngine: eng}
	s.engine = check
	s.diskStat = func(string) (diskStatResult, error) { return diskStatResult{Free: 1 << 30, Volume: "pool:tank"}, nil }

	res, err := s.CheckRestore(context.Background(), RestoreCheckRequest{Kind: checkZFS, Name: d.ID, Source: "local", ZFS: zfsRestoreRequest(zfsRoot)})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Ready {
		t.Fatalf("checks = %+v", res.Checks)
	}
	if hostDid(host, "snapshot ") {
		t.Fatalf("a check took a snapshot: %v", host.recorded())
	}
	target := zfsMemberPath(s.cfg.HostMountRoot, zfsRoot)
	if len(check.steps) != 1 || check.steps[0].Target != target || check.steps[0].SnapshotID != zfsRootSnapID {
		t.Fatalf("steps = %+v, want the root member into %s", check.steps, target)
	}
	// The child dataset mounted inside the target is left out, as the restore leaves it.
	if len(check.steps[0].Excludes) != 1 || check.steps[0].Excludes[0] != strings.TrimPrefix(zfsChild, zfsRoot) {
		t.Fatalf("excludes = %v", check.steps[0].Excludes)
	}
}

// stackFixture puts plex and a second backed-up container, plexdb, into the
// compose project "media". plexdb binds a folder inside plex's appdata.
func stackFixture(t *testing.T) *checkFixture {
	t.Helper()
	f := newCheckFixture(t)
	root := f.svc.cfg.HostMountRoot
	for _, name := range []string{"plex", "plexdb"} {
		in := model.Inspect{Name: name, Config: model.Config{Image: "img/" + name + ":1", Labels: map[string]string{"com.docker.compose.project": "media"}}}
		def, err := json.Marshal(containerDefinition{Inspect: in})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.store.UpsertTarget(store.Target{ContainerName: name, AppdataPaths: []string{root + "/appdata/" + name}, Definition: string(def)}); err != nil {
			t.Fatal(err)
		}
	}
	f.eng.snaps = append(f.eng.snaps, restic.Snapshot{ID: "cccc3333", Tags: []string{"container:plexdb"}, Paths: []string{root + "/appdata/plexdb"}})
	f.docker.list = append(f.docker.list, dockercli.ContainerInfo{Name: "plexdb", Mounts: []dockercli.MountPoint{{Source: "/mnt/user/appdata/plex/db", Destination: "/db"}}})
	return f
}

func TestCheckRestoreStackChecksEveryMember(t *testing.T) {
	f := stackFixture(t)
	f.free = 150
	f.eng.preview = map[string][]restic.PreviewItem{
		f.appdata: {{Action: "restored", Item: "/a", Size: 100}},
		f.svc.cfg.HostMountRoot + "/appdata/plexdb": {{Action: "restored", Item: "/b", Size: 200}},
	}
	res := f.check(t, RestoreCheckRequest{Kind: checkStack, Name: "media"})
	if len(res.Members) != 2 || res.Members[0].Name != "plex" || res.Members[1].Name != "plexdb" {
		t.Fatalf("members = %+v", res.Members)
	}
	if !res.Members[0].Ready || res.Members[1].Ready || res.Ready {
		t.Fatalf("plex fits and plexdb does not, so the stack is not ready: %+v", res)
	}
	if c := lineOf(t, RestoreCheck{Checks: res.Members[1].Checks}, lineSpace); c.Reason != reasonShort {
		t.Fatalf("plexdb space = %+v", c)
	}
}

func TestCheckRestoreStackLeavesItsOwnMembersOutOfTheSharedFolders(t *testing.T) {
	f := stackFixture(t)
	res := f.check(t, RestoreCheckRequest{Kind: checkStack, Name: "media"})
	shared := res.Members[0].Plan.Shared
	if len(shared) != 1 || !slices.Equal(shared[0].Containers, []string{"nextcloud-db"}) {
		t.Fatalf("plex shared = %+v, want only nextcloud-db", shared)
	}
}

func TestCheckRestoreStackWithoutBackupsIsAnError(t *testing.T) {
	f := newCheckFixture(t)
	if _, err := f.svc.CheckRestore(context.Background(), RestoreCheckRequest{Kind: checkStack, Name: "nothing", Source: "local"}); err == nil {
		t.Fatal("a stack with no backed-up member has nothing to check")
	}
}
