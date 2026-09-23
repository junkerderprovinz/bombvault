package api_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// These tests pin the rule that decides which old-name snapshots an entry may
// claim: an alias claims a snapshot of its old name only if that snapshot was
// taken before the alias's linked_at. Snapshots under the entry's current tag
// belong to it whatever their time.

// linkTime is the moment every alias in these tests was linked.
const linkTime = "2024-06-01T00:00:00Z"

func unixOf(t *testing.T, rfc3339 string) int64 {
	t.Helper()
	ts, err := time.Parse(time.RFC3339Nano, rfc3339)
	if err != nil {
		t.Fatal(err)
	}
	return ts.Unix()
}

func snapshotIDs(snaps []restic.Snapshot) []string {
	out := make([]string, 0, len(snaps))
	for _, s := range snaps {
		out = append(out, s.ID)
	}
	sort.Strings(out)
	return out
}

func sortedCopy(ids []string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}

// establishLocalRepo resolves rel under root and drops restic's config marker
// there, so a listing reaches the engine instead of reading "no repo yet".
func establishLocalRepo(t *testing.T, root, rel string) string {
	t.Helper()
	repo, err := paths.Resolve(root, rel)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return repo
}

// The container reader keeps an alias-tagged snapshot only when it predates
// the link, keeps every current-tag snapshot regardless of time, and never
// asks Docker whether the old name is live again.
func TestSnapshotsAliasClaimsOnlyPreLinkHistory(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	establishLocalRepo(t, dir, s.ContainersPath)
	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr-movies", tg.ID, unixOf(t, linkTime)); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "a1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}},
		// Half a second before the link, written in another zone with a fraction.
		{ID: "a2", Time: "2024-06-01T01:59:59.5+02:00", Tags: []string{"container:radarr-movies", "p1"}},
		{ID: "a3", Time: linkTime, Tags: []string{"container:radarr-movies", "p1"}},               // at the link: not before it
		{ID: "a4", Time: "2024-09-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}, // the returned machine's
		{ID: "a5", Time: "not a time", Tags: []string{"container:radarr-movies", "p1"}},           // unprovable: fail closed
		{ID: "b1", Time: "2024-09-02T00:00:00Z", Tags: []string{"container:radarr", "p1"}},
		{ID: "b2", Time: "", Tags: []string{"container:radarr", "p1"}}, // current tag: time plays no part
		{ID: "c1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:sonarr", "p1"}},
	}}
	// The old name is a live container again. The link time already tells the
	// two machines apart, so Docker is not asked.
	d := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{{Name: "radarr-movies", ID: "b2"}}}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

	snaps, err := svc.Snapshots(context.Background(), "radarr", "")
	if err != nil {
		t.Fatalf("Snapshots: %v", err)
	}
	if got, want := snapshotIDs(snaps), []string{"a1", "a2", "b1", "b2"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("owned snapshots = %v, want %v", got, want)
	}
	for _, c := range d.calls {
		if c == "list" {
			t.Fatalf("the reader asked Docker for its live containers: %v", d.calls)
		}
	}
}

// TestSnapshotsVMAliasClaimsOnlyPreLinkHistory is the VM twin, through
// SnapshotsVM.
func TestSnapshotsVMAliasClaimsOnlyPreLinkHistory(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.VMsPath = "backups/vms"
	s.VMsEnabled = true
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	establishLocalRepo(t, dir, s.VMsPath)
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("vm", "windows-11", tg.ID, unixOf(t, linkTime)); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "a1", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}},
		{ID: "a3", Time: linkTime, Tags: []string{"vm:windows-11", "p2"}},
		{ID: "a4", Time: "2024-09-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}},
		{ID: "a5", Time: "", Tags: []string{"vm:windows-11", "p2"}},
		{ID: "b1", Time: "2024-09-02T00:00:00Z", Tags: []string{"vm:win11", "p2"}},
		{ID: "b2", Time: "garbage", Tags: []string{"vm:win11", "p2"}},
		{ID: "c1", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:ubuntu", "p2"}},
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	snaps, err := svc.SnapshotsVM(context.Background(), "win11", "")
	if err != nil {
		t.Fatalf("SnapshotsVM: %v", err)
	}
	if got, want := snapshotIDs(snaps), []string{"a1", "b1", "b2"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("owned snapshots = %v, want %v", got, want)
	}
}

// Delete-all forgets by the IDs the reader returned, so the returned
// machine's snapshot under the old name survives while the entry's own
// history (both names) goes.
func TestDeleteBackupsSparesPostLinkAliasSnapshot(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "rest:http://192.168.1.9:8000/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr", AppdataPaths: []string{"/x"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr-movies", tg.ID, unixOf(t, linkTime)); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}},
		{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}},
		{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}},
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	if err := svc.DeleteBackups(context.Background(), "radarr"); err != nil {
		t.Fatalf("DeleteBackups: %v", err)
	}
	if got, want := sortedCopy(eng.forgotten), []string{"own1", "pre1"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("forgotten = %v, want %v (post1 is another machine's)", got, want)
	}
}

