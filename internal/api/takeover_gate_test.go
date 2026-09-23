package api_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// storeState lists every container row and every alias, so a test can tell
// that a refused operation wrote nothing.
func storeState(t *testing.T, st *store.Repo) string {
	t.Helper()
	targets, err := st.ListTargets()
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, tg := range targets {
		lines = append(lines, fmt.Sprintf("row %s=%s", tg.ID, tg.ContainerName))
	}
	for _, domain := range []string{"container", "vm"} {
		aliases, err := st.ListAliases(domain)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range aliases {
			lines = append(lines, fmt.Sprintf("alias %s:%s->%s@%d", a.Domain, a.OldName, a.TargetID, a.LinkedAt))
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// takeoverService has the local containers repository established with snaps
// in it, and the given containers installed.
func takeoverService(t *testing.T, st *store.Repo, snaps []restic.Snapshot, installed ...string) (*api.Service, *fakeResticEngine, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := establishLocalRepo(t, dir, s.ContainersPath)
	d := &fakeServiceDocker{}
	for _, name := range installed {
		d.listOut = append(d.listOut, dockercli.ContainerInfo{Name: name})
	}
	eng := &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{repo: snaps}}
	return api.NewService(cfg, st, d, fakeVirsh{}, eng), eng, repo
}

// radarr-hd was renamed away from radarr, so radarr still names its older
// backups. Taking another entry over onto radarr would put two entries'
// pre-rename histories under one name. The empty row on radarr stays too,
// since the refusal comes before any write.
func TestTakeOverContainerRefusesAnotherEntrysFormerName(t *testing.T) {
	st := newMemStore(t)
	c, err := st.UpsertTarget(store.Target{ContainerName: "radarr-hd"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr", c.ID, unixOf(t, linkTime)); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"radarr-movies", "radarr"} {
		if _, err := st.UpsertTarget(store.Target{ContainerName: name}); err != nil {
			t.Fatal(err)
		}
	}
	svc, _, _ := takeoverService(t, st, nil, "radarr")
	before := storeState(t, st)

	err = svc.TakeOverContainer(context.Background(), "radarr-movies", "radarr")
	if err == nil || !strings.Contains(err.Error(), `"radarr-hd"`) {
		t.Fatalf("takeover = %v, want a refusal naming radarr-hd", err)
	}
	if after := storeState(t, st); after != before {
		t.Fatalf("a refused takeover changed the store:\nbefore\n%s\nafter\n%s", before, after)
	}
}

// The entry on radarr-movies took up radarr's former name. Its own name
// cannot become its former name as well, since a former name belongs to one
// entry, and the empty row on radarr-new stays.
func TestTakeOverContainerRefusesAnEntryOnAnotherEntrysFormerName(t *testing.T) {
	st := newMemStore(t)
	a, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr-movies", a.ID, unixOf(t, linkTime)); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"radarr-movies", "radarr-new"} {
		if _, err := st.UpsertTarget(store.Target{ContainerName: name}); err != nil {
			t.Fatal(err)
		}
	}
	svc, _, _ := takeoverService(t, st, nil, "radarr-new")
	before := storeState(t, st)

	err = svc.TakeOverContainer(context.Background(), "radarr-movies", "radarr-new")
	if err == nil || !strings.Contains(err.Error(), `"radarr"`) {
		t.Fatalf("takeover = %v, want a refusal naming radarr", err)
	}
	if after := storeState(t, st); after != before {
		t.Fatalf("a refused takeover changed the store:\nbefore\n%s\nafter\n%s", before, after)
	}
}

// The row on radarr owns backups only through its former name radarr-old, so
// with the alias table failing between two reads it would look empty, and it
// must not be deleted to make room.
func TestTakeOverContainerKeepsAnOccupiedRowWhoseFormerNamesCannotBeRead(t *testing.T) {
	db, st := openStore(t)
	if _, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies"}); err != nil {
		t.Fatal(err)
	}
	b, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr-old", b.ID, unixOf(t, linkTime)); err != nil {
		t.Fatal(err)
	}
	bPre := restic.Snapshot{ID: "bpre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-old", "p1"}}
	svc, eng, _ := takeoverService(t, st, []restic.Snapshot{bPre}, "radarr")
	eng.onSnapshots = func(call int) {
		if call == 1 {
			breakAliasReads(t, db)
		}
	}

	if err := svc.TakeOverContainer(context.Background(), "radarr-movies", "radarr"); err == nil {
		t.Fatal("the takeover must be refused when the occupied row's backups cannot be counted")
	}
	if got, err := st.GetTargetByContainer("radarr"); err != nil || got.ID != b.ID {
		t.Fatalf("the row on radarr must stay: %+v, %v", got, err)
	}
}

