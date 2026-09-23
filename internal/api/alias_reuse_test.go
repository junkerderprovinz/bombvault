package api_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// These tests pin the other side of the link-time rule: once an old name is
// linked to one entry, its snapshots from before the link are that entry's,
// whoever answers to the name later. They also pin entries that were renamed
// twice, and the retention paths that cannot filter by time.

// earlyLink is the first link of an entry renamed twice; linkTime is the
// second.
const earlyLink = "2024-03-01T00:00:00Z"

func joined(ids []string) string { return strings.Join(ids, ",") }

// reusedNameStore holds entry A ("radarr", renamed from "radarr-movies" at
// linkTime) and, when withB, entry B, a different container that took the
// name "radarr-movies" up again.
func reusedNameStore(t *testing.T, withB bool) *store.Repo {
	t.Helper()
	st := newMemStore(t)
	a, err := st.UpsertTarget(store.Target{ContainerName: "radarr", AppdataPaths: []string{"/x"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr-movies", a.ID, unixOf(t, linkTime)); err != nil {
		t.Fatal(err)
	}
	if withB {
		if _, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies", AppdataPaths: []string{"/x"}}); err != nil {
			t.Fatal(err)
		}
	}
	return st
}

// The entry that answers to an old name owns only that name's snapshots from
// the link on. The earlier ones are the alias owner's, with or without a row
// for the new machine, and a snapshot whose time cannot be read is nobody's.
func TestSnapshotsOfAReusedNameLeaveTheAliasOwnersHistory(t *testing.T) {
	for _, withB := range []bool{true, false} {
		dir := t.TempDir()
		cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
		st := reusedNameStore(t, withB)
		s := mustSettings(t, st)
		s.ContainersPath = "backups/containers"
		if err := st.UpdateSettings(s); err != nil {
			t.Fatal(err)
		}
		establishLocalRepo(t, dir, s.ContainersPath)
		eng := &fakeResticEngine{snaps: []restic.Snapshot{
			{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}},
			{ID: "atlink", Time: linkTime, Tags: []string{"container:radarr-movies", "p1"}},
			{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}},
			{ID: "odd1", Time: "garbage", Tags: []string{"container:radarr-movies", "p1"}},
			{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}},
		}}
		svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

		b, err := svc.Snapshots(context.Background(), "radarr-movies", "")
		if err != nil {
			t.Fatalf("Snapshots(radarr-movies): %v", err)
		}
		if got, want := snapshotIDs(b), "atlink,post1"; joined(got) != want {
			t.Fatalf("row for the new machine %v: owned = %v, want %s", withB, got, want)
		}
		a, err := svc.Snapshots(context.Background(), "radarr", "")
		if err != nil {
			t.Fatalf("Snapshots(radarr): %v", err)
		}
		if got, want := snapshotIDs(a), "own1,pre1"; joined(got) != want {
			t.Fatalf("row for the new machine %v: the alias owner's = %v, want %s", withB, got, want)
		}
	}
}

// TestSnapshotsVMOfAReusedNameLeaveTheAliasOwnersHistory is the VM twin.
func TestSnapshotsVMOfAReusedNameLeaveTheAliasOwnersHistory(t *testing.T) {
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
	a, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("vm", "windows-11", a.ID, unixOf(t, linkTime)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11"}); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}},
		{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}},
		{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"vm:win11", "p2"}},
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	snaps, err := svc.SnapshotsVM(context.Background(), "windows-11", "")
	if err != nil {
		t.Fatalf("SnapshotsVM: %v", err)
	}
	if got := snapshotIDs(snaps); joined(got) != "post1" {
		t.Fatalf("owned = %v, want [post1]", got)
	}
}

// Delete-all of the new machine forgets by the IDs its reader returned, so
// the alias owner's pre-link snapshots survive it.
func TestDeleteBackupsOfAReusedNameSparesTheAliasOwnersHistory(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := reusedNameStore(t, true)
	s := mustSettings(t, st)
	s.ContainersPath = "rest:http://192.168.1.9:8000/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}},
		{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}},
		{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}},
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	if err := svc.DeleteBackups(context.Background(), "radarr-movies", ""); err != nil {
		t.Fatalf("DeleteBackups: %v", err)
	}
	if got := sortedCopy(eng.forgotten); joined(got) != "post1" {
		t.Fatalf("forgotten = %v, want [post1] (pre1 is radarr's history)", got)
	}
}

