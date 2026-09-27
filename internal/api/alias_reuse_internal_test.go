package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A machine that took an old name up again must not restore the alias owner's
// pre-link snapshot, neither as "latest" while it has none of its own, nor by
// an explicit id copied from anywhere.
func TestPrepareRestoreOfAReusedNameNeverPicksTheAliasOwnersSnapshot(t *testing.T) {
	st := newAliasLinkStore(t)
	const appdata = "/host/user/appdata/radarr-movies"
	a, err := st.UpsertTarget(store.Target{ContainerName: "radarr", AppdataPaths: []string{"/host/user/appdata/radarr"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr-movies", a.ID, linkedAt2024); err != nil {
		t.Fatal(err)
	}
	// B has a row before its first backup (SetInclude creates one).
	def, err := json.Marshal(containerDefinition{Inspect: model.Inspect{Name: "radarr-movies"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies", Definition: string(def), AppdataPaths: []string{appdata}}); err != nil {
		t.Fatal(err)
	}
	pre := restic.Snapshot{ID: "aaaa1111", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}, Paths: []string{appdata}}
	own := restic.Snapshot{ID: "bbbb2222", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}, Paths: []string{"/host/user/appdata/radarr"}}
	post := restic.Snapshot{ID: "cccc3333", Time: "2024-09-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}, Paths: []string{appdata}}
	ref := repoRef{repo: "rest:http://fake/containers", mode: restic.Mode{}}
	ctx := context.Background()

	s := &Service{store: st, cfg: config.Config{HostMountRoot: "/host/user"}, engine: aliasRestoreEngine{snaps: []restic.Snapshot{pre, own}}}
	if plan, err := s.prepareRestoreIn(ctx, ref, "radarr-movies", "latest", true); err != nil || !plan.recreateOnly || plan.snapshotID == pre.ID {
		t.Fatalf("latest with no snapshot of its own = %+v, %v; want a recreate from the definition alone", plan, err)
	}
	if _, err := s.prepareRestoreIn(ctx, ref, "radarr-movies", pre.ID, true); err == nil {
		t.Fatal("radarr's pre-link snapshot must not pass the new machine's ownership check")
	}

	s.engine = aliasRestoreEngine{snaps: []restic.Snapshot{pre, own, post}}
	plan, err := s.prepareRestoreIn(ctx, ref, "radarr-movies", "latest", true)
	if err != nil || plan.snapshotID != post.ID {
		t.Fatalf("latest = %+v, %v; want its own %s", plan.snapshotID, err, post.ID)
	}
}

// TestPrepareRestoreVMOfAReusedNameNeverPicksTheAliasOwnersSnapshot is the VM
// twin.
func TestPrepareRestoreVMOfAReusedNameNeverPicksTheAliasOwnersSnapshot(t *testing.T) {
	st := newAliasLinkStore(t)
	const diskPath = "/host/user/domains/windows-11/vdisk1.img"
	a, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("vm", "windows-11", a.ID, linkedAt2024); err != nil {
		t.Fatal(err)
	}
	def, err := json.Marshal(vmDefinition{DomainXML: "<domain/>", DiskPaths: []string{diskPath}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11", Definition: string(def)}); err != nil {
		t.Fatal(err)
	}
	pre := restic.Snapshot{ID: "aaaa1111", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}, Paths: []string{diskPath}}
	s := &Service{store: st, cfg: config.Config{HostMountRoot: "/host/user"}, engine: aliasRestoreEngine{snaps: []restic.Snapshot{pre}}}
	ref := repoRef{repo: "rest:http://fake/vms", mode: restic.Mode{}}
	ctx := context.Background()

	if plan, err := s.prepareRestoreVMIn(ctx, ref, "windows-11", "latest", true); err == nil && plan.snapshotID == pre.ID {
		t.Fatal("latest resolved to win11's pre-link snapshot")
	}
	if _, err := s.prepareRestoreVMIn(ctx, ref, "windows-11", pre.ID, true); err == nil {
		t.Fatal("win11's pre-link snapshot must not pass the new machine's ownership check")
	}
}

// An alias on the entry's own current name is its own, so it cedes nothing of
// the name to anyone. A rename back drops such an alias, so the state is
// built directly.
func TestContainerIdentityOwnsItsWholeNameDespiteItsOwnAliasOnIt(t *testing.T) {
	st := newAliasLinkStore(t)
	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr", tg.ID, linkedAt2024); err != nil {
		t.Fatal(err)
	}
	snaps := []restic.Snapshot{{ID: "old", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}}}
	eng := &retentionListEngine{snaps: snaps}
	s := &Service{store: st, engine: eng}
	if got := ownedByAny(snaps, s.containerIdentity("radarr")); len(got) != 1 {
		t.Fatalf("owned = %v, want its own snapshot from before it was renamed away", got)
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.RetentionKeepLast = 3
	s.applyRetention(context.Background(), "rest:http://fake/containers", settings, restic.Mode{}, s.containerIdentity("radarr"), "containers", anomalyScope{})
	if len(eng.forgetTags) != 1 || eng.forgetTags[0][0] != "container:radarr" {
		t.Fatalf("forget tag sets = %v; its own old snapshots must not pause its retention", eng.forgetTags)
	}
}

// When the alias table cannot be read, nothing tells whether another entry's
// alias claims the name's older snapshots, so none of them is provably this
// entry's.
func TestContainerIdentityClaimsNothingOfItsNameWhenAliasesAreUnreadable(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	if _, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies"}); err != nil {
		t.Fatal(err)
	}
	s := &Service{store: st}
	snaps := []restic.Snapshot{{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}}
	if got := ownedByAny(snaps, s.containerIdentity("radarr-movies")); len(got) != 1 {
		t.Fatalf("baseline: owned = %v, want its snapshot", got)
	}
	if _, err := db.Exec("DROP TABLE target_aliases"); err != nil {
		t.Fatal(err)
	}
	if got := ownedByAny(snaps, s.containerIdentity("radarr-movies")); len(got) != 0 {
		t.Fatalf("aliases unreadable: owned = %v, want nothing", got)
	}
}

// unlinkCheckEngine lists one fixed set of snapshots, or fails.
type unlinkCheckEngine struct {
	ResticEngine
	snaps []restic.Snapshot
	err   error
}

func (e unlinkCheckEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return e.snaps, e.err
}

// The unlink check is written for both domains, and UnlinkVMAlias makes it
// before the store unlinks.
func TestRefuseUnlinkWhileOldNameReusedCoversVMs(t *testing.T) {
	st := newAliasLinkStore(t)
	a, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	alias, err := st.AddAliasAt("vm", "windows-11", a.ID, linkedAt2024)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	pre := restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}}
	post := restic.Snapshot{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}}
	check := func(eng unlinkCheckEngine) error {
		s := &Service{store: st, cfg: config.Config{HostMountRoot: "/host/user"}, engine: eng}
		return s.refuseUnlinkWhileOldNameReused(context.Background(), settings, alias, "rest:http://fake/vms")
	}
	if err := check(unlinkCheckEngine{snaps: []restic.Snapshot{pre}}); err != nil {
		t.Fatalf("only the entry's own history: %v", err)
	}
	if err := check(unlinkCheckEngine{snaps: []restic.Snapshot{pre, post}}); err == nil {
		t.Fatal("a newer snapshot under the old name must refuse the unlink")
	}
	if err := check(unlinkCheckEngine{err: errors.New("unreachable")}); err == nil {
		t.Fatal("a failed listing must refuse the unlink")
	}
}