// radarr was taken over to radarr-hd and the container is renamed back. The
// entry owns radarr again without a link time, and radarr-hd becomes its
// former name, so the backups under both names stay its own.
func TestTakeOverContainerRenamedBackKeepsBothHistories(t *testing.T) {
	pre := restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}}
	hd := restic.Snapshot{ID: "hd1", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:radarr-hd", "p1", "formerly:radarr"}}
	linked := func(t *testing.T) (*store.Repo, string) {
		t.Helper()
		st := newMemStore(t)
		a, err := st.UpsertTarget(store.Target{ContainerName: "radarr-hd"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddAliasAt("container", "radarr", a.ID, unixOf(t, linkTime)); err != nil {
			t.Fatal(err)
		}
		return st, a.ID
	}

	t.Run("only its own history under the name: renamed back", func(t *testing.T) {
		st, id := linked(t)
		svc, _, _ := takeoverService(t, st, []restic.Snapshot{pre, hd}, "radarr")
		if err := svc.TakeOverContainer(context.Background(), "radarr-hd", "radarr"); err != nil {
			t.Fatalf("TakeOverContainer: %v", err)
		}
		if got, err := st.GetTargetByContainer("radarr"); err != nil || got.ID != id {
			t.Fatalf("entry on radarr = %+v, %v; want the same row", got, err)
		}
		if _, err := st.AliasByOldName("container", "radarr"); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("radarr is the entry's name again, its alias must be gone: %v", err)
		}
		if a, err := st.AliasByOldName("container", "radarr-hd"); err != nil || a.TargetID != id {
			t.Fatalf("radarr-hd alias = %+v, %v; want it linked to the entry", a, err)
		}
		snaps, err := svc.Snapshots(context.Background(), "radarr", "")
		if err != nil {
			t.Fatalf("Snapshots: %v", err)
		}
		if got := snapshotIDs(snaps); joined(got) != "hd1,pre1" {
			t.Fatalf("owned = %v, want both histories", got)
		}
	})

	post := restic.Snapshot{ID: "post1", Time: "2024-09-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}}
	at := restic.Snapshot{ID: "at1", Time: linkTime, Tags: []string{"container:radarr", "p1"}}
	for _, c := range []struct {
		name  string
		snaps []restic.Snapshot
	}{
		{"another machine's backup after the link: refused", []restic.Snapshot{pre, hd, post}},
		{"a backup at the link: refused", []restic.Snapshot{pre, hd, at}},
		{"only another machine's backups under the name: refused", []restic.Snapshot{hd, post}},
	} {
		t.Run(c.name, func(t *testing.T) {
			st, _ := linked(t)
			svc, _, _ := takeoverService(t, st, c.snaps, "radarr")
			before := storeState(t, st)
			if err := svc.TakeOverContainer(context.Background(), "radarr-hd", "radarr"); err == nil {
				t.Fatal("renaming back must be refused while radarr holds a backup from the link on")
			}
			if after := storeState(t, st); after != before {
				t.Fatalf("a refused takeover changed the store:\nbefore\n%s\nafter\n%s", before, after)
			}
		})
	}

	t.Run("the listing fails: refused", func(t *testing.T) {
		st, _ := linked(t)
		svc, eng, repo := takeoverService(t, st, []restic.Snapshot{pre, hd}, "radarr")
		eng.snapsErrFor = map[string]error{repo: errors.New("repository unreadable")}
		before := storeState(t, st)
		if err := svc.TakeOverContainer(context.Background(), "radarr-hd", "radarr"); err == nil {
			t.Fatal("renaming back must be refused when radarr's backups cannot be read")
		}
		if after := storeState(t, st); after != before {
			t.Fatalf("a refused takeover changed the store:\nbefore\n%s\nafter\n%s", before, after)
		}
	})

	t.Run("both names installed: refused", func(t *testing.T) {
		st, _ := linked(t)
		svc, _, _ := takeoverService(t, st, []restic.Snapshot{pre, hd}, "radarr", "radarr-hd")
		before := storeState(t, st)
		if err := svc.TakeOverContainer(context.Background(), "radarr-hd", "radarr"); err == nil {
			t.Fatal("renaming back must be refused while a container still runs under radarr-hd")
		}
		if after := storeState(t, st); after != before {
			t.Fatalf("a refused takeover changed the store:\nbefore\n%s\nafter\n%s", before, after)
		}
	})
}

// linkedRadarr is entry "radarr", taken over from "radarr-movies" at linkTime
// with one backup from before, on a host whose containers d lists.
func linkedRadarr(t *testing.T, d *fakeServiceDocker) (*api.Service, *store.Repo) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
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
	pre := restic.Snapshot{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}}
	eng := &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{repo: {pre}}}
	return api.NewService(cfg, st, d, fakeVirsh{}, eng), st
}

