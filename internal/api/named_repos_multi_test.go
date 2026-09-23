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
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// These tests run the domain-wide operations against a domain with two
// repositories and check what the engine was called with. Source scans can
// only see that a loop calls domainReposForOp and primaryModeFor, not that the
// body uses the loop variable.

// twoRepoDomain builds a service whose containers domain has its own
// repository plus a named repository that one container points at. Both exist
// on disk. It returns the service, the store, and the two locations in the
// order the operations should reach them.
func twoRepoDomain(t *testing.T, eng *fakeResticEngine) (*api.Service, *store.Repo, string, string) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	own := filepath.Join(dir, "backups", "containers")
	cold := filepath.Join(dir, "backups", "cold")
	for _, p := range []string{own, cold} {
		if err := os.MkdirAll(p, 0o755); err != nil { //nolint:gosec // G301: test temp dir
			t.Fatal(err)
		}
		// requireExistingRepo drops a repository without a config file, which
		// would let these tests pass for the wrong reason.
		if err := os.WriteFile(filepath.Join(p, "config"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	named, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RoleRepo, Name: "Cold", Repo: "backups/cold", Enabled: true,
		LimitUpload: 4242, StorageClass: "DEEP_ARCHIVE",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.WritePlacement(store.ItemRef{Domain: "containers", Key: "plex"}, &store.HomeWrite{Repo: named.ID, Choice: store.RepoChosen}, nil, nil); err != nil {
		t.Fatal(err)
	}
	return api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng), st, own, cold
}

// errWrongKey is restic's message for a wrong key, which the Recovery wizard
// classifies on. It is spelled out here because the tests check that this
// exact message reaches the caller.
var errWrongKey = errors.New("wrong password or no key found")

func hasRepo(got []string, want string) bool {
	for _, g := range got {
		if filepath.Clean(g) == filepath.Clean(want) {
			return true
		}
	}
	return false
}

// The named repository must be checked with its own bandwidth cap, not the
// domain's.
func TestCheckDomainChecksBothRepositoriesWithTheirOwnModes(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, _, own, cold := twoRepoDomain(t, eng)

	if err := svc.CheckDomain(context.Background(), "containers", "local"); err != nil {
		t.Fatalf("CheckDomain: %v", err)
	}
	if len(eng.checked) != 2 {
		t.Fatalf("checked = %v, want both repositories of the domain", eng.checked)
	}
	if !hasRepo(eng.checked, own) || !hasRepo(eng.checked, cold) {
		t.Fatalf("checked = %v, want %q and %q", eng.checked, own, cold)
	}
	// The modes run parallel to the repos. Only the named repository has an
	// upload cap, so a single mode reused for both would show here.
	var ownMode, coldMode restic.Mode
	for i, r := range eng.checked {
		if filepath.Clean(r) == filepath.Clean(cold) {
			coldMode = eng.checkedModes[i]
		} else {
			ownMode = eng.checkedModes[i]
		}
	}
	if coldMode.Limits.UploadKBps != 4242 {
		t.Errorf("the named repository was checked with upload cap %d, want its own 4242.\n"+
			"Every repository must be addressed with the mode built for IT.", coldMode.Limits.UploadKBps)
	}
	if ownMode.Limits.UploadKBps != 0 {
		t.Errorf("the domain repository was checked with upload cap %d, want none: it got the named repository's mode", ownMode.Limits.UploadKBps)
	}
}

func TestUnlockDomainUnlocksBothRepositories(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, _, own, cold := twoRepoDomain(t, eng)

	if _, err := svc.UnlockDomain(context.Background(), "containers", "local"); err != nil {
		t.Fatalf("UnlockDomain: %v", err)
	}
	if !hasRepo(eng.unlockedRepos, own) || !hasRepo(eng.unlockedRepos, cold) {
		t.Fatalf("unlocked = %v, want both %q and %q", eng.unlockedRepos, own, cold)
	}
}

// A repository shared with another domain is unlocked without --remove-all, so
// the other domain's running backup keeps its lock. That clears only locks
// restic considers stale, and a fresh lock left by a previous container stays.
// The run is not a failure, but the operator has to learn which repository
// may still be locked.
func TestUnlockNamesTheSharedRepositoryItCouldOnlyClearStaleLocksOn(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, st, _, _ := twoRepoDomain(t, eng)
	// A VM points at the same named repository, so it is shared.
	named, err := st.ListNamedRepos()
	if err != nil || len(named) != 1 {
		t.Fatalf("ListNamedRepos = %v, %v", named, err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.WritePlacement(store.ItemRef{Domain: "vms", Key: "win11"}, &store.HomeWrite{Repo: named[0].ID, Choice: store.RepoChosen}, nil, nil); err != nil {
		t.Fatal(err)
	}

	skipped, err := svc.UnlockDomain(context.Background(), "containers", "local")
	if err != nil {
		t.Fatalf("a shared repository is not a failure - it must not become the error: %v", err)
	}
	if len(skipped) == 0 {
		t.Fatal("the shared repository got only the stale-lock clear and nothing said so.\n" +
			"The Note is reported by the caller, so a skip list that never leaves this\n" +
			"function reaches nobody but the container log - and the button returns green.")
	}
	if !strings.Contains(strings.Join(skipped, " "), "Cold") {
		t.Errorf("the report must name the repository, got %v", skipped)
	}
}

func TestPruneDomainPrunesBothRepositories(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, _, own, cold := twoRepoDomain(t, eng)

	if err := svc.PruneDomain(context.Background(), "containers", "local"); err != nil {
		t.Fatalf("PruneDomain: %v", err)
	}
	if !hasRepo(eng.manualPruned, own) || !hasRepo(eng.manualPruned, cold) {
		t.Fatalf("pruned = %v, want both %q and %q", eng.manualPruned, own, cold)
	}
}

// Switching a named repository off is the usual reaction to a dead share. The
// verify must then fail and name it, rather than report success over the
// repositories it could reach.
func TestASwitchedOffRepositoryMakesTheVerifyReportIncomplete(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, st, own, _ := twoRepoDomain(t, eng)

	rows, err := st.ListNamedRepos()
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListNamedRepos: %v (%d rows)", err, len(rows))
	}
	off := rows[0]
	off.Enabled = false
	if _, err := st.UpsertOffsiteTarget(off); err != nil {
		t.Fatal(err)
	}

	err = svc.CheckDomain(context.Background(), "containers", "local")
	if err == nil {
		t.Fatal("a verify that could not reach a repository of this domain must NOT report success:\n" +
			"the run row, the dashboard card and the operator all read that as 'the data is fine'.")
	}
	if !strings.Contains(err.Error(), "Cold") {
		t.Errorf("the error must name the repository it could not cover, got %q", err.Error())
	}
	// The reachable repository is still verified.
	if !hasRepo(eng.checked, own) {
		t.Errorf("checked = %v, want the reachable repository still verified", eng.checked)
	}
}

// A short id is eight hex characters across two repositories, and guessing
// would delete the wrong backup.
func TestDeleteSnapshotRefusesAnIdThatMatchesInTwoRepositories(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{{ID: "abcd1234deadbeef"}}}
	svc, _, _, _ := twoRepoDomain(t, eng)

	err := svc.DeleteSnapshot(context.Background(), "containers", "abcd1234", "local")
	if err == nil {
		t.Fatal("an id that matches a snapshot in BOTH repositories must be refused, not guessed")
	}
	if !strings.Contains(err.Error(), "more than one repository") {
		t.Errorf("the refusal must say why, got %q", err.Error())
	}
	if len(eng.forgotten) != 0 {
		t.Errorf("nothing may be deleted while the id is ambiguous, forgot %v", eng.forgotten)
	}
}

// Any domain can use a named repository. Copied whole into one domain's
// off-site target, it would carry the other domain's snapshots along, and that
// target's retention would then age them under a foreign keep policy.
func TestASharedNamedRepositoryIsCopiedNarrowedToThisDomain(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "1111aaaa", Tags: []string{"container:plex"}},
		{ID: "2222bbbb", Tags: []string{"vm:win11"}},
	}}
	svc, st, own, cold := twoRepoDomain(t, eng)
	// An empty destination leaves every snapshot pending. Otherwise the fake
	// answers with the source's list and the copy has nothing to do.
	eng.snapsByRepo = map[string][]restic.Snapshot{
		"rest:http://192.168.1.2:8000/containers": nil,
	}

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersOffsite = "rest:http://192.168.1.2:8000/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	if err := svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatalf("ReplicateOffsite: %v", err)
	}
	if len(eng.copied) != 2 || len(eng.copiedIDs) != 2 {
		t.Fatalf("copied = %v, want both repositories of the domain replicated", eng.copied)
	}
	for i, c := range eng.copied {
		// eng.copied holds "<src>-><dest>" under a forward-slash mount root.
		src := filepath.ToSlash(strings.SplitN(c, "->", 2)[0])
		switch {
		case src == filepath.ToSlash(own):
			if eng.copiedIDs[i] != nil {
				t.Errorf("the domain's own repository was narrowed to %v; everything in it belongs to this domain,\n"+
					"so it must be copied whole and let restic's own dedup decide what moves.", eng.copiedIDs[i])
			}
		case src == filepath.ToSlash(cold):
			got := eng.copiedIDs[i]
			if len(got) != 1 || got[0] != "1111aaaa" {
				t.Errorf("the named repository was copied with ids %v, want exactly the container snapshot.\n"+
					"Copying it whole drags the vm: snapshot into the containers destination, where this\n"+
					"domain's retention then ages a backup that belongs to another domain.", got)
			}
		default:
			t.Errorf("unexpected copy %q", c)
		}
	}
}