// TestDeleteBackupsVMSparesPostLinkAliasSnapshot is the VM twin.
func TestDeleteBackupsVMSparesPostLinkAliasSnapshot(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.VMsPath = "rest:http://192.168.1.9:8000/vms"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("vm", "windows-11", tg.ID, unixOf(t, linkTime)); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}},
		{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}},
		{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"vm:win11", "p2"}},
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	if err := svc.DeleteBackupsVM(context.Background(), "win11", ""); err != nil {
		t.Fatalf("DeleteBackupsVM: %v", err)
	}
	if got, want := sortedCopy(eng.forgotten), []string{"own1", "pre1"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("forgotten = %v, want %v (post1 is another machine's)", got, want)
	}
}

// containerRetentionService wires a container backup of "radarr" (alias
// radarr-movies, linked at linkTime) through to its post-backup retention.
func containerRetentionService(t *testing.T, snaps []restic.Snapshot) (*api.Service, *fakeResticEngine, string) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	s.RetentionKeepLast = 3
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := establishLocalRepo(t, root, s.ContainersPath)
	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr-movies", tg.ID, unixOf(t, linkTime)); err != nil {
		t.Fatal(err)
	}
	appdata := root + "/appdata/radarr"
	if err := os.MkdirAll(appdata, 0o750); err != nil {
		t.Fatal(err)
	}
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name:    "/radarr",
		Image:   "radarr:latest",
		Running: true,
		Mounts:  []model.Mount{{Type: "bind", Source: appdata, Destination: "/config"}},
	}}
	eng := &fakeResticEngine{snaps: snaps}
	return api.NewService(cfg, st, d, fakeVirsh{}, eng), eng, repo
}

// restic forget selects by tag and knows no time bound, so the alias tag may
// join the retention group only while every snapshot under it is the entry's
// own.
func TestBackupRetentionTakesAliasOnlyWhileAllItsSnapshotsPredateLink(t *testing.T) {
	pre := restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	post := restic.Snapshot{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	own := restic.Snapshot{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}}

	t.Run("only pre-link history: both tags", func(t *testing.T) {
		svc, eng, _ := containerRetentionService(t, []restic.Snapshot{pre, own})
		if _, err := svc.Backup(context.Background(), "radarr"); err != nil {
			t.Fatalf("Backup: %v", err)
		}
		if len(eng.forgetTags) != 1 || eng.forgetTags[0] != "container:radarr,container:radarr-movies" {
			t.Fatalf("retention tags = %v, want the current and the alias tag in one group", eng.forgetTags)
		}
	})
	t.Run("a post-link snapshot under the old name: current tag only", func(t *testing.T) {
		svc, eng, _ := containerRetentionService(t, []restic.Snapshot{pre, own, post})
		if _, err := svc.Backup(context.Background(), "radarr"); err != nil {
			t.Fatalf("Backup: %v", err)
		}
		if len(eng.forgetTags) != 1 || eng.forgetTags[0] != "container:radarr" {
			t.Fatalf("retention tags = %v, want container:radarr alone", eng.forgetTags)
		}
	})
	t.Run("an alias snapshot with an unreadable time: current tag only", func(t *testing.T) {
		odd := restic.Snapshot{ID: "odd1", Time: "?", Tags: []string{"container:radarr-movies", "p1"}}
		svc, eng, _ := containerRetentionService(t, []restic.Snapshot{pre, own, odd})
		if _, err := svc.Backup(context.Background(), "radarr"); err != nil {
			t.Fatalf("Backup: %v", err)
		}
		if len(eng.forgetTags) != 1 || eng.forgetTags[0] != "container:radarr" {
			t.Fatalf("retention tags = %v, want container:radarr alone", eng.forgetTags)
		}
	})
	t.Run("the listing fails: current tag only", func(t *testing.T) {
		svc, eng, repo := containerRetentionService(t, []restic.Snapshot{pre, own})
		eng.snapsErrFor = map[string]error{repo: errors.New("repository unreadable")}
		if _, err := svc.Backup(context.Background(), "radarr"); err != nil {
			t.Fatalf("Backup: %v", err)
		}
		if len(eng.forgetTags) != 1 || eng.forgetTags[0] != "container:radarr" {
			t.Fatalf("retention tags = %v, want container:radarr alone", eng.forgetTags)
		}
	})
}

