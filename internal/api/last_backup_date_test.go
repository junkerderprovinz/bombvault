package api_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// liveContainer is a running container the fake Docker reports.
func liveContainer(name string) dockercli.ContainerInfo {
	return dockercli.ContainerInfo{Name: name, ID: "live-" + name}
}

// lastBackupOfRow reads a row's lastBackup, which JSON delivers as a float.
func lastBackupOfRow(t *testing.T, row map[string]any) (int64, bool) {
	t.Helper()
	ts, ok := row["lastBackup"].(float64)
	return int64(ts), ok
}

// containersRouter wires a router whose containers repository exists, so the
// snapshot listing has somewhere to read from.
func containersRouter(t *testing.T, d *fakeServiceDocker, eng *fakeResticEngine) (http.Handler, *store.Repo) {
	t.Helper()
	h, st, _, dir := newTestRouterSvcDir(t, d, eng)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	establishLocalRepo(t, dir, s.ContainersPath)
	return h, st
}

// An entry Discover rebuilt has no run to date it, so the date comes from the
// newest backup the entry owns. Nothing here is an orphan: the listing must
// happen for an installed entry too.
func TestListContainersDatesAnInstalledEntryFromItsBackups(t *testing.T) {
	d := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{liveContainer("radarr")}}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}},
		{ID: "bbbb2222", Time: "2024-08-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}},
	}}
	h, st := containersRouter(t, d, eng)
	if _, err := st.UpsertTarget(store.Target{ContainerName: "radarr"}); err != nil {
		t.Fatal(err)
	}

	_, m := doJSON(t, h, http.MethodGet, "/api/containers", "")
	row := containerRow(t, m["containers"].([]any), "radarr")
	got, ok := lastBackupOfRow(t, row)
	if !ok || got != unixOf(t, "2024-08-01T00:00:00Z") {
		t.Fatalf("lastBackup = %v, want the newest snapshot's time", row["lastBackup"])
	}
}

// After an entry is linked, backed up and unlinked again, the run that made
// the backup stays with the entry while the backup stays with the name. The
// date follows the backup: the entry shows the newest one it owns, and the
// name that kept the newer backup shows that one.
func TestListContainersDatesEachSideOfAnUnlinkFromTheBackupsItOwns(t *testing.T) {
	d := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{liveContainer("radarr")}}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-old", "p1"}},
		{ID: "bbbb2222", Time: "2024-08-01T00:00:00Z", Tags: []string{"container:radarr", "p1", "formerly:radarr-old"}},
	}}
	h, st := containersRouter(t, d, eng)
	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr-old"})
	if err != nil {
		t.Fatal(err)
	}
	// The run the entry carries back from the link is younger than either
	// backup, so a date taken from it would be the wrong one twice over.
	finishedRun(t, st, tg.ID)

	_, m := doJSON(t, h, http.MethodGet, "/api/containers", "")
	rows := m["containers"].([]any)
	old := containerRow(t, rows, "radarr-old")
	if got, ok := lastBackupOfRow(t, old); !ok || got != unixOf(t, "2024-01-01T00:00:00Z") {
		t.Fatalf("radarr-old lastBackup = %v, want its own backup, not the run it carries", old["lastBackup"])
	}
	live := containerRow(t, rows, "radarr")
	if got, ok := lastBackupOfRow(t, live); !ok || got != unixOf(t, "2024-08-01T00:00:00Z") {
		t.Fatalf("radarr lastBackup = %v, want the backup made under that name", live["lastBackup"])
	}
}

// An entry whose backups were all deleted has no date, so the card agrees
// with the empty list under it.
func TestListContainersShowsNoDateWhenTheBackupsAreGone(t *testing.T) {
	d := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{liveContainer("radarr")}}
	h, st := containersRouter(t, d, &fakeResticEngine{})
	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	finishedRun(t, st, tg.ID)

	_, m := doJSON(t, h, http.MethodGet, "/api/containers", "")
	row := containerRow(t, m["containers"].([]any), "radarr")
	if _, ok := lastBackupOfRow(t, row); ok {
		t.Fatalf("lastBackup = %v, want none: the entry owns no backups", row["lastBackup"])
	}
}