// Unlinking moves the entry onto its old name. With another container
// installed there next to the entry's own, the entry and its schedule would
// land on that container, so the unlink is refused, and so is one that cannot
// tell which containers are installed.
func TestUnlinkContainerAliasRefusesWhileAnotherContainerHasTheOldName(t *testing.T) {
	for _, c := range []struct {
		name string
		d    *fakeServiceDocker
		want string
	}{
		{"both names installed", &fakeServiceDocker{listOut: []dockercli.ContainerInfo{{Name: "radarr"}, {Name: "radarr-movies"}}}, "rename that container"},
		{"the containers cannot be listed", &fakeServiceDocker{listErr: errors.New("docker socket unreachable")}, "can be listed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			svc, st := linkedRadarr(t, c.d)
			before := storeState(t, st)
			err := svc.UnlinkContainerAlias(context.Background(), "radarr-movies")
			if err == nil || !strings.Contains(err.Error(), `"radarr-movies"`) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("unlink = %v, want a refusal naming radarr-movies and containing %q", err, c.want)
			}
			if after := storeState(t, st); after != before {
				t.Fatalf("a refused unlink changed the store:\nbefore\n%s\nafter\n%s", before, after)
			}
		})
	}
}

// With one of the two names installed there is no other container to land
// on: the container was renamed back, or the old name is free.
func TestUnlinkContainerAliasGoesThroughWithOneOfTheNamesInstalled(t *testing.T) {
	for _, installed := range []string{"radarr-movies", "radarr"} {
		t.Run(installed, func(t *testing.T) {
			svc, st := linkedRadarr(t, &fakeServiceDocker{listOut: []dockercli.ContainerInfo{{Name: installed}}})
			if err := svc.UnlinkContainerAlias(context.Background(), "radarr-movies"); err != nil {
				t.Fatalf("UnlinkContainerAlias: %v", err)
			}
			if _, err := st.GetTargetByContainer("radarr-movies"); err != nil {
				t.Fatalf("the entry must be back on radarr-movies: %v", err)
			}
		})
	}
}

// A restore from an entry linked to BombVault's own container would stop
// BombVault halfway, so no entry moves onto it.
func TestTakeOverContainerRefusesBombVaultsOwnContainer(t *testing.T) {
	t.Setenv("BOMBVAULT_SELF_CONTAINER", "bombvault")
	st := newMemStore(t)
	if _, err := st.UpsertTarget(store.Target{ContainerName: "bombvault-old"}); err != nil {
		t.Fatal(err)
	}
	svc, _, _ := takeoverService(t, st, nil, "bombvault")
	before := storeState(t, st)

	err := svc.TakeOverContainer(context.Background(), "bombvault-old", "bombvault")
	if err == nil || !strings.Contains(err.Error(), "BombVault's own container") {
		t.Fatalf("takeover = %v, want a refusal naming BombVault's own container", err)
	}
	if after := storeState(t, st); after != before {
		t.Fatalf("a refused takeover changed the store:\nbefore\n%s\nafter\n%s", before, after)
	}
}

// After a takeover the off-site copies of the new name's backups are the
// entry's too, and the next off-site prune ages them as one group with its
// own. So they are checked like the local repository, and a copy that cannot
// be read refuses.
func TestTakeOverContainerChecksTheOffsiteCopiesOfTheNewName(t *testing.T) {
	stranger := restic.Snapshot{ID: "other1", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}}
	own := restic.Snapshot{ID: "own1", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:radarr", "p1", "formerly:radarr-movies"}}
	for _, c := range []struct {
		name    string
		offsite []restic.Snapshot
		listErr error
		want    string
	}{
		{name: "another entry's backup off-site only: refused", offsite: []restic.Snapshot{stranger}, want: `off-site target "Primary"`},
		{name: "the off-site copy cannot be read: refused", listErr: errors.New("connection refused"), want: `off-site target "Primary"`},
		{name: "the entry's own earlier backup off-site: taken over", offsite: []restic.Snapshot{own}},
	} {
		t.Run(c.name, func(t *testing.T) {
			st := newMemStore(t)
			if _, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies"}); err != nil {
				t.Fatal(err)
			}
			svc, eng, _ := takeoverService(t, st, nil, "radarr")
			s := mustSettings(t, st)
			s.ContainersOffsite = unlinkOffsiteRepo
			if err := st.UpdateSettings(s); err != nil {
				t.Fatal(err)
			}
			eng.snapsByRepo[unlinkOffsiteRepo] = c.offsite
			if c.listErr != nil {
				eng.snapsErrFor = map[string]error{unlinkOffsiteRepo: c.listErr}
			}
			before := storeState(t, st)

			err := svc.TakeOverContainer(context.Background(), "radarr-movies", "radarr")
			if c.want == "" {
				if err != nil {
					t.Fatalf("TakeOverContainer: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("takeover = %v, want a refusal containing %q", err, c.want)
			}
			if after := storeState(t, st); after != before {
				t.Fatalf("a refused takeover changed the store:\nbefore\n%s\nafter\n%s", before, after)
			}
		})
	}
}
