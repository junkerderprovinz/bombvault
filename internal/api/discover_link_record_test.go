package api_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// These tests pin the link records in the definition mirrors on the backup
// storage: takeover, unlink and every backup keep them current, and after a
// /config loss Discover rebuilds a link only from them, at the time they give.

// containerLinks is a container install backing up into a local domain
// repository, with one live container whose name the test changes.
type containerLinks struct {
	cfg    config.Config
	db     *sql.DB
	st     *store.Repo
	svc    *api.Service
	docker *fakeServiceDocker
	eng    *fakeResticEngine
	root   string
	repo   string
}

func newContainerLinks(t *testing.T) *containerLinks {
	t.Helper()
	root := filepath.ToSlash(t.TempDir())
	f := &containerLinks{root: root, cfg: config.Config{
		AppKey: strings.Repeat("a", 64), DataDir: t.TempDir(), HostMountRoot: root, FlashTemplatesDir: t.TempDir(),
	}}
	f.db, f.st = openStore(t)
	useContainerRepo(t, f.st)
	f.repo = establishLocalRepo(t, root, "backups/containers")
	f.docker = &fakeServiceDocker{inspect: model.Inspect{Image: "radarr:latest"}}
	f.eng = &fakeResticEngine{}
	f.svc = api.NewService(f.cfg, f.st, f.docker, fakeVirsh{}, f.eng)
	return f
}

func useContainerRepo(t *testing.T, st *store.Repo) {
	t.Helper()
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
}

// live makes name the one container Docker lists and inspects, with its
// appdata folder named after it as the automatic detection expects.
func (f *containerLinks) live(t *testing.T, name string) {
	t.Helper()
	appdata := f.root + "/appdata/" + name
	if err := os.MkdirAll(appdata, 0o750); err != nil {
		t.Fatal(err)
	}
	f.docker.listOut = []dockercli.ContainerInfo{{Name: name}}
	f.docker.inspect.Name = "/" + name
	f.docker.inspect.Mounts = []model.Mount{{Type: "bind", Source: appdata, Destination: "/config"}}
}

// backUp backs up the live container called name and keeps the snapshot the
// backup wrote as id, taken at at.
func (f *containerLinks) backUp(t *testing.T, name, id string, at time.Time) {
	t.Helper()
	f.live(t, name)
	if _, err := f.svc.Backup(context.Background(), name); err != nil {
		t.Fatalf("Backup(%s): %v", name, err)
	}
	f.eng.snaps = append(f.eng.snaps, restic.Snapshot{ID: id, Time: at.UTC().Format(time.RFC3339), Tags: append([]string(nil), f.eng.lastTags...)})
}

