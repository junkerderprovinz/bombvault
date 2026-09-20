package api

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestNamedRepoRoleIsInvisibleToTheOffsiteQueries is the load-bearing claim
// behind putting named repositories (#204) in offsite_targets at all: a third
// role is free because every query in that file filters on an explicit one.
//
// If it were not true, the replication loop would start copying backups INTO
// what is meant to be a primary location, and the off-site CRUD would offer it
// as a destination. That is why this is pinned rather than argued.
func TestNamedRepoRoleIsInvisibleToTheOffsiteQueries(t *testing.T) {
	st := newTestStore(t)

	named, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RoleRepo, Name: "Cold storage", Repo: "b2:bucket/cold", Enabled: true,
	})
	if err != nil {
		t.Fatalf("create named repo: %v", err)
	}
	// A real off-site destination beside it, so the queries have something to
	// return and "empty" cannot pass for "filtered".
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Offsite", Repo: "b2:bucket/offsite", Enabled: true,
	}); err != nil {
		t.Fatalf("create offsite target: %v", err)
	}

	all, err := st.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Name != "Offsite" {
		t.Fatalf("ListOffsiteTargets = %v, want only the replication destination", all)
	}
	forDomain, err := st.OffsiteTargetsForDomain("containers")
	if err != nil {
		t.Fatal(err)
	}
	if len(forDomain) != 1 || forDomain[0].Name != "Offsite" {
		t.Fatalf("OffsiteTargetsForDomain = %v, want only the replication destination", forDomain)
	}
	if _, found, _ := st.GetOffsiteTarget(named.ID); found {
		t.Fatal("a named repository must not be reachable through GetOffsiteTarget")
	}

	// And the other way round: the named-repo queries see only their own rows.
	repos, err := st.ListNamedRepos()
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].ID != named.ID {
		t.Fatalf("ListNamedRepos = %v, want only the named repository", repos)
	}
}

// TestItemRepoPathRefusesRatherThanFallingBack pins the decision that matters
// most here. An override that cannot be resolved is an ERROR; it never quietly
// becomes the domain repository.
//
// A fallback would send the next backup somewhere else and look exactly like a
// working backup - the run is green, the snapshot exists, it is simply in the
// wrong place, and nobody finds out until they go looking for a snapshot that
// is not where they expect it.
func TestItemRepoPathRefusesRatherThanFallingBack(t *testing.T) {
	dir := t.TempDir()
	st := newTestStore(t)
	svc := NewService(config.Config{
		AppKey:        strings.Repeat("a", 64),
		DataDir:       dir,
		HostMountRoot: dir,
	}, st, nil, nil, nil)

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	t.Run("no override takes the domain repository", func(t *testing.T) {
		got, err := svc.containerRepoPath(settings, store.Target{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(got, "backups/containers") {
			t.Fatalf("repo = %q, want the domain repository", got)
		}
	})

	t.Run("an id that does not exist is an error, not the domain repository", func(t *testing.T) {
		_, err := svc.containerRepoPath(settings, store.Target{Repo: "ffffffffffffffffffffffffffffffff"})
		if err == nil {
			t.Fatal("a dangling override must fail loudly; falling back would look like a working backup")
		}
		if !strings.Contains(err.Error(), "no longer exists") {
			t.Fatalf("error = %v, want it to say the repository is gone", err)
		}
	})

	t.Run("a switched-off repository is an error too", func(t *testing.T) {
		off, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
			Role: store.RoleRepo, Name: "Paused", Repo: "backups/paused", Enabled: false,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.containerRepoPath(settings, store.Target{Repo: off.ID}); err == nil {
			t.Fatal("a switched-off repository must fail rather than divert the backup")
		}
	})

	t.Run("a live override resolves to its own location", func(t *testing.T) {
		on, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
			Role: store.RoleRepo, Name: "Cold", Repo: "backups/cold", Enabled: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		got, err := svc.containerRepoPath(settings, store.Target{Repo: on.ID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(got, "backups/cold") {
			t.Fatalf("repo = %q, want the named repository's own location", got)
		}
	})
}

// TestDomainReposInUseCoversEveryItemsRepository pins what the dashboard reads.
// A container pointed at a named repository keeps its snapshots there, so an
// overview that only read the domain repository would report it as never backed
// up - the most alarming thing a backup tool can say, and wrong.
func TestDomainReposInUseCoversEveryItemsRepository(t *testing.T) {
	dir := t.TempDir()
	st := newTestStore(t)
	svc := NewService(config.Config{
		AppKey:        strings.Repeat("a", 64),
		DataDir:       dir,
		HostMountRoot: dir,
	}, st, nil, nil, nil)

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	used, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RoleRepo, Name: "Cold", Repo: "backups/cold", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// A second one that NOTHING points at: it must not be scanned, or every
	// overview pays for repositories nobody uses.
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RoleRepo, Name: "Unused", Repo: "backups/unused", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.WritePlacement(store.ItemRef{Domain: "containers", Key: "plex"}, &store.HomeWrite{Repo: used.ID, Choice: store.RepoChosen}, nil, nil); err != nil {
		t.Fatal(err)
	}

	repos, skipped, err := svc.domainReposInUse(settings, "containers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skipped) != 0 {
		t.Fatalf("skipped = %v, want nothing skipped: both repositories are on and resolve", skipped)
	}
	if len(repos) != 2 {
		t.Fatalf("repos = %v, want the domain repository and the one in use", repos)
	}
	if !strings.HasSuffix(repos[0].Loc, "backups/containers") {
		t.Fatalf("repos[0] = %q, want the domain repository first", repos[0].Loc)
	}
	if !strings.HasSuffix(repos[1].Loc, "backups/cold") {
		t.Fatalf("repos[1] = %q, want the repository the container points at", repos[1].Loc)
	}
}

// TestContainerViewCarriesTheRepoOnBothBranches pins a gap that cost a build.
//
// The container list assembles its view in TWO places: one for containers Docker
// reports, one for stored targets Docker no longer knows about. Only the second
// carried the new field, so the picker on a LIVE container always read back "the
// domain repository" no matter what was stored - the save landed, the interface
// said it had not, and clicking it was the only way to find out.
//
// A source scan because the two branches sit forty lines apart in one function
// and are edited for different reasons; a behavioural test would have to build a
// fake Docker to reach the first one.
func TestContainerViewCarriesTheRepoOnBothBranches(t *testing.T) {
	raw, err := os.ReadFile("handlers.go")
	if err != nil {
		t.Fatalf("read handlers.go: %v", err)
	}
	src := string(raw)
	// Matched with the run of spaces left OPEN. A struct literal's field values
	// are aligned by gofmt, so pinning the exact column would make this guard
	// fail the day somebody adds a longer field name beside it - a failure about
	// nothing, in a test whose whole job is to be believed when it speaks.
	for _, want := range []*regexp.Regexp{
		regexp.MustCompile(`v\.Repo\s*=\s*t\.Repo`), // the live-container merge
		regexp.MustCompile(`Repo:\s+t\.Repo,`),      // the not-installed literal
	} {
		if !want.MatchString(src) {
			t.Errorf("the container view no longer carries the per-item repository on one of its two branches (%s).\n"+
				"The picker then reads back the domain repository for an item that is not on it, which is the\n"+
				"exact misreading the control exists to prevent.", want)
		}
	}
}