// A repository that cannot be listed must not read as "never backed up", so
// the run's own date stands in.
func TestListContainersKeepsTheRunsDateWhileTheRepositoryIsUnreadable(t *testing.T) {
	d := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{liveContainer("radarr")}}
	h, st := newTestRouter(t, d, &fakeResticEngine{})
	s := mustSettings(t, st)
	s.ContainersPath = "/absolute-path-is-always-refused"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	run := finishedRun(t, st, tg.ID)

	_, m := doJSON(t, h, http.MethodGet, "/api/containers", "")
	row := containerRow(t, m["containers"].([]any), "radarr")
	if got, ok := lastBackupOfRow(t, row); !ok || got != *run.FinishedAt {
		t.Fatalf("lastBackup = %v, want the run's %d", row["lastBackup"], *run.FinishedAt)
	}
}

// The dashboard measures a backup's duration from the pair, so the start time
// comes only from the run that wrote the backup being dated.
func TestListContainersPairsAStartTimeOnlyWithTheRunThatWroteTheBackup(t *testing.T) {
	d := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{liveContainer("radarr"), liveContainer("sonarr")}}
	eng := &fakeResticEngine{}
	h, st := containersRouter(t, d, eng)
	radarr, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	sonarr, err := st.UpsertTarget(store.Target{ContainerName: "sonarr"})
	if err != nil {
		t.Fatal(err)
	}
	// radarr's backup is written while its run is open, sonarr's long before
	// the run it carries.
	id, err := st.StartRun(radarr.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	written := time.Now().UTC()
	eng.snaps = []restic.Snapshot{
		{ID: "aaaa1111", Time: written.Format(time.RFC3339Nano), Tags: []string{"container:radarr", "p1"}},
		{ID: "bbbb2222", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:sonarr", "p1"}},
	}
	if err := st.FinishRun(id, "success", "aaaa1111", 1, ""); err != nil {
		t.Fatal(err)
	}
	finishedRun(t, st, sonarr.ID)

	_, m := doJSON(t, h, http.MethodGet, "/api/containers", "")
	rows := m["containers"].([]any)
	if containerRow(t, rows, "radarr")["lastBackupStarted"] == nil {
		t.Fatal("radarr: want the start time of the run that wrote its backup")
	}
	if got := containerRow(t, rows, "sonarr")["lastBackupStarted"]; got != nil {
		t.Fatalf("sonarr lastBackupStarted = %v, want none: that run wrote a different backup", got)
	}
}

// finishedRun records a successful backup run for targetID and returns it.
func finishedRun(t *testing.T, st *store.Repo, targetID string) store.Run {
	t.Helper()
	id, err := st.StartRun(targetID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(id, "success", "snap-"+targetID, 1, ""); err != nil {
		t.Fatal(err)
	}
	run, err := st.LastSuccessfulBackup(targetID)
	if err != nil || run == nil {
		t.Fatalf("LastSuccessfulBackup: %v %v", run, err)
	}
	return *run
}

// vmTimesService wires a VM service whose repository exists and whose domain
// list is served by v.
func vmTimesService(t *testing.T, st *store.Repo, v virshcli.Virsh, eng *fakeResticEngine) *api.Service {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	s := mustSettings(t, st)
	s.VMsEnabled = true
	s.VMsPath = "backups/vms"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	establishLocalRepo(t, dir, s.VMsPath)
	return api.NewService(cfg, st, &fakeServiceDocker{}, v, eng)
}

// The VM list dates an entry the same way the container list does.
func TestListVMsDatesEachSideOfAnUnlinkFromTheBackupsItOwns(t *testing.T) {
	st := newMemStore(t)
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	finishedRun(t, st, tg.ID)
	v := &renameSuggestVirsh{vms: []virshcli.VMInfo{{Name: "win11", State: "running"}}}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}},
		{ID: "bbbb2222", Time: "2024-08-01T00:00:00Z", Tags: []string{"vm:win11", "p2", "formerly:windows-11"}},
	}}
	svc := vmTimesService(t, st, v, eng)

	views, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}
	old := vmView(t, views, "windows-11")
	if old.LastBackup == nil || *old.LastBackup != unixOf(t, "2024-01-01T00:00:00Z") {
		t.Fatalf("windows-11 LastBackup = %v, want its own backup, not the run it carries", old.LastBackup)
	}
	live := vmView(t, views, "win11")
	if live.LastBackup == nil || *live.LastBackup != unixOf(t, "2024-08-01T00:00:00Z") {
		t.Fatalf("win11 LastBackup = %v, want the backup made under that name", live.LastBackup)
	}
}