func (f *containerLinks) takeOver(t *testing.T, oldName, newName string) store.Alias {
	t.Helper()
	f.live(t, newName)
	if err := f.svc.TakeOverContainer(context.Background(), oldName, newName); err != nil {
		t.Fatalf("TakeOverContainer(%s, %s): %v", oldName, newName, err)
	}
	a, err := f.st.AliasByOldName("container", oldName)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func (f *containerLinks) forget(id string) {
	kept := f.eng.snaps[:0]
	for _, s := range f.eng.snaps {
		if s.ID != id {
			kept = append(kept, s)
		}
	}
	f.eng.snaps = kept
}

// afterConfigLoss is a fresh install on the same backup storage, rebuilt by
// Discover.
func (f *containerLinks) afterConfigLoss(t *testing.T) (*api.Service, *store.Repo) {
	t.Helper()
	st := newMemStore(t)
	useContainerRepo(t, st)
	svc := api.NewService(f.cfg, st, &fakeServiceDocker{}, fakeVirsh{}, f.eng)
	if _, _, err := svc.Discover(context.Background(), false); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	return svc, st
}

func containerSnapshotIDs(t *testing.T, svc *api.Service, name string) string {
	t.Helper()
	snaps, err := svc.Snapshots(context.Background(), name, "")
	if err != nil {
		t.Fatalf("Snapshots(%s): %v", name, err)
	}
	return joined(snapshotIDs(snaps))
}

// linkedContainer checks that old is linked to owner at linkedAt.
func linkedContainer(t *testing.T, st *store.Repo, old, owner string, linkedAt int64) {
	t.Helper()
	tg, err := st.GetTargetByContainer(owner)
	if err != nil {
		t.Fatalf("%s must be rebuilt: %v", owner, err)
	}
	a, err := st.AliasByOldName("container", old)
	if err != nil || a.TargetID != tg.ID {
		t.Fatalf("alias %s = %+v, %v; want it linked to %s", old, a, err, owner)
	}
	if a.LinkedAt != linkedAt {
		t.Fatalf("alias %s linked at %d, want the recorded %d", old, a.LinkedAt, linkedAt)
	}
}

func wantContainerAlias(t *testing.T, st *store.Repo, old, owner string, linkedAt int64) {
	t.Helper()
	linkedContainer(t, st, old, owner, linkedAt)
	if _, err := st.GetTargetByContainer(old); err == nil {
		t.Fatalf("%s is a former name and must not be rebuilt as its own entry", old)
	}
}

// wantContainerBesideLink checks the conflict state: old is linked to owner at
// linkedAt and is an entry of its own as well.
func wantContainerBesideLink(t *testing.T, st *store.Repo, old, owner string, linkedAt int64) {
	t.Helper()
	linkedContainer(t, st, old, owner, linkedAt)
	if _, err := st.GetTargetByContainer(old); err != nil {
		t.Fatalf("%s holds another container's backups and must be rebuilt as its own entry too: %v", old, err)
	}
}

func wantNoContainerAlias(t *testing.T, st *store.Repo, old string) {
	t.Helper()
	if a, err := st.AliasByOldName("container", old); err == nil {
		t.Fatalf("alias %+v, want %s unlinked", a, old)
	}
	if _, err := st.GetTargetByContainer(old); err != nil {
		t.Fatalf("%s must be rebuilt as its own entry: %v", old, err)
	}
}

// captureLog collects what the standard logger writes while run runs.
func captureLog(run func()) string {
	var buf strings.Builder
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	run()
	return buf.String()
}

var linkBase = time.Now().UTC().Truncate(time.Second)

func TestDiscoverRelinksATakenOverContainerAtItsRecordedTime(t *testing.T) {
	f := newContainerLinks(t)
	f.backUp(t, "radarr-movies", "pre1", linkBase.Add(-30*24*time.Hour))
	link := f.takeOver(t, "radarr-movies", "radarr")
	f.backUp(t, "radarr", "post1", linkBase.Add(time.Hour))

	svc, st := f.afterConfigLoss(t)
	wantContainerAlias(t, st, "radarr-movies", "radarr", link.LinkedAt)
	if got := containerSnapshotIDs(t, svc, "radarr"); got != "post1,pre1" {
		t.Fatalf("radarr snapshots = %s, want both names' backups", got)
	}
}

func TestDiscoverKeepsAnUnlinkedContainerApart(t *testing.T) {
	f := newContainerLinks(t)
	f.backUp(t, "radarr-movies", "pre1", linkBase.Add(-30*24*time.Hour))
	f.takeOver(t, "radarr-movies", "radarr")
	f.backUp(t, "radarr", "post1", linkBase.Add(time.Hour))
	if err := f.svc.UnlinkContainerAlias(context.Background(), "radarr-movies"); err != nil {
		t.Fatalf("UnlinkContainerAlias: %v", err)
	}

	svc, st := f.afterConfigLoss(t)
	wantNoContainerAlias(t, st, "radarr-movies")
	if got := containerSnapshotIDs(t, svc, "radarr-movies"); got != "pre1" {
		t.Fatalf("radarr-movies snapshots = %s, want its own pre1", got)
	}
}

// Retention has pruned the claimant's first backups after the link, and a
// different container has since been backed up under the old name. The
// record still dates the link, so the stranger's backup stays out of it and
// stays with the old name's own entry.
func TestDiscoverDatesAContainerLinkByItsRecordAfterRetention(t *testing.T) {
	f := newContainerLinks(t)
	f.backUp(t, "radarr-movies", "pre1", linkBase.Add(-30*24*time.Hour))
	link := f.takeOver(t, "radarr-movies", "radarr")
	f.backUp(t, "radarr", "post1", linkBase.Add(time.Hour))
	f.backUp(t, "radarr", "post2", linkBase.Add(20*24*time.Hour))
	f.eng.snaps = append(f.eng.snaps, restic.Snapshot{ID: "stranger1", Time: linkBase.Add(10 * 24 * time.Hour).Format(time.RFC3339), Tags: []string{"container:radarr-movies", "p1"}})
	f.forget("post1")

	svc, st := f.afterConfigLoss(t)
	wantContainerBesideLink(t, st, "radarr-movies", "radarr", link.LinkedAt)
	if got := containerSnapshotIDs(t, svc, "radarr"); got != "post2,pre1" {
		t.Fatalf("radarr snapshots = %s, want post2 and pre1 without the stranger's", got)
	}
	if got := containerSnapshotIDs(t, svc, "radarr-movies"); got != "stranger1" {
		t.Fatalf("radarr-movies snapshots = %s, want the stranger's alone", got)
	}
}

// writeDef writes def, encrypted, as the definition file of name in the
// domain repository.
func (f *containerLinks) writeDef(t *testing.T, name string, def []byte) {
	t.Helper()
	enc, err := secret.Encrypt(f.cfg.AppKey, def)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(f.repo, "def"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.repo, "def", name+".def"), enc, 0o600); err != nil {
		t.Fatal(err)
	}
}

// restorer is a service on st that restores from a remote repository under a
// Linux mount root, as TestRestoreUsesStoredDefinitionWhenContainerDeleted
// does, because the path checks take Linux paths.
func (f *containerLinks) restorer(t *testing.T, st *store.Repo) (*api.Service, *fakeServiceDocker) {
	t.Helper()
	s := mustSettings(t, st)
	s.ContainersPath = "rest:http://127.0.0.1/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	cfg := f.cfg
	cfg.HostMountRoot = "/host/user"
	d := &fakeServiceDocker{inspectErr: os.ErrNotExist}
	return api.NewService(cfg, st, d, fakeVirsh{}, f.eng), d
}

// backUpAnother stands for a backup of a different container, running image,
// under name: the definition file it writes and its snapshot id, taken at at.
// The appdata sits under the Linux mount root restorer uses.
func (f *containerLinks) backUpAnother(t *testing.T, name, image, id string, at time.Time) {
	t.Helper()
	appdata := "/host/user/appdata/" + name
	def, err := marshalDefinition(model.Inspect{Name: "/" + name, Config: model.Config{Image: image}}, "<xml/>", appdata)
	if err != nil {
		t.Fatal(err)
	}
	f.writeDef(t, name, def)
	f.eng.snaps = append(f.eng.snaps, restic.Snapshot{ID: id, Time: at.UTC().Format(time.RFC3339), Tags: []string{"container:" + name, "p1"}, Paths: []string{appdata}})
}

