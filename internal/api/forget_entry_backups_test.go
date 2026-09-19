package api_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// forgetCase is one repository state for the row-only removal routes: what
// the entry's repository and the domain's off-site copy list, which of them
// fails, and whether the row may go.
type forgetCase struct {
	name          string
	local, remote []restic.Snapshot
	failLocal     bool
	failRemote    bool
	removed       bool
}

// forgetService has domain's local repository established and the legacy
// off-site column set, and lists c's snapshots there.
func forgetService(t *testing.T, st *store.Repo, domain string, c forgetCase) (*api.Service, *fakeResticEngine) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	s := mustSettings(t, st)
	var rel string
	if domain == "vms" {
		rel = "backups/vms"
		s.VMsPath, s.VMsOffsite = rel, unlinkOffsiteRepo
	} else {
		rel = "backups/containers"
		s.ContainersPath, s.ContainersOffsite = rel, unlinkOffsiteRepo
	}
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := establishLocalRepo(t, dir, rel)
	eng := &fakeResticEngine{
		snapsByRepo: map[string][]restic.Snapshot{repo: c.local, unlinkOffsiteRepo: c.remote},
		snapsErrFor: map[string]error{},
	}
	if c.failLocal {
		eng.snapsErrFor[repo] = errors.New("repository unreadable")
	}
	if c.failRemote {
		eng.snapsErrFor[unlinkOffsiteRepo] = errors.New("connection refused")
	}
	return api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng), eng
}

// The row carries the aliases that make the entry's older backups its own, so
// it stays while any backup it owns is left, locally or off-site, or while
// that cannot be read.
func TestForgetTargetRefusesWhileTheEntryOwnsBackups(t *testing.T) {
	own := restic.Snapshot{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}}
	pre := restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	post := restic.Snapshot{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	for _, c := range []forgetCase{
		{name: "its own backup in its repository", local: []restic.Snapshot{own}},
		{name: "a backup its alias claims", local: []restic.Snapshot{pre}},
		{name: "a backup only off-site", remote: []restic.Snapshot{own}},
		{name: "its repository cannot be read", failLocal: true},
		{name: "the off-site copy cannot be read", failRemote: true},
		{name: "only a later machine's backup under its former name", local: []restic.Snapshot{post}, removed: true},
		{name: "no backups", removed: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			st := newMemStore(t)
			tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := st.AddAliasAt("container", "radarr-movies", tg.ID, unixOf(t, linkTime)); err != nil {
				t.Fatal(err)
			}
			svc, _ := forgetService(t, st, "containers", c)

			err = svc.ForgetTarget(context.Background(), "radarr")
			_, rowErr := st.GetTargetByContainer("radarr")
			_, aliasErr := st.AliasByOldName("container", "radarr-movies")
			if c.removed {
				if err != nil || rowErr == nil || aliasErr == nil {
					t.Fatalf("ForgetTarget = %v; row left %v, alias left %v; want both gone", err, rowErr == nil, aliasErr == nil)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "radarr") {
				t.Fatalf("ForgetTarget = %v, want a refusal naming radarr", err)
			}
			if rowErr != nil || aliasErr != nil {
				t.Fatalf("a refused removal dropped the row (%v) or its alias (%v)", rowErr, aliasErr)
			}
		})
	}
}