// A VM repository that cannot be listed leaves the run's date in place.
func TestListVMsKeepsTheRunsDateWhileTheRepositoryIsUnreadable(t *testing.T) {
	st := newMemStore(t)
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	run := finishedRun(t, st, tg.ID)
	v := &renameSuggestVirsh{vms: []virshcli.VMInfo{{Name: "win11", State: "running"}}}
	svc := vmTimesService(t, st, v, &fakeResticEngine{})
	s := mustSettings(t, st)
	s.VMsPath = "/absolute-path-is-always-refused"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	views, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs: %v", err)
	}
	got := vmView(t, views, "win11")
	if got.LastBackup == nil || *got.LastBackup != *run.FinishedAt {
		t.Fatalf("LastBackup = %v, want the run's %d", got.LastBackup, *run.FinishedAt)
	}
}

// A folder set is dated from its backups too, so a set Discover rebuilt from
// the repository does not read as never backed up.
func TestListFileSetViewsDatesASetFromItsBackups(t *testing.T) {
	st := newMemStore(t)
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	s := mustSettings(t, st)
	s.FilesPath = "backups/files"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	establishLocalRepo(t, dir, s.FilesPath)
	if _, err := st.CreateFileSet(store.FileSet{Name: "docs", Path: "docs"}); err != nil {
		t.Fatal(err)
	}
	gone, err := st.CreateFileSet(store.FileSet{Name: "photos", Path: "photos"})
	if err != nil {
		t.Fatal(err)
	}
	finishedRun(t, st, gone.ID)
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Time: "2024-08-01T00:00:00Z", Tags: []string{"fileset:docs", "p3"}},
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	views, err := svc.ListFileSetViews(context.Background())
	if err != nil {
		t.Fatalf("ListFileSetViews: %v", err)
	}
	byName := map[string]int64{}
	for _, v := range views {
		byName[v.Name] = v.LastBackup
	}
	if byName["docs"] != unixOf(t, "2024-08-01T00:00:00Z") {
		t.Fatalf("docs LastBackup = %d, want its backup's time", byName["docs"])
	}
	if byName["photos"] != 0 {
		t.Fatalf("photos LastBackup = %d, want none: its backups are gone", byName["photos"])
	}
}

// A VM's backups keep the old vm:<name> tag after a rename, so the times fold
// the old name onto the current one, bounded by the link.
func TestLatestVMBackupTimesFoldsAlias(t *testing.T) {
	st := newMemStore(t)
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("vm", "windows-11", tg.ID, unixOf(t, linkTime)); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Time: "2024-03-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}},
		{ID: "bbbb2222", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:win11", "p2"}},
		{ID: "cccc3333", Time: "2024-09-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}},
	}}
	svc := vmTimesService(t, st, &renameSuggestVirsh{}, eng)

	times, err := svc.LatestVMBackupTimes(context.Background())
	if err != nil {
		t.Fatalf("LatestVMBackupTimes: %v", err)
	}
	if got := times["win11"]; got != unixOf(t, "2024-03-01T00:00:00Z") {
		t.Fatalf("win11 = %d, want the newest backup from before the link", got)
	}
	if got := times["windows-11"]; got != unixOf(t, "2024-09-01T00:00:00Z") {
		t.Fatalf("windows-11 = %d, want the backup made under that name after the link", got)
	}
}