// A different container took the old name after the link and was backed up
// under it. The rebuild links the name at its recorded time and brings that
// container back as an entry of its own, whose backup restores it.
func TestDiscoverRebuildsALaterContainerUnderARecordedFormerNameBesideTheLink(t *testing.T) {
	f := newContainerLinks(t)
	f.backUp(t, "radarr-movies", "pre1", linkBase.Add(-30*24*time.Hour))
	link := f.takeOver(t, "radarr-movies", "radarr")
	f.backUp(t, "radarr", "post1", linkBase.Add(time.Hour))
	f.backUpAnother(t, "radarr-movies", "other/radarr:latest", "0badc0de", linkBase.Add(24*time.Hour))

	svc, st := f.afterConfigLoss(t)
	wantContainerBesideLink(t, st, "radarr-movies", "radarr", link.LinkedAt)
	if got := containerSnapshotIDs(t, svc, "radarr"); got != "post1,pre1" {
		t.Fatalf("radarr snapshots = %s, want its own and the old name's from before the link", got)
	}
	restorer, d := f.restorer(t, st)
	if err := restorer.Restore(context.Background(), "radarr-movies", "0badc0de", true, "", false); err != nil {
		t.Fatalf("Restore(radarr-movies, 0badc0de): %v", err)
	}
	if d.createdIn.Config.Image != "other/radarr:latest" {
		t.Fatalf("recreated image = %q, want the other container's", d.createdIn.Config.Image)
	}
}

// A snapshot under the old name whose time does not parse cannot be placed
// before the link, so it counts as another container's.
func TestDiscoverRebuildsTheOldNameBesideTheLinkForASnapshotWithoutATime(t *testing.T) {
	f := newContainerLinks(t)
	f.backUp(t, "radarr-movies", "pre1", linkBase.Add(-30*24*time.Hour))
	link := f.takeOver(t, "radarr-movies", "radarr")
	f.backUp(t, "radarr", "post1", linkBase.Add(time.Hour))
	f.eng.snaps = append(f.eng.snaps, restic.Snapshot{ID: "undated1", Time: "not a time", Tags: []string{"container:radarr-movies", "p1"}})

	_, st := f.afterConfigLoss(t)
	wantContainerBesideLink(t, st, "radarr-movies", "radarr", link.LinkedAt)
}

// A later container under the old name cannot be ruled out while its
// repository does not list, so the name is rebuilt beside the link.
func TestDiscoverRebuildsTheOldNameBesideTheLinkWhenItsBackupsCannotBeListed(t *testing.T) {
	f := newContainerLinks(t)
	f.backUp(t, "radarr-movies", "pre1", linkBase.Add(-30*24*time.Hour))
	link := f.takeOver(t, "radarr-movies", "radarr")
	f.backUp(t, "radarr", "post1", linkBase.Add(time.Hour))
	listings := 0
	f.eng.onSnapshots = func(int) {
		// The first listing finds the names; the second is the fold's.
		if listings++; listings == 2 {
			f.eng.snapshotsErr = errors.New("repository unreadable")
		}
	}

	_, st := f.afterConfigLoss(t)
	wantContainerBesideLink(t, st, "radarr-movies", "radarr", link.LinkedAt)
}

func TestDiscoverRebuildsTheOldNameWhenTheContainerClaimantsDefinitionDoesNotDecrypt(t *testing.T) {
	f := newContainerLinks(t)
	f.backUp(t, "radarr-movies", "pre1", linkBase.Add(-30*24*time.Hour))
	f.takeOver(t, "radarr-movies", "radarr")
	f.backUp(t, "radarr", "post1", linkBase.Add(time.Hour))
	if err := os.WriteFile(filepath.Join(f.repo, "def", "radarr.def"), []byte("not encrypted with this key"), 0o600); err != nil {
		t.Fatal(err)
	}

	var st *store.Repo
	out := captureLog(func() { _, st = f.afterConfigLoss(t) })
	wantNoContainerAlias(t, st, "radarr-movies")
	if want := `"radarr-movies" is a former name on the backups of ["radarr"], but no readable stored definition records the link`; !strings.Contains(out, want) {
		t.Fatalf("log = %s\nwant a line containing %s", out, want)
	}
}

// Renamed back, the entry keeps its own name and records the name it left,
// which records nothing any more. The backup under the name it left predates
// the clock the store stamps the rename back with.
func TestDiscoverRebuildsAContainerRenamedBackUnderItsOwnName(t *testing.T) {
	f := newContainerLinks(t)
	f.backUp(t, "radarr-movies", "pre1", linkBase.Add(-30*24*time.Hour))
	f.takeOver(t, "radarr-movies", "radarr")
	f.backUp(t, "radarr", "post1", linkBase.Add(-time.Hour))
	back := f.takeOver(t, "radarr", "radarr-movies")
	f.backUp(t, "radarr-movies", "back1", linkBase.Add(2*time.Hour))

	svc, st := f.afterConfigLoss(t)
	wantContainerAlias(t, st, "radarr", "radarr-movies", back.LinkedAt)
	if got := containerSnapshotIDs(t, svc, "radarr-movies"); got != "back1,post1,pre1" {
		t.Fatalf("radarr-movies snapshots = %s, want all three", got)
	}
}