// TestDeleteBackupsVMOfAReusedNameSparesTheAliasOwnersHistory is the VM twin.
func TestDeleteBackupsVMOfAReusedNameSparesTheAliasOwnersHistory(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.VMsPath = "rest:http://192.168.1.9:8000/vms"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	a, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("vm", "windows-11", a.ID, unixOf(t, linkTime)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11"}); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}},
		{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}},
		{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"vm:win11", "p2"}},
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	if err := svc.DeleteBackupsVM(context.Background(), "windows-11", ""); err != nil {
		t.Fatalf("DeleteBackupsVM: %v", err)
	}
	if got := sortedCopy(eng.forgotten); joined(got) != "post1" {
		t.Fatalf("forgotten = %v, want [post1] (pre1 is win11's history)", got)
	}
}

// An entry renamed twice holds two aliases, and each claims its own old name
// up to its own link. A snapshot of the first old name taken after the first
// link is not the entry's, even though it predates the second link.
func TestSnapshotsTwoAliasesEachClaimOnlyBeforeItsOwnLink(t *testing.T) {
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
	if _, err := st.AddAliasAt("container", "radarr-old", tg.ID, unixOf(t, earlyLink)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr-movies", tg.ID, unixOf(t, linkTime)); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "old-pre", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-old", "p1"}},
		{ID: "old-mid", Time: "2024-04-01T00:00:00Z", Tags: []string{"container:radarr-old", "p1"}}, // after its own link, before the second
		{ID: "movies-pre", Time: "2024-05-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}},
		{ID: "movies-post", Time: "2024-09-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}},
		{ID: "cur", Time: "2024-10-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}},
	}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	snaps, err := svc.Snapshots(context.Background(), "radarr", "")
	if err != nil {
		t.Fatalf("Snapshots: %v", err)
	}
	if got, want := snapshotIDs(snaps), "cur,movies-pre,old-pre"; joined(got) != want {
		t.Fatalf("owned = %v, want %s", got, want)
	}
}

// retentionBackupService wires a container backup of name through to its
// post-backup retention. setup adds whatever rows and aliases the case needs.
func retentionBackupService(t *testing.T, name string, snaps []restic.Snapshot, setup func(st *store.Repo)) (*api.Service, *fakeResticEngine, string) {
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
	setup(st)
	appdata := root + "/appdata/" + name
	if err := os.MkdirAll(appdata, 0o750); err != nil {
		t.Fatal(err)
	}
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name:    "/" + name,
		Image:   name + ":latest",
		Running: true,
		Mounts:  []model.Mount{{Type: "bind", Source: appdata, Destination: "/config"}},
	}}
	eng := &fakeResticEngine{snaps: snaps}
	return api.NewService(cfg, st, d, fakeVirsh{}, eng), eng, repo
}