// The append-only toggle promises that nothing on this box deletes from the
// repository, and that holds for a local path such as a share too.
func TestAnAppendOnlyLocalNamedRepositoryIsNotPruned(t *testing.T) {
	eng := &fakeResticEngine{}
	svc, st, own, cold := twoRepoDomain(t, eng)

	rows, err := st.ListNamedRepos()
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListNamedRepos: %v (%d rows)", err, len(rows))
	}
	row := rows[0]
	row.Immutable = true
	if _, err := st.UpsertOffsiteTarget(row); err != nil {
		t.Fatal(err)
	}

	if err := svc.PruneDomain(context.Background(), "containers", "local"); err != nil {
		t.Fatalf("PruneDomain: %v", err)
	}
	if !hasRepo(eng.manualPruned, own) {
		t.Errorf("pruned = %v, want the domain's own repository still pruned", eng.manualPruned)
	}
	if hasRepo(eng.manualPruned, cold) {
		t.Errorf("the append-only repository was pruned (%v).\n"+
			"The toggle states that nothing on this box may delete from it; a switch that is\n"+
			"read by nothing is worse than no switch, because the screen promises a guarantee.", eng.manualPruned)
	}
}

func TestDeleteSnapshotHitsTheRepositoryHoldingIt(t *testing.T) {
	eng := &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, _, own, cold := twoRepoDomain(t, eng)
	// The snapshot exists only in the named repository.
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "abcd1234deadbeef", Tags: []string{"container:plex"}}}
	eng.snapsByRepo[filepath.ToSlash(own)] = nil

	if err := svc.DeleteSnapshot(context.Background(), "containers", "abcd1234", "local"); err != nil {
		t.Fatalf("DeleteSnapshot: %v", err)
	}
	if len(eng.forgotRepos) != 1 {
		t.Fatalf("forgot from %v, want exactly one repository", eng.forgotRepos)
	}
	if filepath.Clean(eng.forgotRepos[0]) != filepath.Clean(cold) {
		t.Errorf("the snapshot was forgotten from %q, want %q - the repository that actually holds it.\n"+
			"Deleting from the wrong repository of a domain either fails or removes a different backup.",
			eng.forgotRepos[0], cold)
	}
	if eng.forgotModes[0].Limits.UploadKBps != 4242 {
		t.Errorf("the delete was addressed with upload cap %d, want the named repository's own 4242",
			eng.forgotModes[0].Limits.UploadKBps)
	}
}