// Taken over again after an unlink and not backed up since, the entry's link
// lives only in the record the takeover wrote.
func TestDiscoverRelinksAContainerTakenOverAgainBeforeItsNextBackup(t *testing.T) {
	f := newContainerLinks(t)
	f.backUp(t, "radarr-movies", "pre1", linkBase.Add(-30*24*time.Hour))
	f.takeOver(t, "radarr-movies", "radarr")
	f.backUp(t, "radarr", "post1", linkBase.Add(-time.Hour))
	if err := f.svc.UnlinkContainerAlias(context.Background(), "radarr-movies"); err != nil {
		t.Fatalf("UnlinkContainerAlias: %v", err)
	}
	again := f.takeOver(t, "radarr-movies", "radarr")

	_, st := f.afterConfigLoss(t)
	wantContainerAlias(t, st, "radarr-movies", "radarr", again.LinkedAt)
}

// Unlinking the middle name of A -> B -> C puts the entry back on B, which
// records A, while C records nothing.
func TestDiscoverRebuildsAContainerChainAfterUnlinkingItsMiddleName(t *testing.T) {
	f := newContainerLinks(t)
	f.backUp(t, "radarr-a", "a1", linkBase.Add(-30*24*time.Hour))
	first := f.takeOver(t, "radarr-a", "radarr-b")
	f.backUp(t, "radarr-b", "b1", linkBase.Add(-time.Hour))
	f.takeOver(t, "radarr-b", "radarr-c")
	f.backUp(t, "radarr-c", "c1", linkBase.Add(time.Hour))
	if err := f.svc.UnlinkContainerAlias(context.Background(), "radarr-b"); err != nil {
		t.Fatalf("UnlinkContainerAlias: %v", err)
	}

	_, st := f.afterConfigLoss(t)
	wantContainerAlias(t, st, "radarr-a", "radarr-b", first.LinkedAt)
	c, err := st.GetTargetByContainer("radarr-c")
	if err != nil {
		t.Fatalf("radarr-c must be rebuilt as its own entry: %v", err)
	}
	if names, _ := st.AliasNames("container", c.ID); len(names) != 0 {
		t.Fatalf("radarr-c aliases = %v, want none", names)
	}
}

// A definition file from before link records has no aliases field, and the
// entry it rebuilds still restores.
func TestAContainerDefinitionWithoutLinkRecordsStillRestores(t *testing.T) {
	f := newContainerLinks(t)
	const appdata = "/host/user/appdata/plex"
	def, err := marshalDefinition(model.Inspect{Name: "/plex", Config: model.Config{Image: "plex:latest"}}, "<xml/>", appdata)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(def), "aliases") {
		t.Fatalf("definition %s must be in the format from before link records", def)
	}
	f.writeDef(t, "plex", def)
	f.eng.snaps = []restic.Snapshot{{ID: "deadbeef", Tags: []string{"container:plex", "p1"}, Paths: []string{appdata}}}

	_, st := f.afterConfigLoss(t)
	if _, err := st.GetTargetByContainer("plex"); err != nil {
		t.Fatalf("plex must be rebuilt: %v", err)
	}
	svc, d := f.restorer(t, st)
	if err := svc.Restore(context.Background(), "plex", "deadbeef", true, "", false); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if d.createdIn.Config.Image != "plex:latest" {
		t.Fatalf("recreated image = %q, want plex:latest from the stored definition", d.createdIn.Config.Image)
	}
}

// An unlink on a rebuilt install starts from a definition that came back with
// its link records, and still leaves none behind.
func TestDiscoverKeepsAContainerUnlinkedAfterARebuildApart(t *testing.T) {
	f := newContainerLinks(t)
	f.backUp(t, "radarr-movies", "pre1", linkBase.Add(-30*24*time.Hour))
	f.takeOver(t, "radarr-movies", "radarr")
	f.backUp(t, "radarr", "post1", linkBase.Add(time.Hour))
	rebuilt, _ := f.afterConfigLoss(t)
	if err := rebuilt.UnlinkContainerAlias(context.Background(), "radarr-movies"); err != nil {
		t.Fatalf("UnlinkContainerAlias: %v", err)
	}

	_, st := f.afterConfigLoss(t)
	wantNoContainerAlias(t, st, "radarr-movies")
}

// Two definitions that record each other, as a stale record could leave them,
// rebuild one entry with the other as its alias, the same way every time.
func TestDiscoverRebuildsOneOfTwoNamesThatRecordEachOther(t *testing.T) {
	f := newContainerLinks(t)
	f.eng.snaps = []restic.Snapshot{
		{ID: "aaaa1111", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}},
		{ID: "bbbb2222", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:radarr", "p1", "formerly:radarr-movies"}},
	}
	writeStoredDef(t, f.svc, f.repo, "radarr", "radarr-movies")
	writeStoredDef(t, f.svc, f.repo, "radarr-movies", "radarr")

	_, st := f.afterConfigLoss(t)
	wantContainerAlias(t, st, "radarr-movies", "radarr", unixOf(t, linkTime))
}

