package api

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

func inPlaceDirs(paths ...string) []backup.RestoreDir {
	out := make([]backup.RestoreDir, len(paths))
	for i, p := range paths {
		out[i] = backup.RestoreDir{Subtree: p, Target: p}
	}
	return out
}

func TestBesideLayoutGivesAContainerFolderOneCopyForAllItsBinds(t *testing.T) {
	roots, dirs := besideLayout(inPlaceDirs(
		"/host/user/appdata/SnapOtter/conf",
		"/host/user/appdata/SnapOtter/data",
		"/host/user/appdata/nexterm",
	), besideRootOf)

	wantRoots := []besideRoot{
		{original: "/host/user/appdata/SnapOtter", copy: "/host/user/appdata/SnapOtter-restored"},
		{original: "/host/user/appdata/nexterm", copy: "/host/user/appdata/nexterm-restored"},
	}
	if !slices.Equal(roots, wantRoots) {
		t.Fatalf("roots = %+v, want %+v", roots, wantRoots)
	}
	wantDirs := []backup.RestoreDir{
		{Subtree: "/host/user/appdata/SnapOtter/conf", Target: "/host/user/appdata/SnapOtter-restored/conf"},
		{Subtree: "/host/user/appdata/SnapOtter/data", Target: "/host/user/appdata/SnapOtter-restored/data"},
		{Subtree: "/host/user/appdata/nexterm", Target: "/host/user/appdata/nexterm-restored"},
	}
	if !slices.Equal(dirs, wantDirs) {
		t.Fatalf("dirs = %+v, want %+v", dirs, wantDirs)
	}
}

func TestBesideLayoutNeverPutsACopyInsideAnOriginal(t *testing.T) {
	roots, dirs := besideLayout(inPlaceDirs("/host/data/app", "/host/data/app/cache"), besideRootOf)

	if len(roots) != 1 || roots[0].copy != "/host/data/app-restored" {
		t.Fatalf("roots = %+v, want the outer folder alone", roots)
	}
	if dirs[1].Target != "/host/data/app-restored/cache" {
		t.Fatalf("the inner folder goes to %s, want it inside the outer one's copy", dirs[1].Target)
	}
	for _, d := range dirs {
		if d.Target == "/host/data/app" || strings.HasPrefix(d.Target, "/host/data/app/") {
			t.Fatalf("%s lies inside the original", d.Target)
		}
	}
}

func TestBesideLayoutKeepsTheSnapshotFolderOfARenamedVM(t *testing.T) {
	roots, dirs := besideLayout([]backup.RestoreDir{
		{Subtree: "/host/domains/old", Target: "/host/domains/new"},
	}, func(dir string) string { return dir })

	if len(roots) != 1 || roots[0].original != "/host/domains/new" {
		t.Fatalf("roots = %+v", roots)
	}
	if dirs[0] != (backup.RestoreDir{Subtree: "/host/domains/old", Target: "/host/domains/new-restored"}) {
		t.Fatalf("dirs = %+v", dirs)
	}
}

func TestBesideBindsMovesTheCopiedFoldersAndKeepsTheRest(t *testing.T) {
	roots := []besideRoot{{original: "/mnt/user/appdata/immich", copy: "/mnt/user/appdata/immich-restored"}}
	binds, shared, err := besideBinds([]string{
		"/mnt/user/appdata/immich:/config",
		"/mnt/user/appdata/immich/db/:/db:rw,z",
		"/mnt/user/appdata/immichkit:/kit",
		"/mnt/user/fotos:/photos",
		"/etc/localtime:/etc/localtime:ro",
		"modelcache:/cache",
		"/mnt/user:/everything:ro",
	}, roots)
	if err != nil {
		t.Fatalf("besideBinds: %v", err)
	}
	want := []string{
		"/mnt/user/appdata/immich-restored:/config",
		"/mnt/user/appdata/immich-restored/db:/db:rw,z",
		"/mnt/user/appdata/immichkit:/kit",
		"/mnt/user/fotos:/photos",
		"/etc/localtime:/etc/localtime:ro",
		"modelcache:/cache",
		"/mnt/user:/everything:ro",
	}
	if !slices.Equal(binds, want) {
		t.Fatalf("binds = %q\nwant    %q", binds, want)
	}
	// Read-only volumes cannot be written through, so they are no sharing.
	if wantShared := []string{"/mnt/user/appdata/immichkit", "/mnt/user/fotos", "modelcache"}; !slices.Equal(shared, wantShared) {
		t.Fatalf("shared = %q, want %q", shared, wantShared)
	}
}