// A snapshot delete honours the append-only flag of a local named repository
// just as prune does.
func TestAnAppendOnlyLocalNamedRepositoryRefusesASnapshotDelete(t *testing.T) {
	eng := &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, st, own, cold := twoRepoDomain(t, eng)
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "abcd1234deadbeef", Tags: []string{"container:plex"}}}
	eng.snapsByRepo[filepath.ToSlash(own)] = nil

	rows, err := st.ListNamedRepos()
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListNamedRepos: %v (%d rows)", err, len(rows))
	}
	row := rows[0]
	row.Immutable = true
	if _, err := st.UpsertOffsiteTarget(row); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteSnapshot(context.Background(), "containers", "abcd1234", "local"); err == nil {
		t.Fatal("a snapshot delete against an append-only repository must be refused:\n" +
			"the toggle promises, in 42 languages, that nothing on this box may delete from it.")
	}
	if len(eng.forgotRepos) != 0 {
		t.Errorf("something was forgotten anyway, from %v", eng.forgotRepos)
	}
}

// Discover is what an operator runs after losing /config, and the named
// repositories are what it has to find.
func TestDiscoverSearchesBothRepositories(t *testing.T) {
	eng := &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, _, own, cold := twoRepoDomain(t, eng)
	eng.snapsByRepo[filepath.ToSlash(own)] = []restic.Snapshot{{ID: "1111aaaa", Tags: []string{"container:sonarr"}}}
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "2222bbbb", Tags: []string{"container:plex"}}}

	// Without definitions where a real rebuild finds them, Discover counts
	// nothing and the test would pass for the wrong reason.
	writeDiscoverableDef(t, filepath.Join(own, "def"), "sonarr")
	writeDiscoverableDef(t, filepath.Join(cold, "def"), "plex")

	// A probe is read-only.
	res, err := svc.Discover(context.Background(), true)
	n, skipped := res.Found, res.Skipped
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("both repositories are present and enabled, yet the pass reported skips: %v", skipped)
	}
	if !hasRepo(eng.listedRepos, cold) {
		t.Errorf("the pass listed %v and never looked in the named repository %q. "+
			"This is the pass an operator runs after losing /config; an item whose backups are "+
			"in there stays unrecoverable while its snapshots sit intact.", eng.listedRepos, cold)
	}
	if !hasRepo(eng.listedRepos, own) {
		t.Errorf("the pass listed %v and never looked in the domain repository %q", eng.listedRepos, own)
	}
	if n != 2 {
		t.Errorf("Discover found %d items, want 2 - one from each repository.", n)
	}
}