// TestBackupVMRetentionTakesAliasOnlyWhileAllItsSnapshotsPredateLink is the
// VM twin, through the real BackupVM retention call site.
func TestBackupVMRetentionTakesAliasOnlyWhileAllItsSnapshotsPredateLink(t *testing.T) {
	pre := restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:plainvm-old", "p2"}}
	post := restic.Snapshot{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"vm:plainvm-old", "p2"}}
	own := restic.Snapshot{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"vm:plainvm", "p2"}}

	run := func(t *testing.T, snaps []restic.Snapshot, listErr error) []string {
		t.Helper()
		svc, eng, st, root := vmZvolTestService(t, fileOnlyVMDomainXML, &zvolTPMSSH{})
		repo := establishLocalRepo(t, root, "backups/vms")
		eng.snaps = snaps
		if listErr != nil {
			eng.snapsErrFor = map[string]error{repo: listErr}
		}
		tg, err := st.UpsertVMTarget(store.VMTarget{Name: "plainvm"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddAliasAt("vm", "plainvm-old", tg.ID, unixOf(t, linkTime)); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.BackupVM(context.Background(), "plainvm"); err != nil {
			t.Fatalf("BackupVM: %v", err)
		}
		return eng.forgetTags
	}

	if got := run(t, []restic.Snapshot{pre, own}, nil); len(got) != 1 || got[0] != "vm:plainvm,vm:plainvm-old" {
		t.Fatalf("only pre-link history: retention tags = %v, want both in one group", got)
	}
	if got := run(t, []restic.Snapshot{pre, own, post}, nil); len(got) != 1 || got[0] != "vm:plainvm" {
		t.Fatalf("post-link snapshot: retention tags = %v, want vm:plainvm alone", got)
	}
	if got := run(t, []restic.Snapshot{pre, own}, errors.New("ssh: connection reset")); len(got) != 1 || got[0] != "vm:plainvm" {
		t.Fatalf("listing fails: retention tags = %v, want vm:plainvm alone", got)
	}
}

// Only the pre-link snapshots of an old name fold into the owner's date. A
// post-link one is the returned machine's and keeps the old name's own key.
func TestLatestContainerBackupTimesKeepsPostLinkSnapshotUnderOldName(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	establishLocalRepo(t, dir, s.ContainersPath)
	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr-movies", tg.ID, unixOf(t, linkTime)); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "own1", Time: "2024-02-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}},
		{ID: "pre1", Time: "2024-05-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}},
		{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}},
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	times, err := svc.LatestContainerBackupTimes(context.Background())
	if err != nil {
		t.Fatalf("LatestContainerBackupTimes: %v", err)
	}
	if got, want := times["radarr"].Newest(), unixOf(t, "2024-05-01T00:00:00Z"); got != want {
		t.Fatalf("radarr = %d, want its newest pre-link snapshot %d (%+v)", got, want, times)
	}
	if got, want := times["radarr-movies"].Newest(), unixOf(t, "2024-09-01T00:00:00Z"); got != want {
		t.Fatalf("radarr-movies = %d, want the returned machine's own %d (%+v)", got, want, times)
	}
}