// blockMirror puts a plain file where the definition mirror directory dir
// goes, so every write to it fails.
func blockMirror(t *testing.T, dir string) {
	t.Helper()
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("in the way"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// breakAliases gives entry id an alias no read can scan.
func breakAliases(t *testing.T, db *sql.DB, domain, id string) {
	t.Helper()
	if _, err := db.Exec("INSERT INTO target_aliases (id, domain, old_name, target_id, linked_at) VALUES ('bad', ?, 'unreadable', ?, 'garbage')", domain, id); err != nil {
		t.Fatal(err)
	}
}

// An unlink that left its record behind would be undone by the next rebuild,
// so it stops while the record cannot be removed.
func TestUnlinkContainerAliasRefusesWhileItsLinkRecordCannotBeRemoved(t *testing.T) {
	f := newContainerLinks(t)
	f.backUp(t, "radarr-movies", "pre1", linkBase.Add(-30*24*time.Hour))
	f.takeOver(t, "radarr-movies", "radarr")
	f.backUp(t, "radarr", "post1", linkBase.Add(time.Hour))
	blockMirror(t, filepath.Join(f.repo, "def"))

	err := f.svc.UnlinkContainerAlias(context.Background(), "radarr-movies")
	if err == nil || !strings.Contains(err.Error(), "could not be removed") {
		t.Fatalf("UnlinkContainerAlias = %v, want a refusal because the record stays", err)
	}
	if _, err := f.st.AliasByOldName("container", "radarr-movies"); err != nil {
		t.Fatalf("the refused unlink must keep the alias: %v", err)
	}
	if _, err := f.st.GetTargetByContainer("radarr"); err != nil {
		t.Fatalf("the entry must stay on radarr: %v", err)
	}
}

func TestTakeOverContainerRefusesWhileTheLeftNamesLinkRecordsCannotBeRemoved(t *testing.T) {
	f := newContainerLinks(t)
	f.backUp(t, "radarr-movies", "pre1", linkBase.Add(-30*24*time.Hour))
	blockMirror(t, filepath.Join(f.repo, "def"))
	f.live(t, "radarr")

	err := f.svc.TakeOverContainer(context.Background(), "radarr-movies", "radarr")
	if err == nil || !strings.Contains(err.Error(), "could not be removed") {
		t.Fatalf("TakeOverContainer = %v, want a refusal because the old name's records stay", err)
	}
	if _, err := f.st.AliasByOldName("container", "radarr-movies"); err == nil {
		t.Fatal("the refused takeover must not link radarr-movies")
	}
	if _, err := f.st.GetTargetByContainer("radarr-movies"); err != nil {
		t.Fatalf("the entry must stay on radarr-movies: %v", err)
	}
}

// Without the aliases a backup would write the mirror without its link
// records, so it leaves the mirror as the last write left it.
func TestAContainerBackupKeepsTheLinkRecordWhenItCannotReadTheAliases(t *testing.T) {
	f := newContainerLinks(t)
	f.backUp(t, "radarr-movies", "pre1", linkBase.Add(-30*24*time.Hour))
	link := f.takeOver(t, "radarr-movies", "radarr")
	f.backUp(t, "radarr", "post1", linkBase.Add(time.Hour))
	tg, err := f.st.GetTargetByContainer("radarr")
	if err != nil {
		t.Fatal(err)
	}
	breakAliases(t, f.db, "container", tg.ID)
	f.backUp(t, "radarr", "post2", linkBase.Add(2*time.Hour))

	_, st := f.afterConfigLoss(t)
	wantContainerAlias(t, st, "radarr-movies", "radarr", link.LinkedAt)
}

// backUp backs up the VM called name and keeps the snapshot the backup wrote
// as id, taken at at.
func (f *vmTakeover) backUp(t *testing.T, name, id string, at time.Time) {
	t.Helper()
	if _, err := f.svc.BackupVM(context.Background(), name); err != nil {
		t.Fatalf("BackupVM(%s): %v", name, err)
	}
	f.eng.snapsByRepo[f.repo] = append(f.eng.snapsByRepo[f.repo], restic.Snapshot{ID: id, Time: at.UTC().Format(time.RFC3339), Tags: append([]string(nil), f.eng.lastTags...)})
}

// takeOver renames the one defined VM to newName and takes the entry over.
func (f *vmTakeover) takeOver(t *testing.T, oldName, newName string) store.Alias {
	t.Helper()
	f.virsh.vms = []virshcli.VMInfo{{Name: newName, State: "shut off"}}
	if _, ok := f.virsh.xmlByName[newName]; !ok {
		f.virsh.xmlByName[newName] = unraidVMXML(newName)
	}
	if err := f.svc.TakeOverVM(context.Background(), oldName, newName); err != nil {
		t.Fatalf("TakeOverVM(%s, %s): %v", oldName, newName, err)
	}
	a, err := f.st.AliasByOldName("vm", oldName)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func (f *vmTakeover) forget(id string) {
	snaps := f.eng.snapsByRepo[f.repo]
	kept := snaps[:0]
	for _, s := range snaps {
		if s.ID != id {
			kept = append(kept, s)
		}
	}
	f.eng.snapsByRepo[f.repo] = kept
}

func (f *vmTakeover) afterConfigLoss(t *testing.T) (*api.Service, *store.Repo) {
	t.Helper()
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.VMsPath = "backups/vms"
	s.VMsEnabled = true
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(f.cfg, st, &fakeServiceDocker{}, fakeVirsh{}, f.eng)
	if _, _, err := svc.DiscoverVMs(context.Background(), false); err != nil {
		t.Fatalf("DiscoverVMs: %v", err)
	}
	return svc, st
}

func vmSnapshotIDs(t *testing.T, svc *api.Service, name string) string {
	t.Helper()
	snaps, err := svc.SnapshotsVM(context.Background(), name, "")
	if err != nil {
		t.Fatalf("SnapshotsVM(%s): %v", name, err)
	}
	return joined(snapshotIDs(snaps))
}

// linkedVM checks that old is linked to owner at linkedAt and returns the
// alias.
func linkedVM(t *testing.T, st *store.Repo, old, owner string, linkedAt int64) store.Alias {
	t.Helper()
	tg, err := st.GetVMTargetByName(owner)
	if err != nil {
		t.Fatalf("%s must be rebuilt: %v", owner, err)
	}
	a, err := st.AliasByOldName("vm", old)
	if err != nil || a.TargetID != tg.ID {
		t.Fatalf("alias %s = %+v, %v; want it linked to %s", old, a, err, owner)
	}
	if a.LinkedAt != linkedAt {
		t.Fatalf("alias %s linked at %d, want the recorded %d", old, a.LinkedAt, linkedAt)
	}
	return a
}

func wantVMAlias(t *testing.T, st *store.Repo, old, owner string, linkedAt int64) {
	t.Helper()
	linkedVM(t, st, old, owner, linkedAt)
	if _, err := st.GetVMTargetByName(old); err == nil {
		t.Fatalf("%s is a former name and must not be rebuilt as its own entry", old)
	}
}

// wantVMBesideLink is wantContainerBesideLink for VMs, and returns the alias.
func wantVMBesideLink(t *testing.T, st *store.Repo, old, owner string, linkedAt int64) store.Alias {
	t.Helper()
	a := linkedVM(t, st, old, owner, linkedAt)
	if _, err := st.GetVMTargetByName(old); err != nil {
		t.Fatalf("%s holds another VM's backups and must be rebuilt as its own entry too: %v", old, err)
	}
	return a
}

// wantDefinition checks that got is the VM definition want, field by field.
func wantDefinition(t *testing.T, got, want string) {
	t.Helper()
	var g, w vmTakeoverDef
	if err := json.Unmarshal([]byte(got), &g); err != nil {
		t.Fatalf("definition %q: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Fatalf("definition = %+v, want %+v", g, w)
	}
}

// backUpAnother defines a different VM, with another libvirt UUID, under name
// and backs it up as id, taken at at.
func (f *vmTakeover) backUpAnother(t *testing.T, name, id string, at time.Time) {
	t.Helper()
	f.virsh.vms = []virshcli.VMInfo{{Name: name, State: "shut off"}}
	f.virsh.xmlByName[name] = strings.Replace(unraidVMXML(name), testVMUUID, otherVMUUID, 1)
	f.backUp(t, name, id, at)
}

func wantNoVMAlias(t *testing.T, st *store.Repo, old string) {
	t.Helper()
	if a, err := st.AliasByOldName("vm", old); err == nil {
		t.Fatalf("alias %+v, want %s unlinked", a, old)
	}
	if _, err := st.GetVMTargetByName(old); err != nil {
		t.Fatalf("%s must be rebuilt as its own entry: %v", old, err)
	}
}

func TestDiscoverVMsRelinksATakenOverVMAtItsRecordedTime(t *testing.T) {
	f := newVMTakeover(t, vmPre)
	link := f.takeOver(t, "windows-11", "win11")
	f.backUp(t, "win11", "post1", linkBase.Add(time.Hour))

	svc, st := f.afterConfigLoss(t)
	wantVMAlias(t, st, "windows-11", "win11", link.LinkedAt)
	if got := vmSnapshotIDs(t, svc, "win11"); got != "post1,pre1" {
		t.Fatalf("win11 snapshots = %s, want both names' backups", got)
	}
	a, err := st.AliasByOldName("vm", "windows-11")
	if err != nil {
		t.Fatal(err)
	}
	wantDefinition(t, a.PrevDefinition, f.oldDef)
}

// A different VM took the old name after the link and was backed up under it,
// which rewrote the old name's definition file. The rebuild links the name
// with the definition the entry itself had there, brings the other VM back as
// an entry of its own, and an unlink stays refused.
func TestDiscoverVMsRebuildsALaterVMUnderARecordedFormerNameBesideTheLink(t *testing.T) {
	f := newVMTakeover(t, vmPre)
	link := f.takeOver(t, "windows-11", "win11")
	f.backUp(t, "win11", "post1", linkBase.Add(time.Hour))
	f.backUpAnother(t, "windows-11", "other1", linkBase.Add(24*time.Hour))

	svc, st := f.afterConfigLoss(t)
	a := wantVMBesideLink(t, st, "windows-11", "win11", link.LinkedAt)
	wantDefinition(t, a.PrevDefinition, f.oldDef)
	other, err := st.GetVMTargetByName("windows-11")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(other.Definition, otherVMUUID) {
		t.Fatalf("windows-11 was rebuilt from %q, want the other VM's definition", other.Definition)
	}
	if got := vmSnapshotIDs(t, svc, "win11"); got != "post1,pre1" {
		t.Fatalf("win11 snapshots = %s, want its own and the old name's from before the link", got)
	}
	if got := vmSnapshotIDs(t, svc, "windows-11"); got != "other1" {
		t.Fatalf("windows-11 snapshots = %s, want the other VM's", got)
	}

	rebuilt := api.NewService(f.cfg, st, &fakeServiceDocker{}, f.virsh, f.eng)
	before := vmState(t, st)
	if err := rebuilt.UnlinkVMAlias(context.Background(), "windows-11"); err == nil {
		t.Fatal("unlinking windows-11 must be refused while another VM's entry and backups sit under it")
	}
	if after := vmState(t, st); after != before {
		t.Fatalf("a refused unlink changed the store:\nbefore\n%s\nafter\n%s", before, after)
	}
}

// The other VM's backups are gone, so the rebuild links the old name alone.
// The definition it keeps is still the entry's own, whose UUID is not the
// other VM's, so an unlink onto that VM is refused.
func TestDiscoverVMsKeepsTheEntrysOwnDefinitionAfterAnotherVMRewroteTheOldNamesFile(t *testing.T) {
	f := newVMTakeover(t, vmPre)
	link := f.takeOver(t, "windows-11", "win11")
	f.backUp(t, "win11", "post1", linkBase.Add(time.Hour))
	f.backUpAnother(t, "windows-11", "other1", linkBase.Add(24*time.Hour))
	f.forget("other1")

	_, st := f.afterConfigLoss(t)
	wantVMAlias(t, st, "windows-11", "win11", link.LinkedAt)
	a, err := st.AliasByOldName("vm", "windows-11")
	if err != nil {
		t.Fatal(err)
	}
	wantDefinition(t, a.PrevDefinition, f.oldDef)

	rebuilt := api.NewService(f.cfg, st, &fakeServiceDocker{}, f.virsh, f.eng)
	before := vmState(t, st)
	if err := rebuilt.UnlinkVMAlias(context.Background(), "windows-11"); err == nil || !strings.Contains(err.Error(), "rename that VM") {
		t.Fatalf("UnlinkVMAlias = %v, want a refusal naming the other VM", err)
	}
	if after := vmState(t, st); after != before {
		t.Fatalf("a refused unlink changed the store:\nbefore\n%s\nafter\n%s", before, after)
	}
}

func TestDiscoverVMsKeepsAnUnlinkedVMApart(t *testing.T) {
	f := newVMTakeover(t, vmPre)
	f.takeOver(t, "windows-11", "win11")
	f.backUp(t, "win11", "post1", linkBase.Add(time.Hour))
	if err := f.svc.UnlinkVMAlias(context.Background(), "windows-11"); err != nil {
		t.Fatalf("UnlinkVMAlias: %v", err)
	}

	svc, st := f.afterConfigLoss(t)
	wantNoVMAlias(t, st, "windows-11")
	if got := vmSnapshotIDs(t, svc, "windows-11"); got != "pre1" {
		t.Fatalf("windows-11 snapshots = %s, want its own pre1", got)
	}
}

func TestDiscoverVMsDatesALinkByItsRecordAfterRetention(t *testing.T) {
	f := newVMTakeover(t, vmPre)
	link := f.takeOver(t, "windows-11", "win11")
	f.backUp(t, "win11", "post1", linkBase.Add(time.Hour))
	f.backUp(t, "win11", "post2", linkBase.Add(20*24*time.Hour))
	f.eng.snapsByRepo[f.repo] = append(f.eng.snapsByRepo[f.repo], restic.Snapshot{ID: "stranger1", Time: linkBase.Add(10 * 24 * time.Hour).Format(time.RFC3339), Tags: []string{"vm:windows-11", "p2"}})
	f.forget("post1")

	svc, st := f.afterConfigLoss(t)
	wantVMBesideLink(t, st, "windows-11", "win11", link.LinkedAt)
	if got := vmSnapshotIDs(t, svc, "win11"); got != "post2,pre1" {
		t.Fatalf("win11 snapshots = %s, want post2 and pre1 without the stranger's", got)
	}
	if got := vmSnapshotIDs(t, svc, "windows-11"); got != "stranger1" {
		t.Fatalf("windows-11 snapshots = %s, want the stranger's alone", got)
	}
}

func TestDiscoverVMsRebuildsTheOldNameWhenTheClaimantsDefinitionDoesNotDecrypt(t *testing.T) {
	f := newVMTakeover(t, vmPre)
	f.takeOver(t, "windows-11", "win11")
	f.backUp(t, "win11", "post1", linkBase.Add(time.Hour))
	if err := os.WriteFile(filepath.Join(f.repo, "vm-def", "win11.def"), []byte("not encrypted with this key"), 0o600); err != nil {
		t.Fatal(err)
	}

	var st *store.Repo
	out := captureLog(func() { _, st = f.afterConfigLoss(t) })
	wantNoVMAlias(t, st, "windows-11")
	if want := `"windows-11" is a former name on the backups of ["win11"], but no readable stored definition records the link`; !strings.Contains(out, want) {
		t.Fatalf("log = %s\nwant a line containing %s", out, want)
	}
}

func TestDiscoverVMsRebuildsAVMRenamedBackUnderItsOwnName(t *testing.T) {
	f := newVMTakeover(t, vmPre)
	f.takeOver(t, "windows-11", "win11")
	f.backUp(t, "win11", "post1", linkBase.Add(-time.Hour))
	back := f.takeOver(t, "win11", "windows-11")
	f.backUp(t, "windows-11", "back1", linkBase.Add(2*time.Hour))

	svc, st := f.afterConfigLoss(t)
	wantVMAlias(t, st, "win11", "windows-11", back.LinkedAt)
	if got := vmSnapshotIDs(t, svc, "windows-11"); got != "back1,post1,pre1" {
		t.Fatalf("windows-11 snapshots = %s, want all three", got)
	}
}

func TestDiscoverVMsRelinksAVMTakenOverAgainBeforeItsNextBackup(t *testing.T) {
	f := newVMTakeover(t, vmPre)
	f.takeOver(t, "windows-11", "win11")
	f.backUp(t, "win11", "post1", linkBase.Add(-time.Hour))
	if err := f.svc.UnlinkVMAlias(context.Background(), "windows-11"); err != nil {
		t.Fatalf("UnlinkVMAlias: %v", err)
	}
	again := f.takeOver(t, "windows-11", "win11")

	_, st := f.afterConfigLoss(t)
	wantVMAlias(t, st, "windows-11", "win11", again.LinkedAt)
	a, err := st.AliasByOldName("vm", "windows-11")
	if err != nil {
		t.Fatal(err)
	}
	wantDefinition(t, a.PrevDefinition, f.oldDef)
}

func TestDiscoverVMsRebuildsAChainAfterUnlinkingItsMiddleName(t *testing.T) {
	f := newVMTakeoverNamed(t, "windows-10", "windows-11", restic.Snapshot{ID: "a1", Time: linkBase.Add(-30 * 24 * time.Hour).Format(time.RFC3339), Tags: []string{"vm:windows-10", "p2"}})
	first := f.takeOver(t, "windows-10", "windows-11")
	f.backUp(t, "windows-11", "b1", linkBase.Add(-time.Hour))
	f.takeOver(t, "windows-11", "win11")
	f.backUp(t, "win11", "c1", linkBase.Add(time.Hour))
	if err := f.svc.UnlinkVMAlias(context.Background(), "windows-11"); err != nil {
		t.Fatalf("UnlinkVMAlias: %v", err)
	}

	_, st := f.afterConfigLoss(t)
	wantVMAlias(t, st, "windows-10", "windows-11", first.LinkedAt)
	c, err := st.GetVMTargetByName("win11")
	if err != nil {
		t.Fatalf("win11 must be rebuilt as its own entry: %v", err)
	}
	if names, _ := st.AliasNames("vm", c.ID); len(names) != 0 {
		t.Fatalf("win11 aliases = %v, want none", names)
	}
}

// A VM definition file from before link records has no aliases field and
// still rebuilds its entry with the definition as written.
func TestAVMDefinitionWithoutLinkRecordsStillDecodes(t *testing.T) {
	const def = `{"domain_xml":"<domain><name>ubuntu</name></domain>","disk_paths":["/x/vdisk1.img"],"nvram_host_path":"","method":"live","was_autostart":true}`
	svc, st, _ := vmsAfterConfigLoss(t, []restic.Snapshot{{ID: "u1", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:ubuntu", "p2"}}}, map[string]string{"ubuntu": def})

	if _, _, err := svc.DiscoverVMs(context.Background(), false); err != nil {
		t.Fatalf("DiscoverVMs: %v", err)
	}
	tg, err := st.GetVMTargetByName("ubuntu")
	if err != nil {
		t.Fatalf("ubuntu must be rebuilt: %v", err)
	}
	if tg.Definition != def || tg.Method != "live" {
		t.Fatalf("rebuilt entry = %+v, want the stored definition as written", tg)
	}
}

func TestUnlinkVMAliasRefusesWhileItsLinkRecordCannotBeRemoved(t *testing.T) {
	f := newVMTakeover(t, vmPre)
	f.takeOver(t, "windows-11", "win11")
	blockMirror(t, filepath.Join(f.repo, "vm-def"))
	before := vmState(t, f.st)

	err := f.svc.UnlinkVMAlias(context.Background(), "windows-11")
	if err == nil || !strings.Contains(err.Error(), "could not be removed") {
		t.Fatalf("UnlinkVMAlias = %v, want a refusal because the record stays", err)
	}
	if after := vmState(t, f.st); after != before {
		t.Fatalf("a refused unlink changed the store:\nbefore\n%s\nafter\n%s", before, after)
	}
}

func TestTakeOverVMRefusesWhileTheLeftNamesLinkRecordsCannotBeRemoved(t *testing.T) {
	f := newVMTakeover(t, vmPre)
	blockMirror(t, filepath.Join(f.repo, "vm-def"))
	before := vmState(t, f.st)

	err := f.svc.TakeOverVM(context.Background(), "windows-11", "win11")
	if err == nil || !strings.Contains(err.Error(), "could not be removed") {
		t.Fatalf("TakeOverVM = %v, want a refusal because the old name's records stay", err)
	}
	if after := vmState(t, f.st); after != before {
		t.Fatalf("a refused takeover changed the store:\nbefore\n%s\nafter\n%s", before, after)
	}
}

func TestAVMBackupKeepsTheLinkRecordWhenItCannotReadTheAliases(t *testing.T) {
	f := newVMTakeover(t, vmPre)
	link := f.takeOver(t, "windows-11", "win11")
	f.backUp(t, "win11", "post1", linkBase.Add(time.Hour))
	breakAliases(t, f.db, "vm", f.id)
	f.backUp(t, "win11", "post2", linkBase.Add(2*time.Hour))

	_, st := f.afterConfigLoss(t)
	wantVMAlias(t, st, "windows-11", "win11", link.LinkedAt)
}

func TestDiscoverVMsKeepsAVMUnlinkedAfterARebuildApart(t *testing.T) {
	f := newVMTakeover(t, vmPre)
	f.takeOver(t, "windows-11", "win11")
	f.backUp(t, "win11", "post1", linkBase.Add(time.Hour))
	rebuilt, _ := f.afterConfigLoss(t)
	if err := rebuilt.UnlinkVMAlias(context.Background(), "windows-11"); err != nil {
		t.Fatalf("UnlinkVMAlias: %v", err)
	}

	_, st := f.afterConfigLoss(t)
	wantNoVMAlias(t, st, "windows-11")
}