// TestForgetVMTargetRefusesWhileTheEntryOwnsBackups is the VM twin.
func TestForgetVMTargetRefusesWhileTheEntryOwnsBackups(t *testing.T) {
	own := restic.Snapshot{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"vm:win11", "p2"}}
	for _, c := range []forgetCase{
		{name: "its own backup in its repository", local: []restic.Snapshot{own}},
		{name: "a backup only off-site", remote: []restic.Snapshot{own}},
		{name: "its repository cannot be read", failLocal: true},
		{name: "the off-site copy cannot be read", failRemote: true},
		{name: "no backups", removed: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			st := newMemStore(t)
			if _, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"}); err != nil {
				t.Fatal(err)
			}
			svc, _ := forgetService(t, st, "vms", c)

			err := svc.ForgetVMTarget(context.Background(), "win11")
			_, rowErr := st.GetVMTargetByName("win11")
			if c.removed {
				if err != nil || rowErr == nil {
					t.Fatalf("ForgetVMTarget = %v, row left %v; want it gone", err, rowErr == nil)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "win11") {
				t.Fatalf("ForgetVMTarget = %v, want a refusal naming win11", err)
			}
			if rowErr != nil {
				t.Fatalf("a refused removal dropped the row: %v", rowErr)
			}
		})
	}
}

// Without the entry's repository the check cannot rule out backups, so the
// row stays.
func TestForgetRefusesWhenTheEntrysRepositoryDoesNotResolve(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath, s.VMsPath = "../outside", "../outside"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "radarr"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"}); err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})

	if err := svc.ForgetTarget(context.Background(), "radarr"); err == nil {
		t.Fatal("ForgetTarget must refuse when the entry's repository does not resolve")
	}
	if _, err := st.GetTargetByContainer("radarr"); err != nil {
		t.Fatalf("the container row must stay: %v", err)
	}
	if err := svc.ForgetVMTarget(context.Background(), "win11"); err == nil {
		t.Fatal("ForgetVMTarget must refuse when the entry's repository does not resolve")
	}
	if _, err := st.GetVMTargetByName("win11"); err != nil {
		t.Fatalf("the VM row must stay: %v", err)
	}
}

// breakAliasReads makes every read of the alias table fail while a delete by
// target id still works, the way a read can fail between two statements.
func breakAliasReads(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("ALTER TABLE target_aliases RENAME COLUMN linked_at TO linked_at_unread"); err != nil {
		t.Fatal(err)
	}
}

// openStore is newMemStore with the database handle, for a test that breaks a
// table underneath the service.
func openStore(t *testing.T) (*sql.DB, *store.Repo) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return db, store.New(db)
}

// Without its aliases the entry would seem to own less than it does, so the
// row stays.
func TestForgetRefusesWhenTheEntrysFormerNamesCannotBeRead(t *testing.T) {
	t.Run("container", func(t *testing.T) {
		db, st := openStore(t)
		tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddAliasAt("container", "radarr-movies", tg.ID, unixOf(t, linkTime)); err != nil {
			t.Fatal(err)
		}
		own := restic.Snapshot{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}}
		pre := restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
		svc, _ := forgetService(t, st, "containers", forgetCase{local: []restic.Snapshot{own, pre}})
		breakAliasReads(t, db)

		err = svc.ForgetTarget(context.Background(), "radarr")
		if err == nil || !strings.Contains(err.Error(), "could not be checked") {
			t.Fatalf("ForgetTarget = %v, want a refusal saying the backups could not be checked", err)
		}
		if _, err := st.GetTargetByContainer("radarr"); err != nil {
			t.Fatalf("the row must stay: %v", err)
		}
	})
	t.Run("VM", func(t *testing.T) {
		db, st := openStore(t)
		tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddAliasAt("vm", "windows-11", tg.ID, unixOf(t, linkTime)); err != nil {
			t.Fatal(err)
		}
		pre := restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}}
		svc, _ := forgetService(t, st, "vms", forgetCase{local: []restic.Snapshot{pre}})
		breakAliasReads(t, db)

		err = svc.ForgetVMTarget(context.Background(), "win11")
		if err == nil || !strings.Contains(err.Error(), "could not be checked") {
			t.Fatalf("ForgetVMTarget = %v, want a refusal saying the backups could not be checked", err)
		}
		if _, err := st.GetVMTargetByName("win11"); err != nil {
			t.Fatalf("the row must stay: %v", err)
		}
	})
}