// With two aliases, one alias that has a post-link snapshot is left out of
// retention without taking the other with it, and without letting the other
// in when it too is mixed.
func TestBackupRetentionDecidesEachOfTwoAliasesOnItsOwn(t *testing.T) {
	oldPre := restic.Snapshot{ID: "old-pre", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-old", "p1"}}
	oldPost := restic.Snapshot{ID: "old-post", Time: "2024-04-01T00:00:00Z", Tags: []string{"container:radarr-old", "p1"}}
	moviesPre := restic.Snapshot{ID: "movies-pre", Time: "2024-05-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	moviesPost := restic.Snapshot{ID: "movies-post", Time: "2024-09-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	cur := restic.Snapshot{ID: "cur", Time: "2024-10-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}}
	twoAliases := func(st *store.Repo) {
		tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddAliasAt("container", "radarr-old", tg.ID, unixOf(t, earlyLink)); err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddAliasAt("container", "radarr-movies", tg.ID, unixOf(t, linkTime)); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		name  string
		snaps []restic.Snapshot
		want  string
	}{
		{"first alias mixed", []restic.Snapshot{oldPre, oldPost, moviesPre, cur}, "container:radarr,container:radarr-movies"},
		{"second alias mixed", []restic.Snapshot{oldPre, moviesPre, moviesPost, cur}, "container:radarr,container:radarr-old"},
		{"both mixed", []restic.Snapshot{oldPre, oldPost, moviesPre, moviesPost, cur}, "container:radarr"},
	} {
		t.Run(c.name, func(t *testing.T) {
			svc, eng, _ := retentionBackupService(t, "radarr", c.snaps, twoAliases)
			if _, err := svc.Backup(context.Background(), "radarr"); err != nil {
				t.Fatalf("Backup: %v", err)
			}
			if len(eng.forgetTags) != 1 || eng.forgetTags[0] != c.want {
				t.Fatalf("retention tags = %v, want [%s]", eng.forgetTags, c.want)
			}
		})
	}
}

// restic forget selects by tag, so the new machine's retention would age the
// alias owner's pre-link snapshots under its own policy. It is paused while
// any of them sit under its tag, and runs normally once none do.
func TestBackupRetentionOfAReusedNamePausesWhileItHoldsTheAliasOwnersHistory(t *testing.T) {
	pre := restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	post := restic.Snapshot{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	own := restic.Snapshot{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}}
	rows := func(st *store.Repo) {
		a, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
		if err != nil {
			t.Fatal(err)
		}
		// The reusing machine is settled on the domain path, so its backup runs
		// even when the listing that would settle it fails.
		if _, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies", RepoChosen: store.RepoChosen}); err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddAliasAt("container", "radarr-movies", a.ID, unixOf(t, linkTime)); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("the alias owner's history is still there: paused", func(t *testing.T) {
		svc, eng, _ := retentionBackupService(t, "radarr-movies", []restic.Snapshot{pre, own, post}, rows)
		if _, err := svc.Backup(context.Background(), "radarr-movies"); err != nil {
			t.Fatalf("Backup: %v", err)
		}
		if len(eng.forgetTags) != 0 {
			t.Fatalf("retention ran with %v; it must pause while radarr's pre-link snapshots sit under the tag", eng.forgetTags)
		}
	})
	t.Run("only its own snapshots under the tag: runs", func(t *testing.T) {
		svc, eng, _ := retentionBackupService(t, "radarr-movies", []restic.Snapshot{own, post}, rows)
		if _, err := svc.Backup(context.Background(), "radarr-movies"); err != nil {
			t.Fatalf("Backup: %v", err)
		}
		if len(eng.forgetTags) != 1 || eng.forgetTags[0] != "container:radarr-movies" {
			t.Fatalf("retention tags = %v, want [container:radarr-movies]", eng.forgetTags)
		}
	})
	t.Run("the listing fails: paused", func(t *testing.T) {
		svc, eng, repo := retentionBackupService(t, "radarr-movies", []restic.Snapshot{own, post}, rows)
		eng.snapsErrFor = map[string]error{repo: errors.New("repository unreadable")}
		if _, err := svc.Backup(context.Background(), "radarr-movies"); err != nil {
			t.Fatalf("Backup: %v", err)
		}
		if len(eng.forgetTags) != 0 {
			t.Fatalf("retention ran with %v on a failed listing; it must pause", eng.forgetTags)
		}
	})
}

// TestBackupVMRetentionOfAReusedNamePausesWhileItHoldsTheAliasOwnersHistory
// is the VM twin, through BackupVM.
func TestBackupVMRetentionOfAReusedNamePausesWhileItHoldsTheAliasOwnersHistory(t *testing.T) {
	pre := restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:plainvm", "p2"}}
	post := restic.Snapshot{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"vm:plainvm", "p2"}}
	run := func(t *testing.T, snaps []restic.Snapshot) []string {
		t.Helper()
		svc, eng, st, root := vmZvolTestService(t, fileOnlyVMDomainXML, &zvolTPMSSH{})
		establishLocalRepo(t, root, "backups/vms")
		eng.snaps = snaps
		a, err := st.UpsertVMTarget(store.VMTarget{Name: "plainvm-new"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddAliasAt("vm", "plainvm", a.ID, unixOf(t, linkTime)); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.BackupVM(context.Background(), "plainvm"); err != nil {
			t.Fatalf("BackupVM: %v", err)
		}
		return eng.forgetTags
	}
	if got := run(t, []restic.Snapshot{pre, post}); len(got) != 0 {
		t.Fatalf("retention ran with %v; it must pause while plainvm-new's pre-link snapshots sit under the tag", got)
	}
	if got := run(t, []restic.Snapshot{post}); len(got) != 1 || got[0] != "vm:plainvm" {
		t.Fatalf("retention tags = %v, want [vm:plainvm]", got)
	}
}

// pruneGroups runs a manual Prune of domain and returns the forget groups the
// per-identity pass issued, sorted.
func pruneGroups(t *testing.T, domain string, snaps []restic.Snapshot, setup func(st *store.Repo)) []string {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	s.VMsPath = "backups/vms"
	s.RetentionKeepLast = 3
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	establishLocalRepo(t, dir, s.ContainersPath)
	establishLocalRepo(t, dir, s.VMsPath)
	setup(st)
	eng := &fakeResticEngine{snaps: snaps}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)
	if err := svc.PruneDomain(context.Background(), domain, ""); err != nil {
		t.Fatalf("PruneDomain: %v", err)
	}
	return sortedCopy(eng.forgetTags)
}

// The per-identity pass cannot tell the alias owner's pre-link snapshots from
// the new machine's under one tag, so that tag gets no forget at all while
// both are there, whether or not a row holds the name.
func TestPruneDomainLeavesAMixedOldNameAloneWhenARowHoldsIt(t *testing.T) {
	pre := restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	post := restic.Snapshot{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	own := restic.Snapshot{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}}
	withB := func(st *store.Repo) {
		a, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddAliasAt("container", "radarr-movies", a.ID, unixOf(t, linkTime)); err != nil {
			t.Fatal(err)
		}
		if _, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies"}); err != nil {
			t.Fatal(err)
		}
	}
	if got := pruneGroups(t, "containers", []restic.Snapshot{own, pre, post}, withB); joined(got) != "container:radarr" {
		t.Fatalf("mixed old name held by a row: forget groups = %v, want [container:radarr] only", got)
	}
	// Only the alias owner's history under the name: kept apart from the owner
	// while a row holds the name, but aged as its own group.
	if got := pruneGroups(t, "containers", []restic.Snapshot{own, pre}, withB); len(got) != 2 || joined(got) != "container:radarr,container:radarr-movies" {
		t.Fatalf("clean old name held by a row: forget groups = %q, want two separate groups", got)
	}
	// Only the new machine's snapshots under the name: its own group.
	if got := pruneGroups(t, "containers", []restic.Snapshot{own, post}, withB); len(got) != 2 || joined(got) != "container:radarr,container:radarr-movies" {
		t.Fatalf("new machine's snapshots only: forget groups = %q, want two separate groups", got)
	}
}

// The per-identity pass folds a renamed VM's old name like a container's,
// under the same rule, and leaves a mixed old name alone.
func TestPruneDomainFoldsVMAliasOnlyWhileAllItsSnapshotsPredateLink(t *testing.T) {
	pre := restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}}
	post := restic.Snapshot{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}}
	own := restic.Snapshot{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"vm:win11", "p2"}}
	rows := func(withB bool) func(st *store.Repo) {
		return func(st *store.Repo) {
			a, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := st.AddAliasAt("vm", "windows-11", a.ID, unixOf(t, linkTime)); err != nil {
				t.Fatal(err)
			}
			if withB {
				if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11"}); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if got := pruneGroups(t, "vms", []restic.Snapshot{own, pre}, rows(false)); joined(got) != "vm:win11,vm:windows-11" || len(got) != 1 {
		t.Fatalf("only pre-link history: forget groups = %v, want one folded group", got)
	}
	if got := pruneGroups(t, "vms", []restic.Snapshot{own, pre, post}, rows(false)); joined(got) != "vm:win11" {
		t.Fatalf("mixed old name: forget groups = %v, want [vm:win11] only", got)
	}
	if got := pruneGroups(t, "vms", []restic.Snapshot{own, pre, post}, rows(true)); joined(got) != "vm:win11" {
		t.Fatalf("mixed old name held by a row: forget groups = %v, want [vm:win11] only", got)
	}
	if got := pruneGroups(t, "vms", []restic.Snapshot{own, pre}, rows(true)); len(got) != 2 {
		t.Fatalf("clean old name held by a row: forget groups = %v, want two separate groups", got)
	}
}

// C was renamed from radarr to radarr-hd, and A was later renamed from
// radarr-movies onto radarr before its first backup there. radarr holds only
// C's older snapshots, A's former name only A's, and each is aged on its own.
func TestPruneDomainKeepsAFormerNameApartFromTheRowThatHoldsIt(t *testing.T) {
	for _, c := range []struct {
		domain, prefix string
		upsert         func(st *store.Repo, name string) string
	}{
		{"containers", "container:", func(st *store.Repo, name string) string {
			tg, err := st.UpsertTarget(store.Target{ContainerName: name})
			if err != nil {
				t.Fatal(err)
			}
			return tg.ID
		}},
		{"vms", "vm:", func(st *store.Repo, name string) string {
			tg, err := st.UpsertVMTarget(store.VMTarget{Name: name})
			if err != nil {
				t.Fatal(err)
			}
			return tg.ID
		}},
	} {
		t.Run(c.domain, func(t *testing.T) {
			aliasDomain := strings.TrimSuffix(c.prefix, ":")
			setup := func(st *store.Repo) {
				if _, err := st.AddAliasAt(aliasDomain, "radarr", c.upsert(st, "radarr-hd"), unixOf(t, earlyLink)); err != nil {
					t.Fatal(err)
				}
				if _, err := st.AddAliasAt(aliasDomain, "radarr-movies", c.upsert(st, "radarr"), unixOf(t, linkTime)); err != nil {
					t.Fatal(err)
				}
			}
			snaps := []restic.Snapshot{
				{ID: "c-pre", Time: "2024-01-01T00:00:00Z", Tags: []string{c.prefix + "radarr"}},
				{ID: "c-own", Time: "2024-10-01T00:00:00Z", Tags: []string{c.prefix + "radarr-hd"}},
				{ID: "a-pre", Time: "2024-05-01T00:00:00Z", Tags: []string{c.prefix + "radarr-movies"}},
			}
			got := pruneGroups(t, c.domain, snaps, setup)
			want := []string{c.prefix + "radarr", c.prefix + "radarr-hd", c.prefix + "radarr-movies"}
			if len(got) != len(want) || joined(got) != joined(want) {
				t.Fatalf("forget groups = %q, want three separate groups %q", got, want)
			}
		})
	}
}

// Without a listing the per-identity pass cannot see an identity, let alone
// an alias, so it stops rather than fall back to a repository-wide forget.
func TestPruneDomainForgetsNothingWhenTheListingFails(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	s.RetentionKeepLast = 3
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := establishLocalRepo(t, dir, s.ContainersPath)
	eng := &fakeResticEngine{snapsErrFor: map[string]error{repo: errors.New("repository unreadable")}}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	if err := svc.PruneDomain(context.Background(), "containers", ""); err == nil {
		t.Fatal("a prune whose listing failed must report it")
	}
	if len(eng.prunedRepos) != 0 || len(eng.manualPruned) != 0 {
		t.Fatalf("nothing may be forgotten or pruned without a listing: forget %v, prune %v", eng.forgetTags, eng.manualPruned)
	}
}

// A repository written before identity tags existed still gets a
// repository-wide pass.
func TestPruneDomainKeepsTheLegacyPassForUntaggedRepositories(t *testing.T) {
	got := pruneGroups(t, "containers", []restic.Snapshot{{ID: "old1", Time: "2023-01-01T00:00:00Z", Tags: []string{"p1"}}}, func(*store.Repo) {})
	if len(got) != 1 || got[0] != "" {
		t.Fatalf("forget groups = %q, want one untagged repository-wide pass", got)
	}
}

// unlinkOffsiteRepo is the legacy off-site column unlinkService sets, which
// the service reads as off-site target "Primary".
const unlinkOffsiteRepo = "rest:http://192.168.1.9:8000/offsite"

// unlinkService holds entry "radarr", taken over from "radarr-movies" at
// linkTime, with its local repository established and the given snapshots in
// it.
func unlinkService(t *testing.T, snaps []restic.Snapshot, offsite []restic.Snapshot) (*api.Service, *store.Repo, *fakeResticEngine, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	if offsite != nil {
		s.ContainersOffsite = unlinkOffsiteRepo
	}
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := establishLocalRepo(t, dir, s.ContainersPath)
	a, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr-movies", a.ID, unixOf(t, linkTime)); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{repo: snaps, unlinkOffsiteRepo: offsite}}
	d := &fakeServiceDocker{listOut: nil}
	return api.NewService(cfg, st, d, fakeVirsh{}, eng), st, eng, repo
}

// Unlinking moves the entry back onto the old tag, which carries no time
// bound, so the returned machine's snapshots under that name would become the
// entry's own. It is refused until the old name holds nothing from after the
// link, in the entry's repository and its off-site copies, and a failed
// listing refuses too.
func TestUnlinkContainerAliasRefusedWhileTheOldNameHoldsNewerBackups(t *testing.T) {
	pre := restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	post := restic.Snapshot{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	own := restic.Snapshot{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}}

	stillLinked := func(t *testing.T, st *store.Repo) {
		t.Helper()
		if _, err := st.AliasByOldName("container", "radarr-movies"); err != nil {
			t.Fatalf("the alias must stay after a refused unlink: %v", err)
		}
		if _, err := st.GetTargetByContainer("radarr"); err != nil {
			t.Fatalf("the entry must keep its name after a refused unlink: %v", err)
		}
	}

	t.Run("a newer snapshot under the old name: refused", func(t *testing.T) {
		svc, st, _, _ := unlinkService(t, []restic.Snapshot{pre, own, post}, nil)
		err := svc.UnlinkContainerAlias(context.Background(), "radarr-movies")
		if err == nil || !strings.Contains(err.Error(), "radarr-movies") {
			t.Fatalf("unlink = %v, want a refusal that names radarr-movies", err)
		}
		stillLinked(t, st)
	})
	t.Run("only a later machine's snapshots under the old name: refused", func(t *testing.T) {
		svc, st, _, _ := unlinkService(t, []restic.Snapshot{own, post}, nil)
		if err := svc.UnlinkContainerAlias(context.Background(), "radarr-movies"); err == nil {
			t.Fatal("unlink must be refused when the entry's own history under the old name is gone and a later machine's is left")
		}
		stillLinked(t, st)
	})
	t.Run("an off-site target that does not resolve: refused, naming it", func(t *testing.T) {
		svc, st, _, _ := unlinkService(t, []restic.Snapshot{pre, own}, nil)
		if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Cloud", Repo: "../outside", Enabled: true}); err != nil {
			t.Fatal(err)
		}
		err := svc.UnlinkContainerAlias(context.Background(), "radarr-movies")
		if err == nil || !strings.Contains(err.Error(), `off-site target "Cloud"`) {
			t.Fatalf("unlink = %v, want a refusal naming the off-site target it could not resolve", err)
		}
		stillLinked(t, st)
	})
	t.Run("a newer snapshot in the off-site copy: refused", func(t *testing.T) {
		svc, st, _, _ := unlinkService(t, []restic.Snapshot{pre, own}, []restic.Snapshot{pre, own, post})
		if err := svc.UnlinkContainerAlias(context.Background(), "radarr-movies"); err == nil {
			t.Fatal("unlink must be refused while the off-site copy holds a newer snapshot of the old name")
		}
		stillLinked(t, st)
	})
	t.Run("the listing fails: refused, naming the repository", func(t *testing.T) {
		svc, st, eng, repo := unlinkService(t, []restic.Snapshot{pre, own}, nil)
		eng.snapsErrFor = map[string]error{repo: errors.New("repository unreadable")}
		err := svc.UnlinkContainerAlias(context.Background(), "radarr-movies")
		if err == nil || !strings.Contains(err.Error(), "folder containers") {
			t.Fatalf("unlink = %v, want a refusal naming the repository it could not read", err)
		}
		stillLinked(t, st)
	})
	t.Run("the off-site listing fails: refused, naming the target", func(t *testing.T) {
		svc, st, eng, _ := unlinkService(t, []restic.Snapshot{pre, own}, []restic.Snapshot{pre, own})
		eng.snapsErrFor = map[string]error{unlinkOffsiteRepo: errors.New("connection refused")}
		err := svc.UnlinkContainerAlias(context.Background(), "radarr-movies")
		if err == nil || !strings.Contains(err.Error(), `off-site target "Primary"`) {
			t.Fatalf("unlink = %v, want a refusal naming the off-site target it could not read", err)
		}
		stillLinked(t, st)
	})
	t.Run("only the entry's own history under the old name: unlinked", func(t *testing.T) {
		svc, st, _, _ := unlinkService(t, []restic.Snapshot{pre, own}, []restic.Snapshot{pre, own})
		if err := svc.UnlinkContainerAlias(context.Background(), "radarr-movies"); err != nil {
			t.Fatalf("UnlinkContainerAlias: %v", err)
		}
		if _, err := st.GetTargetByContainer("radarr-movies"); err != nil {
			t.Fatalf("the entry must be back on its old name: %v", err)
		}
	})
}