// The per-identity retention pass folds an old name into its owner's group
// under the same rule as the post-backup one, and leaves a tag that mixes the
// owner's pre-link snapshots with a later machine's out of every group.
func TestPruneDomainFoldsAliasOnlyWhileAllItsSnapshotsPredateLink(t *testing.T) {
	run := func(t *testing.T, snaps []restic.Snapshot) []string {
		t.Helper()
		dir := t.TempDir()
		cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
		st := newMemStore(t)
		s := mustSettings(t, st)
		s.ContainersPath = "backups/containers"
		s.RetentionKeepLast = 3
		if err := st.UpdateSettings(s); err != nil {
			t.Fatal(err)
		}
		establishLocalRepo(t, dir, s.ContainersPath)
		tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddAliasAt("container", "radarr-movies", tg.ID, unixOf(t, linkTime)); err != nil {
			t.Fatal(err)
		}
		eng := &fakeResticEngine{snaps: snaps}
		svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)
		if _, err := svc.PruneDomain(context.Background(), "containers", ""); err != nil {
			t.Fatalf("PruneDomain: %v", err)
		}
		return sortedCopy(eng.forgetTags)
	}
	pre := restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	post := restic.Snapshot{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	own := restic.Snapshot{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}}

	if got := run(t, []restic.Snapshot{own, pre}); len(got) != 1 || got[0] != "container:radarr,container:radarr-movies" {
		t.Fatalf("only pre-link history: forget groups = %v, want one folded group", got)
	}
	// Both machines' snapshots under one tag: no tag-scoped forget can keep the
	// owner's apart, so that tag gets none. As its own group, every backup of
	// the returned machine would age one of the owner's out.
	if got := run(t, []restic.Snapshot{own, pre, post}); len(got) != 1 || got[0] != "container:radarr" {
		t.Fatalf("post-link snapshot: forget groups = %v, want [container:radarr] only", got)
	}
}

// A /config-loss rebuild re-creates the alias with the linked_at radarr's
// stored definition records, not the time of its first backup after the
// rename and not the time Discover runs. Either would move the claim's bound.
func TestDiscoverLinksAFoldedAliasAtItsRecordedTime(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := establishLocalRepo(t, dir, s.ContainersPath)
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}},
		{ID: "bbbb2222", Time: "2024-06-01T12:30:00.75Z", Tags: []string{"container:radarr", "p1", "formerly:radarr-movies"}},
		{ID: "bbbb3333", Time: "2024-08-01T00:00:00Z", Tags: []string{"container:radarr", "p1", "formerly:radarr-movies"}},
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)
	writeStoredDef(t, svc, repo, "radarr", "radarr-movies")
	writeStoredDef(t, svc, repo, "radarr-movies")

	if _, _, err := svc.Discover(context.Background(), false); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	a, err := st.AliasByOldName("container", "radarr-movies")
	if err != nil {
		t.Fatalf("radarr-movies must be folded into an alias: %v", err)
	}
	if want := unixOf(t, linkTime); a.LinkedAt != want {
		t.Fatalf("linked_at = %d, want the recorded %d", a.LinkedAt, want)
	}
}

// A recorded former name with no snapshots of its own is linked all the same,
// so later backups keep naming it.
func TestDiscoverLinksARecordedFormerNameWithoutSnapshotsOfItsOwn(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := establishLocalRepo(t, dir, s.ContainersPath)
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "bbbb2222", Time: "2024-06-01T12:30:00Z", Tags: []string{"container:radarr", "p1", "formerly:radarr-movies"}},
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)
	writeStoredDef(t, svc, repo, "radarr", "radarr-movies")

	if _, _, err := svc.Discover(context.Background(), false); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	a, err := st.AliasByOldName("container", "radarr-movies")
	if err != nil {
		t.Fatalf("radarr-movies must be folded into an alias: %v", err)
	}
	if want := unixOf(t, linkTime); a.LinkedAt != want {
		t.Fatalf("linked_at = %d, want the recorded %d", a.LinkedAt, want)
	}
}
