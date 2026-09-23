package api_test

import (
	"context"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A container's dumps carry its name as surely as its files backups do, so a
// rename moves both. These tests pin that the identity answering for a
// container covers its dbdump:<old name> snapshots under the same link-time
// rule, and that a stranger's dumps under a name still block a takeover.

// dumpAliasService is entry A under "radarr", renamed from "radarr-movies" at
// linkTime, with snaps in its containers repository.
func dumpAliasService(t *testing.T, snaps []restic.Snapshot) (*api.Service, *fakeResticEngine) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	establishLocalRepo(t, dir, s.ContainersPath)
	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr", AppdataPaths: []string{"/x"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr-movies", tg.ID, unixOf(t, linkTime)); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snaps: snaps}
	return api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng), eng
}

func TestDumpsOfARenamedContainerAreListedUnderItsNewName(t *testing.T) {
	svc, _ := dumpAliasService(t, []restic.Snapshot{
		{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"dbdump:radarr-movies", "p1", "dbengine:postgres"}},
		{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"dbdump:radarr-movies", "p1", "dbengine:postgres"}},
		{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"dbdump:radarr", "p1", "dbengine:postgres"}},
	})

	dumps, err := svc.DBDumps(context.Background(), "radarr", "local")
	if err != nil {
		t.Fatalf("DBDumps: %v", err)
	}
	var ids []string
	for _, d := range dumps {
		ids = append(ids, d.ID)
	}
	if got, want := strings.Join(ids, ","), "own1,pre1"; got != want {
		t.Fatalf("dumps = %s, want %s: the ones from before the link are the entry's, the later one belongs to whoever holds the old name today", got, want)
	}
}

func TestDeletingBackupsForgetsTheDumpsOfAFormerName(t *testing.T) {
	svc, eng := dumpAliasService(t, []restic.Snapshot{
		{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"dbdump:radarr-movies", "p1"}},
		{ID: "files1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}},
		{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"dbdump:radarr", "p1"}},
		{ID: "other", Time: "2024-07-01T00:00:00Z", Tags: []string{"dbdump:sonarr", "p1"}},
	})

	if err := svc.DeleteBackups(context.Background(), "radarr"); err != nil {
		t.Fatalf("DeleteBackups: %v", err)
	}
	if got, want := strings.Join(sortedCopy(eng.forgotten), ","), "files1,own1,pre1"; got != want {
		t.Fatalf("forgot %s, want %s: everything the entry owns under either identity", got, want)
	}
}

func TestTakeoverIsRefusedWhileTheNameHoldsAStrangersDump(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	establishLocalRepo(t, dir, s.ContainersPath)
	if _, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies", AppdataPaths: []string{"/x"}}); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "stranger", Time: "2024-07-01T00:00:00Z", Tags: []string{"dbdump:radarr", "p1"}},
	}}
	d := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{{Name: "radarr"}}}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

	err := svc.TakeOverContainer(context.Background(), "radarr-movies", "radarr")
	if err == nil || !strings.Contains(err.Error(), "already has backups") {
		t.Fatalf("takeover = %v, want a refusal: the dumps under that name are not this entry's", err)
	}
}