// A former name that holds only a later machine's snapshots holds none of the
// alias owner's, so the tag groups with whatever the row on that name folds
// under it, with or without that row.
func TestFoldTagKeysAFormerNameHoldingOnlyALaterMachinesSnapshotsByItsOwnTag(t *testing.T) {
	post := restic.Snapshot{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	for _, withRow := range []bool{false, true} {
		st := newAliasLinkStore(t)
		a, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddAliasAt("container", "radarr-movies", a.ID, linkedAt2024); err != nil {
			t.Fatal(err)
		}
		if withRow {
			if _, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies"}); err != nil {
				t.Fatal(err)
			}
		}
		s := &Service{store: st}
		canon, skip := s.foldTag("container:radarr-movies", []restic.Snapshot{post}, s.aliasFoldDomains())
		if skip || canon != "container:radarr-movies" {
			t.Fatalf("row on the name %v: foldTag = %q, skip %v; want its own tag", withRow, canon, skip)
		}
	}
}

// Each store read an identity rests on can fail on its own, and each leaves
// the identity partial.
func TestContainerIdentityRecordsEveryFailedRead(t *testing.T) {
	open := func(t *testing.T) (*sql.DB, *store.Repo, string) {
		t.Helper()
		db, err := store.Open(":memory:")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if err := store.Migrate(db); err != nil {
			t.Fatal(err)
		}
		st := store.New(db)
		tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
		if err != nil {
			t.Fatal(err)
		}
		return db, st, tg.ID
	}
	t.Run("readable: complete, with or without a row", func(t *testing.T) {
		_, st, _ := open(t)
		for _, name := range []string{"radarr", "sonarr"} {
			if err := (&Service{store: st}).containerIdentity(name).readErr; err != nil {
				t.Fatalf("%s: readErr = %v", name, err)
			}
		}
	})
	t.Run("the alias on a name without a row", func(t *testing.T) {
		db, st, _ := open(t)
		if _, err := db.Exec("DROP TABLE target_aliases"); err != nil {
			t.Fatal(err)
		}
		if (&Service{store: st}).containerIdentity("sonarr").readErr == nil {
			t.Fatal("an unread alias on the name must leave the identity partial")
		}
	})
	t.Run("the entry's own alias list", func(t *testing.T) {
		db, st, id := open(t)
		if _, err := db.Exec("INSERT INTO target_aliases (id, domain, old_name, target_id, linked_at) VALUES ('bad', 'container', 'radarr-old', ?, 'garbage')", id); err != nil {
			t.Fatal(err)
		}
		if (&Service{store: st}).containerIdentity("radarr").readErr == nil {
			t.Fatal("an unread alias list must leave the identity partial")
		}
	})
	t.Run("the entry's row", func(t *testing.T) {
		db, st, _ := open(t)
		if _, err := db.Exec("ALTER TABLE targets RENAME COLUMN definition TO definition_unread"); err != nil {
			t.Fatal(err)
		}
		if (&Service{store: st}).containerIdentity("radarr").readErr == nil {
			t.Fatal("an unread row must leave the identity partial")
		}
	})
}