func TestBesideBindsRefusesACopyThatWouldRunOnTheOriginalsFiles(t *testing.T) {
	roots := []besideRoot{{original: "/mnt/user/appdata/immich", copy: "/mnt/user/appdata/immich-restored"}}
	for name, binds := range map[string][]string{
		"only a whole share leads to the folder":  {"/mnt/user/appdata:/appdata"},
		"the folder is bound through its pool":    {"/mnt/user/appdata/immich:/config", "/mnt/cache/appdata/immich/db:/db"},
		"nothing leads to the folder at all":      {"/mnt/user/media:/media"},
		"the pool path is the only one that does": {"/mnt/cache/appdata/immich:/config"},
	} {
		if _, _, err := besideBinds(binds, roots); !isDestinationRefusal(err) {
			t.Errorf("%s: err = %v, want a refusal", name, err)
		}
	}
}

func TestBesidePortsTakesTheNextPortNoContainerHolds(t *testing.T) {
	bindings := map[string][]model.PortBinding{
		"2283/tcp": {{HostPort: "2283"}},
		"2284/tcp": {{HostPort: "2284"}},
		"53/udp":   {{HostIP: "192.168.1.5", HostPort: "53"}},
		"9000/tcp": {{HostPort: ""}},
		"7000/tcp": {{HostPort: "7000-7010"}},
	}
	allocs := []model.Allocation{
		{Name: "immich", HostPorts: []string{"2283/tcp", "2284/tcp", "53/udp"}},
		{Name: "other", HostPorts: []string{"2285/tcp", "54/tcp"}},
	}

	got, view := besidePorts(bindings, allocs)

	if p := got["2283/tcp"][0].HostPort; p != "2286" {
		t.Errorf("2283 -> %s, want 2286: 2284 is the original's own and 2285 another container's", p)
	}
	if p := got["2284/tcp"][0].HostPort; p != "2287" {
		t.Errorf("2284 -> %s, want 2287, the next one the copy has not taken itself", p)
	}
	if b := got["53/udp"][0]; b.HostPort != "54" || b.HostIP != "192.168.1.5" {
		t.Errorf("53/udp -> %+v, want 54 on the same address: 54/tcp is another protocol", b)
	}
	if p := got["9000/tcp"][0].HostPort; p != "" {
		t.Errorf("an unpublished port got host port %s", p)
	}
	if p := got["7000/tcp"][0].HostPort; p != "" {
		t.Errorf("a port range got host port %s, want it left for Docker to pick", p)
	}
	want := []BesidePort{
		{Container: "2283/tcp", Original: "2283", Copy: "2286"},
		{Container: "2284/tcp", Original: "2284", Copy: "2287"},
		{Container: "53/udp", Original: "53", Copy: "54"},
		{Container: "7000/tcp", Original: "7000-7010", Copy: ""},
	}
	if !slices.Equal(view, want) {
		t.Fatalf("view = %+v\nwant   %+v", view, want)
	}
	if bindings["2283/tcp"][0].HostPort != "2283" {
		t.Fatal("the original's bindings were changed")
	}
}

func TestBesideLabelsMarkTheCopyAndLeaveItsComposeProject(t *testing.T) {
	got := besideLabels(map[string]string{
		"com.docker.compose.project": "media",
		"com.docker.compose.service": "immich",
		"net.unraid.docker.managed":  "dockerman",
	}, "immich", "abcd1234")

	want := map[string]string{
		"net.unraid.docker.managed": "dockerman",
		restoredFromLabel:           "immich",
		restoredSnapshotLabel:       "abcd1234",
	}
	if len(got) != len(want) {
		t.Fatalf("labels = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("labels = %v, want %v", got, want)
		}
	}
}

// besideCheckDocker adds what a restore next to the original asks Docker to
// the check's fake: which names are taken and which ports are held.
type besideCheckDocker struct {
	*checkDocker
	taken  map[string]bool
	allocs []model.Allocation
}

func (d *besideCheckDocker) InspectName(_ context.Context, name string) (string, error) {
	if d.taken[name] {
		return name, nil
	}
	return "", nil
}