// The snapshots under radarr-movies from before its link are radarr's, so the
// entry that took the name up again owns none of them and its row can go,
// while radarr keeps its alias.
func TestForgetTargetRemovesAnEntryOnAnotherEntrysFormerNameThatOwnsNothing(t *testing.T) {
	pre := restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	st := reusedNameStore(t, true)
	svc, _ := forgetService(t, st, "containers", forgetCase{local: []restic.Snapshot{pre}, remote: []restic.Snapshot{pre}})

	if err := svc.ForgetTarget(context.Background(), "radarr-movies"); err != nil {
		t.Fatalf("ForgetTarget: %v", err)
	}
	if _, err := st.GetTargetByContainer("radarr-movies"); err == nil {
		t.Fatal("the row on radarr-movies must be gone")
	}
	if _, err := st.AliasByOldName("container", "radarr-movies"); err != nil {
		t.Fatalf("radarr's alias must stay: %v", err)
	}
}

// Without its aliases delete-all would forget only part of the entry's
// backups and then drop the aliases that make the rest its own, so nothing is
// deleted.
func TestDeleteBackupsRefusesWhenTheEntrysFormerNamesCannotBeRead(t *testing.T) {
	vmEntry := func(t *testing.T) (*sql.DB, *store.Repo) {
		t.Helper()
		db, st := openStore(t)
		tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddAliasAt("vm", "windows-11", tg.ID, unixOf(t, linkTime)); err != nil {
			t.Fatal(err)
		}
		return db, st
	}
	vmSnaps := []restic.Snapshot{
		{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"vm:win11", "p2"}},
		{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}},
	}
	refused := func(t *testing.T, err error, eng *fakeResticEngine) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), "could not be checked") {
			t.Fatalf("delete-all = %v, want a refusal saying the backups could not be checked", err)
		}
		if eng != nil && len(eng.forgotten) != 0 {
			t.Fatalf("nothing may be forgotten, got %v", eng.forgotten)
		}
	}

	t.Run("container", func(t *testing.T) {
		db, st := openStore(t)
		tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddAliasAt("container", "radarr-movies", tg.ID, unixOf(t, linkTime)); err != nil {
			t.Fatal(err)
		}
		own := restic.Snapshot{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}}
		pre := restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
		svc, eng := forgetService(t, st, "containers", forgetCase{local: []restic.Snapshot{own, pre}})
		breakAliasReads(t, db)

		refused(t, svc.DeleteBackups(context.Background(), "radarr"), eng)
		if _, err := st.GetTargetByContainer("radarr"); err != nil {
			t.Fatalf("the row must stay: %v", err)
		}
	})
	t.Run("VM", func(t *testing.T) {
		db, st := vmEntry(t)
		svc, eng := forgetService(t, st, "vms", forgetCase{local: vmSnaps})
		breakAliasReads(t, db)

		refused(t, svc.DeleteBackupsVM(context.Background(), "win11", ""), eng)
		if _, err := st.GetVMTargetByName("win11"); err != nil {
			t.Fatalf("the row must stay: %v", err)
		}
	})
	t.Run("VM off-site", func(t *testing.T) {
		db, st := vmEntry(t)
		svc, eng := forgetService(t, st, "vms", forgetCase{remote: vmSnaps})
		breakAliasReads(t, db)

		refused(t, svc.DeleteBackupsVM(context.Background(), "win11", "offsite"), eng)
	})
	t.Run("VM without a local repository", func(t *testing.T) {
		db, st := vmEntry(t)
		dir := t.TempDir()
		cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
		s := mustSettings(t, st)
		s.VMsPath = "backups/vms"
		if err := st.UpdateSettings(s); err != nil {
			t.Fatal(err)
		}
		svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
		breakAliasReads(t, db)

		refused(t, svc.DeleteBackupsVM(context.Background(), "win11", ""), nil)
		if _, err := st.GetVMTargetByName("win11"); err != nil {
			t.Fatalf("the row must stay: %v", err)
		}
	})
}