// writeDiscoverableDef writes an encrypted definition for name into dir.
func writeDiscoverableDef(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: test temp dir
		t.Fatal(err)
	}
	enc, err := secret.Encrypt(strings.Repeat("a", 64), []byte(`{"appdataPaths":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".def"), enc, 0o600); err != nil {
		t.Fatal(err)
	}
}

// A named repository that cannot be listed is a skip, but the domain's own is
// an error: the Recovery wizard classifies on it to tell a wrong APP_KEY from
// an empty archive.
func TestDiscoverFailsWhenTheDomainRepositoryCannotBeRead(t *testing.T) {
	eng := &fakeResticEngine{
		snapsByRepo: map[string][]restic.Snapshot{},
		snapsErrFor: map[string]error{},
	}
	svc, _, own, cold := twoRepoDomain(t, eng)
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "2222bbbb", Tags: []string{"container:plex"}}}
	eng.snapsErrFor[filepath.ToSlash(own)] = errWrongKey

	_, err := svc.Discover(context.Background(), true)
	if err == nil {
		t.Fatal("a domain repository that cannot be opened must reach the caller as an ERROR:\n" +
			"the Recovery wizard reads it to tell a wrong APP_KEY from an empty archive, and a\n" +
			"skip nobody declares turns that into a silent '0 found'.")
	}
	if !strings.Contains(err.Error(), "wrong password") {
		t.Errorf("the real reason must survive, got %q", err.Error())
	}
}

// When the domain's own repository cannot be read, a real (not dry-run)
// Discover still rebuilds the row but leaves its repository empty. Only the
// named repositories were searched, and the column latches: a later Discover
// leaves a set value alone, the PATCH route will not clear it for an item with
// backups, and the next backup follows it into the wrong archive.
func TestAPartialDiscoverRebuildsTheRowButNotItsRepository(t *testing.T) {
	eng := &fakeResticEngine{
		snapsByRepo: map[string][]restic.Snapshot{},
		snapsErrFor: map[string]error{},
	}
	svc, st, own, cold := twoRepoDomain(t, eng)
	// sonarr has no row yet, so the has-backups refusal is not asked and
	// nothing else stands in the way of the attribution.
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "2222bbbb", Tags: []string{"container:sonarr"}}}
	eng.snapsErrFor[filepath.ToSlash(own)] = errWrongKey
	writeDiscoverableDef(t, filepath.Join(cold, "def"), "sonarr")

	res, err := svc.Discover(context.Background(), false)
	n := res.Found
	if err == nil {
		t.Fatal("the domain repository failed to open, so the pass must still report that")
	}
	if n != 1 {
		t.Fatalf("discovered = %d, want 1: the partial rebuild is the point of carrying the error alongside the result", n)
	}
	tg, gErr := st.GetTargetByContainer("sonarr")
	if gErr != nil {
		t.Fatalf("the ROW must still be rebuilt from a partial pass - it is idempotent and recoverable: %v", gErr)
	}
	if strings.TrimSpace(tg.Repo) != "" {
		t.Errorf("the pass attributed sonarr to %q while the domain's own repository could not be opened.\n"+
			"Only the named repositories were searched, so 'the newest snapshot is in Cold' was decided\n"+
			"without looking at the repository the item is probably in - and this column latches:\n"+
			"a later Discover leaves a non-empty column alone and the PATCH route refuses to clear it\n"+
			"for an item with backups, so the item is re-homed for good and its next backup follows.", tg.Repo)
	}
}

// An unmounted share, the likeliest state after losing /config, leaves the
// domain repository unread without a listing error. It must withhold the
// attribution just like a failed listing.
func TestAnUnmountedDomainRepositoryAlsoWithholdsTheAttribution(t *testing.T) {
	eng := &fakeResticEngine{
		snapsByRepo: map[string][]restic.Snapshot{},
		snapsErrFor: map[string]error{},
	}
	svc, st, own, cold := twoRepoDomain(t, eng)
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "2222bbbb", Tags: []string{"container:sonarr"}}}
	writeDiscoverableDef(t, filepath.Join(cold, "def"), "sonarr")
	// The domain repository was established and its share is gone, so
	// localRepoMissing fires before any listing.
	if err := st.MarkRepoEstablished(filepath.ToSlash(own)); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(own); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Discover(context.Background(), false)
	n := res.Found
	if err == nil {
		t.Fatal("a domain repository that was there and is not reachable now must reach the caller as an ERROR.\n" +
			"It is the same fact as a listing that failed - the repository was not opened - and the\n" +
			"attribution gate keys on exactly that.")
	}
	if n != 1 {
		t.Fatalf("discovered = %d, want 1: the partial rebuild is still the point", n)
	}
	tg, gErr := st.GetTargetByContainer("sonarr")
	if gErr != nil {
		t.Fatalf("the row must still be rebuilt: %v", gErr)
	}
	if strings.TrimSpace(tg.Repo) != "" {
		t.Errorf("sonarr was attributed to %q while the domain repository sat on an unmounted share.\n"+
			"Only the named repositories were searched, the column latches, and the next backup\n"+
			"follows it into the wrong archive.", tg.Repo)
	}
}

func TestAPartialDiscoverOfVMsWithholdsTheAttributionToo(t *testing.T) {
	eng := &fakeResticEngine{
		snapsByRepo: map[string][]restic.Snapshot{},
		snapsErrFor: map[string]error{},
	}
	svc, st, own, cold := twoRepoDomain(t, eng)
	// The VMs domain reuses the containers folder, so the same two repositories
	// serve both.
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.VMsPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "4444dddd", Tags: []string{"vm:win11"}}}
	eng.snapsErrFor[filepath.ToSlash(own)] = errWrongKey
	writeDiscoverableDef(t, filepath.Join(cold, "vm-def"), "win11")

	res, dErr := svc.DiscoverVMs(context.Background(), false)
	n := res.Found
	if dErr == nil {
		t.Fatal("the domain repository failed to open, so the pass must still report that")
	}
	if n != 1 {
		t.Fatalf("discovered = %d, want 1: the partial rebuild is the point", n)
	}
	vm, gErr := st.GetVMTargetByName("win11")
	if gErr != nil {
		t.Fatalf("the ROW must still be rebuilt from a partial pass: %v", gErr)
	}
	if strings.TrimSpace(vm.Repo) != "" {
		t.Errorf("the pass attributed win11 to %q while the domain's own repository could not be opened.\n"+
			"Same latch as the container path: a later Discover leaves a non-empty column alone and\n"+
			"the PATCH route refuses to clear it for an item with backups.", vm.Repo)
	}
}

// When every item uses a named repository, the domain repository is never
// created. That is not a failure, so the attribution goes ahead.
func TestAnAllNamedDomainStillDiscoversWithoutItsOwnRepository(t *testing.T) {
	eng := &fakeResticEngine{
		snapsByRepo: map[string][]restic.Snapshot{},
		snapsErrFor: map[string]error{},
	}
	svc, st, own, cold := twoRepoDomain(t, eng)
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "2222bbbb", Tags: []string{"container:sonarr"}}}
	writeDiscoverableDef(t, filepath.Join(cold, "def"), "sonarr")
	// Never created and never marked established.
	if err := os.RemoveAll(own); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Discover(context.Background(), false)
	n, skipped := res.Found, res.Skipped
	if err != nil {
		t.Fatalf("an all-named domain has no repository of its own; that is not a failure: %v", err)
	}
	if n != 1 {
		t.Fatalf("discovered = %d, want 1", n)
	}
	tg, gErr := st.GetTargetByContainer("sonarr")
	if gErr != nil {
		t.Fatalf("sonarr not rebuilt: %v", gErr)
	}
	if strings.TrimSpace(tg.Repo) == "" {
		t.Error("the attribution was withheld for an all-named domain.\n" +
			"There is no repository of the domain's own to have missed anything, so the named\n" +
			"evidence is complete - and an empty column sends the next backup somewhere else.")
	}
	// It is still reported, because after losing /config "never created" and
	// "on an unmounted share" look the same.
	if len(skipped) == 0 {
		t.Error("the missing domain repository was not named at all; the wizard has to show it\n" +
			"before anybody trusts a result assembled without it")
	}
}

// Without a domain repository the named one heads the source list, and it
// must still be narrowed to this domain's snapshots rather than judged by its
// position.
func TestANamedRepositoryAtTheHeadOfTheListIsStillNarrowed(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "1111aaaa", Tags: []string{"container:plex"}},
		{ID: "2222bbbb", Tags: []string{"vm:win11"}},
	}}
	svc, st, own, _ := twoRepoDomain(t, eng)
	if err := os.RemoveAll(own); err != nil {
		t.Fatal(err)
	}
	dest := "rest:http://192.168.1.2:8000/containers"
	eng.snapsByRepo = map[string][]restic.Snapshot{dest: nil}

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersOffsite = dest
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	if err := svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatalf("ReplicateOffsite: %v", err)
	}
	if len(eng.copiedIDs) != 1 {
		t.Fatalf("copied %d times, want exactly the one repository this domain has: %v", len(eng.copiedIDs), eng.copied)
	}
	got := eng.copiedIDs[0]
	if len(got) != 1 || got[0] != "1111aaaa" {
		t.Errorf("the named repository was copied with ids %v, want exactly the container snapshot.\n"+
			"It is first in the list here, and a rule that infers 'the domain's own' from position\n"+
			"therefore copies it whole - dragging another domain's snapshots into this domain's\n"+
			"off-site destination, where this domain's retention then ages them.", got)
	}
}

// A copy lands at the destination under a new id and keeps the source id in
// Original, so that is the field to compare.
func TestACopyAlreadyAtTheDestinationIsNotSentAgain(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "1111aaaa", Tags: []string{"container:plex"}},
		{ID: "3333cccc", Tags: []string{"container:sonarr"}},
	}}
	svc, st, own, _ := twoRepoDomain(t, eng)
	if err := os.RemoveAll(own); err != nil {
		t.Fatal(err)
	}
	dest := "rest:http://192.168.1.2:8000/containers"
	// The destination already holds a copy of 1111aaaa.
	eng.snapsByRepo = map[string][]restic.Snapshot{
		dest: {{ID: "9999ffff", Original: "1111aaaa", Tags: []string{"container:plex"}}},
	}

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersOffsite = dest
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	if err := svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatalf("ReplicateOffsite: %v", err)
	}
	if len(eng.copiedIDs) != 1 {
		t.Fatalf("copied %d times, want one: %v", len(eng.copiedIDs), eng.copied)
	}
	got := eng.copiedIDs[0]
	if len(got) != 1 || got[0] != "3333cccc" {
		t.Errorf("the copy was handed %v, want only the snapshot the destination does not have.\n"+
			"A destination snapshot carries the source id in Original, not in its own id, so a\n"+
			"comparison on ids alone never recognises anything and the whole history goes on argv.", got)
	}
}

// With two sources and one failing, the destination must not be aged: a
// forget and prune would run over a replica that is missing what the failed
// source carries.
func TestAPartialCopyDoesNotAgeTheDestination(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "1111aaaa", Tags: []string{"container:plex"}},
	}}
	svc, st, _, cold := twoRepoDomain(t, eng)
	dest := "rest:http://192.168.1.2:8000/containers"
	eng.snapsByRepo = map[string][]restic.Snapshot{dest: nil}
	// The named source fails; the domain's own succeeds.
	eng.copyErrFor = map[string]error{filepath.ToSlash(cold): errors.New("the share went away mid-copy")}

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersOffsite = dest
	s.OffsiteRetentionKeepLast = 3
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	if err := svc.ReplicateOffsite(context.Background(), "containers"); err == nil {
		t.Fatal("a source that could not be copied must make the pass report failure")
	}
	if len(eng.prunedRepos) != 0 {
		t.Errorf("the destination was aged after a partial copy (%v).\n"+
			"A forget plus prune here deletes history against a replica that is missing\n"+
			"exactly the snapshots the failed source was carrying.", eng.prunedRepos)
	}
}

// A source that was established once and is unreachable now is skipped before
// the copy loop, so it never becomes a copy error. The destination, which may
// by then hold the only copy of its items, must still not be aged.
func TestASourceThatNeverReachedTheCopyDoesNotAgeTheDestination(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "1111aaaa", Tags: []string{"container:plex"}},
	}}
	svc, st, _, cold := twoRepoDomain(t, eng)
	dest := "rest:http://192.168.1.2:8000/containers"
	eng.snapsByRepo = map[string][]restic.Snapshot{dest: nil}
	// The marker is keyed by the resolved location, which uses forward
	// slashes.
	if err := st.MarkRepoEstablished(filepath.ToSlash(cold)); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(cold); err != nil {
		t.Fatal(err)
	}

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersOffsite = dest
	s.OffsiteRetentionKeepLast = 3
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	if err := svc.ReplicateOffsite(context.Background(), "containers"); err == nil {
		t.Fatal("a repository this domain uses that could not be reached must make the pass report incomplete")
	}
	if len(eng.prunedRepos) != 0 {
		t.Errorf("the destination was aged (%v) after a pass one of whose sources was never opened.\n"+
			"The copy loop saw no error because the source never reached it, so 'the pass completed\n"+
			"without error' is true and still means nothing about whether the destination is current.", eng.prunedRepos)
	}
}

// DeleteBackups checks protection before taking the domain lock and again
// inside it. The second check must read the flag afresh, so switching
// append-only on while a delete is in flight still stops it.
func TestAppendOnlyTurnedOnMidDeleteIsHonoured(t *testing.T) {
	eng := &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, st, own, cold := twoRepoDomain(t, eng)
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "abcd1234deadbeef", Tags: []string{"container:plex"}}}
	eng.snapsByRepo[filepath.ToSlash(own)] = nil

	// Append-only goes on between the listing before the lock and the one
	// inside it.
	eng.onSnapshots = func(call int) {
		if call != 1 {
			return
		}
		rows, err := st.ListNamedRepos()
		if err != nil || len(rows) != 1 {
			t.Errorf("ListNamedRepos = %v, %v", rows, err)
			return
		}
		rows[0].Immutable = true
		if _, err := st.UpsertOffsiteTarget(rows[0]); err != nil {
			t.Errorf("flip Immutable: %v", err)
		}
	}

	err := svc.DeleteBackups(context.Background(), "plex", "")
	if err == nil {
		t.Fatal("the delete went ahead although Append-only was switched on while it ran.\n" +
			"The gate inside the domain lock replayed the answer from before the lock, so the\n" +
			"flag was read once, at the one moment it was still off.")
	}
	if len(eng.forgotten) != 0 {
		t.Errorf("something was forgotten (%v) despite the flag being on by then", eng.forgotten)
	}
}

// restic copy uses its one set of backend credentials for the destination;
// only the repository password has a --from- counterpart. The mode must
// therefore be the destination's, even when the source carries caps of its
// own.
func TestTheCopyIsOpenedWithTheDestinationsMode(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "1111aaaa", Tags: []string{"container:plex"}},
	}}
	svc, st, _, _ := twoRepoDomain(t, eng)
	dest := "rest:http://192.168.1.2:8000/containers"
	eng.snapsByRepo = map[string][]restic.Snapshot{dest: nil}

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersOffsite = dest
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if err := svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatalf("ReplicateOffsite: %v", err)
	}
	if len(eng.copiedModes) == 0 {
		t.Fatal("nothing was copied, so this test measured nothing")
	}
	for i, m := range eng.copiedModes {
		if m.Limits.UploadKBps == 4242 {
			t.Errorf("copy %d was opened with the SOURCE's own mode (its %d KB/s cap).\n"+
				"The named repository carries that cap, the destination carries none, and the\n"+
				"destination is the end restic authenticates. A copy built from the source's row\n"+
				"opens the wrong account and never applies the destination's own caps.", i, m.Limits.UploadKBps)
		}
	}
}

func TestADestinationThatIsAlsoASourceIsNeverAged(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "1111aaaa", Tags: []string{"container:plex"}},
	}}
	svc, st, _, _ := twoRepoDomain(t, eng)

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	// The off-site destination is the named repository a container backs up
	// to, so its snapshots have no second copy anywhere.
	s.ContainersOffsite = "backups/cold"
	s.OffsiteRetentionKeepLast = 3
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	_ = svc.ReplicateOffsite(context.Background(), "containers")
	if len(eng.prunedRepos) != 0 {
		t.Errorf("a destination that is itself one of the sources was aged (%v).\n"+
			"Whatever the other sources managed, a forget plus prune there deletes snapshots\n"+
			"whose only copy is the thing being pruned.", eng.prunedRepos)
	}
}

// Only remote named repositories are left out of replication. A remote
// backup path is still a source, because replication is the only way such an
// install gets a second copy.
func TestARemotePrimaryIsStillReplicated(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{{ID: "1111aaaa", Tags: []string{"container:plex"}}}}
	svc, st, _, _ := twoRepoDomain(t, eng)

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	// A remote primary replicated into a second bucket of the same account, as
	// docs/offsite-recovery.md describes.
	s.ContainersPath = "s3:https://s3.example/bucket/primary"
	s.ContainersOffsite = "s3:https://s3.example/bucket/offsite"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	if err := svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatalf("a domain with a remote primary must still replicate: %v", err)
	}
	found := false
	for _, c := range eng.copied {
		if strings.HasPrefix(c, "s3:https://s3.example/bucket/primary->") {
			found = true
		}
	}
	if !found {
		t.Errorf("the remote primary was not copied (copies: %v).\n"+
			"For flash and config, which can have no named repositories, it is the only\n"+
			"source there is, and excluding it failed the nightly pass without attempting\n"+
			"anything.", eng.copied)
	}
}

// The primary and the destination can use different credential sets, since
// primaryModeFor and offsiteModeForTarget read independent CredsRef fields.
// Such a copy is still attempted: failing visibly beats silently dropping the
// only source.
func TestARemotePrimaryWithItsOwnCredentialsIsStillAttempted(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{{ID: "1111aaaa", Tags: []string{"container:plex"}}}}
	svc, st, _, _ := twoRepoDomain(t, eng)

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersPath = "s3:https://s3.example/bucket/primary"
	s.ContainersOffsite = "s3:https://other.example/bucket/offsite"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	// The primary's row names a different account from the destination's.
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RolePrimary, Domain: "containers", Repo: s.ContainersPath,
		Enabled: true, CredsRef: "account-a",
	}); err != nil {
		t.Fatal(err)
	}

	_ = svc.ReplicateOffsite(context.Background(), "containers")
	found := false
	for _, c := range eng.copied {
		if strings.HasPrefix(c, "s3:https://s3.example/bucket/primary->") {
			found = true
		}
	}
	if !found {
		t.Errorf("the remote primary was dropped because it carries its own credential set (copies: %v).\n"+
			"A copy that cannot authenticate fails visibly every night and writes nothing;\n"+
			"a source silently left out reports a green run over a domain with no second copy.", eng.copied)
	}
}

// Listing snapshots and restoring must open an item's named repository with
// that repository's own mode.
func TestEveryReaderAddressesItsOwnRepository(t *testing.T) {
	eng := &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, _, own, cold := twoRepoDomain(t, eng)
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "abcd1234deadbeef", Tags: []string{"container:plex"}}}
	eng.snapsByRepo[filepath.ToSlash(own)] = nil

	// plex uses the named repository with its 4242 upload cap; the domain's own
	// has none, so a shared mode shows.
	if _, err := svc.Snapshots(context.Background(), "plex", "local"); err != nil {
		t.Fatalf("Snapshots: %v", err)
	}
	for i, r := range eng.listedRepos {
		if filepath.Clean(r) != filepath.Clean(cold) {
			continue
		}
		if eng.listedModes[i].Limits.UploadKBps != 4242 {
			t.Errorf("the named repository was listed with upload cap %d, want its own 4242.\n"+
				"A reader that builds the DOMAIN's mode after resolving the ITEM's repository\n"+
				"opens somebody else's bucket with the wrong credentials.", eng.listedModes[i].Limits.UploadKBps)
		}
	}

	eng.listedRepos, eng.listedModes = nil, nil
	if _, err := svc.StartRestore(context.Background(), "plex", "abcd1234deadbeef", "local", true); err != nil {
		// The restore may fail for other reasons, such as no definition or no
		// Docker. Only the mode of what it opened matters here.
		t.Logf("StartRestore returned %v (the mode below is what this test is about)", err)
	}
	for i, r := range eng.listedRepos {
		if filepath.Clean(r) != filepath.Clean(cold) {
			continue
		}
		if eng.listedModes[i].Limits.UploadKBps != 4242 {
			t.Errorf("the restore opened the named repository with upload cap %d, want its own 4242.\n"+
				"prepareRestore resolves the item's repository and then has to describe THAT one;\n"+
				"its VM twin was converted in the same sweep and this line was not.", eng.listedModes[i].Limits.UploadKBps)
		}
	}
}

// The bulk delete of an item on an append-only local named repository is
// refused, and nothing is forgotten or pruned.
func TestEveryAppendOnlyGateRefusesALocalNamedRepository(t *testing.T) {
	eng := &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, st, own, cold := twoRepoDomain(t, eng)
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "abcd1234deadbeef", Tags: []string{"container:plex"}}}
	eng.snapsByRepo[filepath.ToSlash(own)] = nil

	rows, err := st.ListNamedRepos()
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListNamedRepos: %v (%d rows)", err, len(rows))
	}
	row := rows[0]
	row.Immutable = true
	if _, err := st.UpsertOffsiteTarget(row); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteBackups(context.Background(), "plex", ""); err == nil {
		t.Error("DeleteBackups went ahead on an append-only repository")
	}
	if len(eng.forgotRepos) != 0 {
		t.Errorf("something was forgotten from %v despite the append-only flag", eng.forgotRepos)
	}
	if len(eng.prunedRepos) != 0 {
		t.Errorf("something was pruned in %v despite the append-only flag", eng.prunedRepos)
	}
}

// Append-only protects snapshots. A container without any must still be
// removable, or it stays in the "not installed" list until the whole
// repository loses its protection.
func TestAnEmptyContainerRowCanStillBeCleared(t *testing.T) {
	eng := &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, st, own, cold := twoRepoDomain(t, eng)
	eng.snapsByRepo[filepath.ToSlash(cold)] = nil
	eng.snapsByRepo[filepath.ToSlash(own)] = nil

	rows, err := st.ListNamedRepos()
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListNamedRepos: %v", err)
	}
	row := rows[0]
	row.Immutable = true
	if _, err := st.UpsertOffsiteTarget(row); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteBackups(context.Background(), "plex", ""); err != nil {
		t.Fatalf("a container with NO snapshots must still be removable from the list: %v", err)
	}
	if len(eng.forgotRepos) != 0 {
		t.Errorf("nothing should have been forgotten, got %v", eng.forgotRepos)
	}
}