func (d *besideCheckDocker) Allocations(context.Context) ([]model.Allocation, error) {
	return d.allocs, nil
}

// newBesideCheckFixture is the check fixture on a plain Docker host whose
// Host Data bind is mounted, with plex running on port 32400.
func newBesideCheckFixture(t *testing.T) (*checkFixture, *besideCheckDocker) {
	t.Helper()
	f := newCheckFixture(t)
	writeMountFixture(t, "/", f.svc.cfg.HostMountRoot)
	f.svc.SetPlatform(platform.Generic{})
	docker := &besideCheckDocker{
		checkDocker: f.docker,
		allocs:      []model.Allocation{{Name: "plex", HostPorts: []string{"32400/tcp"}}},
	}
	f.svc.docker = docker
	return f, docker
}

func besideOf(appdata string) string { return appdata + besideSuffix }

func TestCheckRestoreBesideDescribesTheCopyAndWritesNothing(t *testing.T) {
	f, _ := newBesideCheckFixture(t)
	if err := os.WriteFile(f.appdata+"/library.db", make([]byte, 10), 0o600); err != nil {
		t.Fatal(err)
	}
	f.eng.preview = map[string][]restic.PreviewItem{besideOf(f.appdata): {
		{Action: "restored", Item: "/library.db", Size: 50},
		{Action: "restored", Item: "/new.conf", Size: 100},
		{Action: "restored", Item: "/"},
	}}

	res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111", Target: targetBeside})

	if !res.Ready {
		t.Fatalf("checks = %+v", res.Checks)
	}
	if len(f.eng.steps) != 1 || f.eng.steps[0].Subtree != f.appdata || f.eng.steps[0].Target != besideOf(f.appdata) {
		t.Fatalf("the dry run must replay the restore's own call, got %+v", f.eng.steps)
	}
	// Every file of a copy is new and takes its full size, whatever lies in
	// the original.
	p := res.Plan
	if p.InPlace || p.Added != 2 || p.Changed != 0 || len(p.Shared) != 0 {
		t.Fatalf("plan = %+v", p)
	}
	if c := lineOf(t, res, lineSpace); c.Status != lineOK || c.Need != 150 {
		t.Fatalf("space = %+v, want 150 bytes needed", c)
	}
	if p.Files[0].Path != "/mnt/user/appdata/plex-restored/library.db" {
		t.Fatalf("files = %+v", p.Files)
	}
	want := BesidePlan{
		Name:    "plex-restored",
		Folders: []BesideFolder{{Original: "/mnt/user/appdata/plex", Copy: "/mnt/user/appdata/plex-restored"}},
		Container: &BesideContainer{
			Ports:  []BesidePort{{Container: "32400/tcp", Original: "32400", Copy: "32401"}},
			Shared: []string{},
		},
	}
	got := p.Beside
	if got == nil || got.Name != want.Name || !slices.Equal(got.Folders, want.Folders) ||
		got.Container == nil || !slices.Equal(got.Container.Ports, want.Container.Ports) || len(got.Container.Shared) != 0 {
		t.Fatalf("beside = %+v, container %+v", got, got.Container)
	}
	// The settings are compared with the original's, as for a restore in place.
	wantPorts := []DefinitionChange{
		{Field: "port", Change: changeAdded, Backup: "32401 -> 32400/tcp"},
		{Field: "volume", Change: changeAdded, Backup: "/mnt/user/appdata/plex-restored:/config"},
		{Field: "volume", Change: changeRemoved, Now: "/mnt/user/appdata/plex:/config"},
	}
	for _, w := range wantPorts {
		if !slices.Contains(p.Definition, w) {
			t.Errorf("definition lacks %+v: %+v", w, p.Definition)
		}
	}
	if _, err := os.Stat(besideOf(f.appdata)); !os.IsNotExist(err) {
		t.Fatalf("a check created the copy's folder: %v", err)
	}
}

func TestCheckRestoreBesideRefusesAFolderThatIsThere(t *testing.T) {
	f, _ := newBesideCheckFixture(t)
	if err := os.Mkdir(besideOf(f.appdata), 0o750); err != nil {
		t.Fatal(err)
	}

	res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111", Target: targetBeside})

	c := lineOf(t, res, lineSpace)
	if res.Ready || c.Status != lineFail || c.Reason != reasonRefused || !strings.Contains(c.Detail, "/mnt/user/appdata/plex-restored already exists") {
		t.Fatalf("ready=%v space=%+v", res.Ready, c)
	}
	if res.Plan != nil || len(f.eng.steps) != 0 {
		t.Fatal("a refused restore has no plan")
	}
}