// An entry whose only backups sit under a former name would read as empty
// without its aliases, so the question fails instead of answering no.
func TestHasBackupsFailsWhenTheFormerNamesCannotBeRead(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath, settings.VMsPath = "rest:http://fake/containers", "rest:http://fake/vms"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	c, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr-movies", c.ID, linkedAt2024); err != nil {
		t.Fatal(err)
	}
	v, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("vm", "windows-11", v.ID, linkedAt2024); err != nil {
		t.Fatal(err)
	}
	snaps := []restic.Snapshot{
		{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}},
		{ID: "pre2", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}},
	}
	s := &Service{store: st, cfg: config.Config{HostMountRoot: "/host/user"}, engine: unlinkCheckEngine{snaps: snaps}}
	ctx := context.Background()
	if had, err := s.containerHasBackups(ctx, "radarr"); err != nil || !had {
		t.Fatalf("baseline container = %v, %v; want its alias's backup", had, err)
	}
	if had, err := s.vmHasBackups(ctx, "win11"); err != nil || !had {
		t.Fatalf("baseline VM = %v, %v; want its alias's backup", had, err)
	}
	if _, err := db.Exec("ALTER TABLE target_aliases RENAME COLUMN linked_at TO linked_at_unread"); err != nil {
		t.Fatal(err)
	}
	if had, err := s.containerHasBackups(ctx, "radarr"); err == nil && !had {
		t.Fatal("container: an unread alias table must not answer no backups")
	}
	if had, err := s.vmHasBackups(ctx, "win11"); err == nil && !had {
		t.Fatal("VM: an unread alias table must not answer no backups")
	}
}

// Without the off-site target list the check cannot see a copy that holds a
// newer backup under the old name, so it refuses.
func TestRefuseUnlinkWhileOldNameReusedRefusesWhenTheOffsiteTargetsAreUnreadable(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	a, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	alias, err := st.AddAliasAt("container", "radarr-movies", a.ID, linkedAt2024)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	pre := restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	s := &Service{store: st, cfg: config.Config{HostMountRoot: "/host/user"}, engine: unlinkCheckEngine{snaps: []restic.Snapshot{pre}}}
	if err := s.refuseUnlinkWhileOldNameReused(context.Background(), settings, alias, "rest:http://fake/containers"); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	if _, err := db.Exec("DROP TABLE offsite_targets"); err != nil {
		t.Fatal(err)
	}
	if err := s.refuseUnlinkWhileOldNameReused(context.Background(), settings, alias, "rest:http://fake/containers"); err == nil {
		t.Fatal("an unreadable off-site target list must refuse the unlink")
	}
}