func TestCheckRestoreBesideRefusesATakenContainerName(t *testing.T) {
	f, docker := newBesideCheckFixture(t)
	docker.taken = map[string]bool{"plex-restored": true}

	res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111", Target: targetBeside})

	c := lineOf(t, res, lineSpace)
	if res.Ready || c.Reason != reasonRefused || !strings.Contains(c.Detail, "a container named plex-restored already exists") {
		t.Fatalf("ready=%v space=%+v", res.Ready, c)
	}
}

func TestCheckRestoreBesideFlagsANetworkTheCopyWouldNeed(t *testing.T) {
	f, _ := newBesideCheckFixture(t)
	backedUpOnGoneNetwork(t, f)

	assertNetworkGone(t, f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111", Target: targetBeside}))

	f.docker.networks = append(f.docker.networks, "ix-plex_default")
	if res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111", Target: targetBeside}); !res.Ready {
		t.Fatalf("checks = %+v, want ready once the network exists", res.Checks)
	}
}

func TestCheckRestoreBesideBlocksWhenTheCopyDoesNotFit(t *testing.T) {
	f, _ := newBesideCheckFixture(t)
	f.free = 120
	f.eng.preview = map[string][]restic.PreviewItem{besideOf(f.appdata): {{Action: "restored", Item: "/big", Size: 150}}}

	res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111", Target: targetBeside})

	c := lineOf(t, res, lineSpace)
	if res.Ready || c.Status != lineFail || c.Reason != reasonShort || c.Need != 150 || c.Free != 120 {
		t.Fatalf("ready=%v space=%+v", res.Ready, c)
	}
}

func TestCheckRestoreBesideRefusesAnUnmountedShare(t *testing.T) {
	f, _ := newBesideCheckFixture(t)
	f.svc.SetPlatform(platform.Unraid{})

	res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111", Target: targetBeside})

	c := lineOf(t, res, lineSpace)
	if res.Ready || c.Reason != reasonRefused || !strings.Contains(c.Detail, "not on a mounted pool or share") {
		t.Fatalf("ready=%v space=%+v", res.Ready, c)
	}
}

func TestCheckRestoreBesideRefusesAFolderThatIsAMountOfItsOwn(t *testing.T) {
	f, _ := newBesideCheckFixture(t)
	f.svc.diskStat = func(p string) (diskStatResult, error) {
		if p == f.appdata {
			return diskStatResult{Free: f.free, Volume: "dev:2"}, nil
		}
		return diskStatResult{Free: f.free, Volume: "dev:1"}, nil
	}

	res := f.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: "aaaa1111", Target: targetBeside})

	c := lineOf(t, res, lineSpace)
	if res.Ready || c.Reason != reasonRefused || !strings.Contains(c.Detail, "a mount of its own") {
		t.Fatalf("ready=%v space=%+v", res.Ready, c)
	}
}

func TestCheckRestoreRejectsATargetTheKindDoesNotHave(t *testing.T) {
	f := newCheckFixture(t)
	for _, req := range []RestoreCheckRequest{
		{Kind: checkContainer, Name: "plex", Target: "sideways"},
		{Kind: checkContainerTo, Name: "plex", Target: targetBeside},
		{Kind: checkFlash, Target: targetBeside},
	} {
		if _, err := f.svc.CheckRestore(context.Background(), req); err == nil {
			t.Errorf("%s with target %q must be an error", req.Kind, req.Target)
		}
	}
}

func TestStartRestoreBesideIsRefusedBeforeAnythingStarts(t *testing.T) {
	f, docker := newBesideCheckFixture(t)
	docker.taken = map[string]bool{"plex-restored": true}

	started, err := f.svc.StartRestore(context.Background(), "plex", "aaaa1111", "local", RestoreOptions{Beside: true})

	if started || !isDestinationRefusal(err) {
		t.Fatalf("started=%v err=%v, want a refusal", started, err)
	}
	if f.svc.BackupInProgress() {
		t.Fatal("a refused restore must release the single-flight guard")
	}
}

var _ dockercli.Docker = (*besideCheckDocker)(nil)
