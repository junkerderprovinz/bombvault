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

// The tests in this file drive the domain-wide operations against a domain that
// really has TWO repositories, and assert what the engine was actually called
// with.
//
// They exist because the guards written alongside the multi-repository rewrite
// are source scans: they assert that the loop CALLS domainReposForOp and
// primaryModeFor, and cannot see what the loop does with the answer. The exact
// regression the rewrite exists to prevent - resolve every repository and then
// use element [0], or hoist one mode out of the loop so every repository is
// addressed with the domain's credentials - contains every string those guards
// look for and passes all of them. A source scan is the right instrument for
// "which of two distant lines share a read"; it is the wrong one for "the loop
// body uses the loop variable", and that distinction was missed.

// twoRepoDomain builds a service whose Containers domain has its own repository
// plus a named repository (#204) that one container points at. Both exist on
// disk, so reposThatExist keeps them. It returns the service, the store, and the
// two resolved locations in the order the operations should reach them.
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
		// requireExistingRepo stats <repo>/config; a repository that is not there
		// is dropped, which would make these tests pass for the wrong reason.
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

// errWrongKey is restic's own sentence for the failure the Recovery wizard
// classifies on. Spelled out here rather than referenced from production code:
// the point of the test is that this exact message survives to the caller.
var errWrongKey = errors.New("wrong password or no key found")

func hasRepo(got []string, want string) bool {
	for _, g := range got {
		if filepath.Clean(g) == filepath.Clean(want) {
			return true
		}
	}
	return false
}

// TestCheckDomainChecksBothRepositoriesWithTheirOwnModes is the behavioural twin
// of the two source scans over CheckDomain: it asserts the loop reached BOTH
// repositories and that the named one was addressed with ITS OWN bandwidth cap,
// not the domain's.
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
	// The modes are parallel to the repos. The named repository carries an upload
	// cap of 4242; the domain's own carries none. One mode hoisted out of the loop
	// would make these two equal, which is the whole point of the assertion.
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

// TestUnlockDomainUnlocksBothRepositories pins the same for the unlock button,
// which exists precisely for the case where something is stuck.
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

// TestUnlockNamesTheSharedRepositoryItCouldOnlyClearStaleLocksOn pins the half
// of repoSkip.Note that had no channel.
//
// A repository shared with another domain gets `restic unlock` WITHOUT
// --remove-all, because forcing there would yank the lock out from under that
// domain's running backup. That is deliberate, permanent and correct, so it must
// not stamp the run red - but `restic unlock` removes only what restic itself
// calls stale, and a lock a previous container incarnation left is not stale
// until it is old enough. So the one case this button exists for is exactly the
// case where it can come back green having changed nothing, and the operator has
// to be told which repository that was.
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

// TestPruneDomainPrunesBothRepositories pins that the space retention freed on a
// named repository is actually reclaimed.
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

// TestASwitchedOffRepositoryMakesTheVerifyReportIncomplete is the finding the
// fourth review round led with: a named repository that is switched off (the
// ordinary reaction to a share dying) silently dropped out of every domain-wide
// operation, and the operation then reported success over the remainder.
//
// The verify must FAIL, naming the repository, rather than go green about data
// nobody opened.
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
	// …and it still verified what it COULD reach. Refusing outright would leave
	// the reachable half unverified, which is worse than an honest partial pass.
	if !hasRepo(eng.checked, own) {
		t.Errorf("checked = %v, want the reachable repository still verified", eng.checked)
	}
}

// TestDeleteSnapshotRefusesAnIdThatMatchesInTwoRepositories pins the ambiguity
// guard. A short id is eight hex characters and both repositories were written
// by the same BombVault, so a collision is likelier here than restic's own
// within-one-repository odds - and the cost of guessing is deleting the wrong
// backup.
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

// TestASharedNamedRepositoryIsCopiedNarrowedToThisDomain is the behavioural
// proof the off-site half never had.
//
// A named repository can be shared: nothing scopes one to a single domain and
// the same picker offers it to containers, VMs and folder sets alike. Copying
// it WHOLE into one domain's off-site destination carries the other domain's
// snapshots along, and applyRetentionPerIdentity at the destination then ages
// them under this domain's keep-policy - a foreign policy deleting backups
// whose own destination is untouched.
//
// The decision used to be the source's INDEX in a slice both callers reshape.
// It is now the reference's own identity, and this test reads the snapshot ids
// the engine was actually handed, which is the only place the difference shows.
func TestASharedNamedRepositoryIsCopiedNarrowedToThisDomain(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "1111aaaa", Tags: []string{"container:plex"}},
		{ID: "2222bbbb", Tags: []string{"vm:win11"}},
	}}
	svc, st, own, cold := twoRepoDomain(t, eng)
	// The DESTINATION is empty, so every snapshot is still pending. Without this
	// the fake answers the destination with the source's own list and the copy
	// correctly finds nothing to do - proving the fake, not the code.
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
		// eng.copied is "<src>-><dest>"; the locations are resolved under a
		// forward-slash mount root, so both sides are normalised before comparing.
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

// TestAnAppendOnlyLocalNamedRepositoryIsNotPruned pins the guarantee the
// interface prints next to the toggle.
//
// Every gate read the flag through primaryIsImmutable, which returns false for
// any local path before it ever consults a named row - so for "backups/cold" on
// a share, the first shape the hint advertises, the switch was on screen, the
// tooltip promised nothing on this box may delete from it, and prune repacked
// it anyway.
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

// TestDeleteSnapshotHitsTheRepositoryHoldingIt is the instrument the delete path
// never had.
//
// The fake used to discard Forget's repo argument, so rewriting DeleteSnapshot
// to forget from the first repository of a two-repository domain left the whole
// suite green: every assertion around repoHoldingSnapshot was an assertion about
// the fake. This reads the repository the engine was actually handed.
func TestDeleteSnapshotHitsTheRepositoryHoldingIt(t *testing.T) {
	eng := &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, _, own, cold := twoRepoDomain(t, eng)
	// The snapshot exists ONLY in the named repository, so a delete that goes to
	// the domain's own is unambiguously wrong.
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
	// …and with the named repository's own mode, not the domain's.
	if eng.forgotModes[0].Limits.UploadKBps != 4242 {
		t.Errorf("the delete was addressed with upload cap %d, want the named repository's own 4242",
			eng.forgotModes[0].Limits.UploadKBps)
	}
}

// TestAnAppendOnlyLocalNamedRepositoryRefusesASnapshotDelete covers the other
// five gates that read the append-only flag.
//
// TestAnAppendOnlyLocalNamedRepositoryIsNotPruned exercised PruneDomain, which
// was the ONE gate routed through the reference. The other five asked
// primaryIsImmutable, which returned false for any local path before consulting
// the named row, so the toggle protected a cloud archive and not the NAS share
// the hint advertises.
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

// TestDiscoverSearchesBothRepositories is the behavioural proof the disaster
// recovery path never had.
//
// discoverNamesAcrossRepos had no test at all: inserting `refs = refs[:1]`,
// which reverts discovery to the domain repository only, left the suite green.
// That is the pass an operator runs after losing /config, and the named
// repositories are exactly what it exists to find.
func TestDiscoverSearchesBothRepositories(t *testing.T) {
	eng := &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, _, own, cold := twoRepoDomain(t, eng)
	eng.snapsByRepo[filepath.ToSlash(own)] = []restic.Snapshot{{ID: "1111aaaa", Tags: []string{"container:sonarr"}}}
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "2222bbbb", Tags: []string{"container:plex"}}}

	// The definitions have to be where a real rebuild would find them: beside the
	// snapshots for the item on the named repository, in the domain mirror for the
	// other. Without them Discover counts nothing and the test would pass for the
	// wrong reason.
	writeDiscoverableDef(t, filepath.Join(own, "def"), "sonarr")
	writeDiscoverableDef(t, filepath.Join(cold, "def"), "plex")

	// probe=true: read-only, so nothing is written back to the store.
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

// writeDiscoverableDef puts an encrypted, decryptable definition where Discover
// reads one, so a discovery test counts what a real rebuild would rebuild.
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

// TestDiscoverFailsWhenTheDomainRepositoryCannotBeRead pins the asymmetry the
// Recovery wizard depends on.
//
// A NAMED repository that cannot be listed is a skip; the domain's OWN is an
// error, because that error is what the wizard classifies on to tell a wrong
// APP_KEY from an empty archive. Folding it into the skip list replaced a red
// remedy panel with a silent "0 found" on the one screen an operator reaches on
// their worst day.
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

// TestAPartialDiscoverRebuildsTheRowButNotItsRepository is the test the one
// above cannot be: it runs dryRun=FALSE, so it reaches the write block at all.
//
// Returning the partial name map alongside the read error let a rebuild get as
// far as it could instead of returning nothing, which is right. What came with
// it is that the write loop then decided an item's REPOSITORY from evidence the
// pass itself knows is incomplete: the named repositories are searched first and
// the domain's own last, so when the domain's own is the one that failed, the
// newest-wins comparison was decided without ever opening the repository most
// items are actually in.
//
// The row is the recoverable half and must still be written. The repository
// column is the half that latches: a later Discover sees it non-empty and leaves
// it alone, the PATCH route refuses to clear it for an item with backups, and no
// route deletes it - so a wrong attribution here is permanent, and the next
// scheduled backup follows it into the wrong archive.
func TestAPartialDiscoverRebuildsTheRowButNotItsRepository(t *testing.T) {
	eng := &fakeResticEngine{
		snapsByRepo: map[string][]restic.Snapshot{},
		snapsErrFor: map[string]error{},
	}
	svc, st, own, cold := twoRepoDomain(t, eng)
	// sonarr has no row yet, so this pass creates one - the shape where the
	// has-backups refusal is deliberately not asked and nothing else stands in
	// the way of the attribution.
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
	_ = own
}

// TestAnUnmountedDomainRepositoryAlsoWithholdsTheAttribution closes the other
// half of the same defect.
//
// The gate keys on readErr, and discoverNamesAcrossRepos has TWO exits that
// leave a repository unread. Only the listing failure produced an error; the
// localRepoMissing branch fires earlier, records a skip and continues, so an
// UNMOUNTED share - which is the shape a post-/config-loss Discover is most
// likely to meet - reached the write loop with err == nil and re-homed items on
// named-repository evidence alone.
func TestAnUnmountedDomainRepositoryAlsoWithholdsTheAttribution(t *testing.T) {
	eng := &fakeResticEngine{
		snapsByRepo: map[string][]restic.Snapshot{},
		snapsErrFor: map[string]error{},
	}
	svc, st, own, cold := twoRepoDomain(t, eng)
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "2222bbbb", Tags: []string{"container:sonarr"}}}
	writeDiscoverableDef(t, filepath.Join(cold, "def"), "sonarr")
	// The domain repository was a working repository and its share is gone now.
	// Nothing errors: localRepoMissing fires before the listing ever happens.
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

// TestAPartialDiscoverOfVMsWithholdsTheAttributionToo is the gap a source scan
// could not see.
//
// The container test above and the file-set one elsewhere both exercise their
// own entry point; DiscoverVMs had the gate in the code and nothing that ran it.
// A scan that only checks a function call is present would stay green if the
// condition were inverted, or if the gate were deleted while its comment stayed.
func TestAPartialDiscoverOfVMsWithholdsTheAttributionToo(t *testing.T) {
	eng := &fakeResticEngine{
		snapsByRepo: map[string][]restic.Snapshot{},
		snapsErrFor: map[string]error{},
	}
	svc, st, own, cold := twoRepoDomain(t, eng)
	// The VMs domain shares the folder the containers domain uses here, so the
	// same two repositories serve both and the fixture stays small.
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

// TestAnAllNamedDomainStillDiscoversWithoutItsOwnRepository is the other side of
// that gate, and the reason it is not simply "the domain repository is missing,
// so fail".
//
// The install issue #204 exists for never creates the domain repository at all:
// every backup only ever creates the item's own location. Ending the pass on
// that would break exactly the configuration the feature added - the mistake the
// sixth round already paid for once in offsiteReplicationSources.
func TestAnAllNamedDomainStillDiscoversWithoutItsOwnRepository(t *testing.T) {
	eng := &fakeResticEngine{
		snapsByRepo: map[string][]restic.Snapshot{},
		snapsErrFor: map[string]error{},
	}
	svc, st, own, cold := twoRepoDomain(t, eng)
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "2222bbbb", Tags: []string{"container:sonarr"}}}
	writeDiscoverableDef(t, filepath.Join(cold, "def"), "sonarr")
	// Never created, and no established marker - the ordinary all-named shape.
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
	// …and it is still SAID, because after a /config loss this branch cannot tell
	// "never created" from "on a share that is not mounted".
	if len(skipped) == 0 {
		t.Error("the missing domain repository was not named at all; the wizard has to show it\n" +
			"before anybody trusts a result assembled without it")
	}
}

// TestANamedRepositoryAtTheHeadOfTheListIsStillNarrowed is the discriminator the
// test above cannot be.
//
// In that fixture the domain's own repository is present and FIRST, so
// reinstating the old positional rule ("index 0 is the domain's own, everything
// after it is named") leaves the suite green: the named repository is at index 1
// either way. Here the domain's own repository does not exist at all - the
// ordinary shape once every item is pointed somewhere else - so the named
// repository IS the head of the list, and a rule that reads identity off a
// position copies it whole.
func TestANamedRepositoryAtTheHeadOfTheListIsStillNarrowed(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "1111aaaa", Tags: []string{"container:plex"}},
		{ID: "2222bbbb", Tags: []string{"vm:win11"}},
	}}
	svc, st, own, cold := twoRepoDomain(t, eng)
	// No domain repository on disk: every item of this domain lives on the named
	// one, which is what issue #204 asked for.
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
	_ = cold
}

// TestACopyAlreadyAtTheDestinationIsNotSentAgain pins the identity rule restic
// actually uses.
//
// A copy lands at the destination under a NEW id and records the source id in
// Original. Comparing the destination's own ids against the source's own ids
// therefore matches nothing, ever: the narrowing was inert, and every historical
// id of the domain went onto restic's command line on every pass. No test could
// see it, because every fixture left the destination empty.
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
	// The destination already holds a copy of 1111aaaa. Its own id is a different
	// one, exactly as restic writes it; the link back is Original.
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

// TestAPartialCopyDoesNotAgeTheDestination is the behavioural test the off-site
// merge blocker never had.
//
// The gate used to ask "did EVERY source fail?" and now asks "did anything
// land?". Those two agree in every fixture the package could previously build,
// because the fake carried one global copy error: with it set every copy failed,
// without it none did. The shape that separates them is two sources where one
// fails, and it is the shape that costs data - a forget plus prune over a
// destination that did not receive what this pass was meant to bring it.
func TestAPartialCopyDoesNotAgeTheDestination(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "1111aaaa", Tags: []string{"container:plex"}},
	}}
	svc, st, own, cold := twoRepoDomain(t, eng)
	dest := "rest:http://192.168.1.2:8000/containers"
	eng.snapsByRepo = map[string][]restic.Snapshot{dest: nil}
	// The NAMED source fails; the domain's own succeeds. Under the old gate that
	// read as "one error, two sources, so a partial pass", and the retention ran.
	eng.copyErrFor = map[string]error{filepath.ToSlash(cold): errors.New("the share went away mid-copy")}

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersOffsite = dest
	s.OffsiteRetentionKeepLast = 3 // a policy, so the retention would have something to do
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	// A failing source makes the whole pass fail, which is correct and expected.
	if err := svc.ReplicateOffsite(context.Background(), "containers"); err == nil {
		t.Fatal("a source that could not be copied must make the pass report failure")
	}
	// …and the destination must NOT have been aged: what did land is not what
	// this pass was supposed to bring.
	if len(eng.prunedRepos) != 0 {
		t.Errorf("the destination was aged after a partial copy (%v).\n"+
			"A forget plus prune here deletes history against a replica that is missing\n"+
			"exactly the snapshots the failed source was carrying.", eng.prunedRepos)
	}
	_ = own
}

// TestASourceThatNeverReachedTheCopyDoesNotAgeTheDestination covers the shape
// the gate above cannot see.
//
// Moving the gate from "did anything land" to "did the pass complete without
// error" was right in the direction that mattered, and it removed the last
// `copied` term with it. A source that was dropped BEFORE the copy loop - it was
// a working repository once and is not reachable now - never becomes a copy
// error, so the remaining sources report a clean pass and the destination is
// aged under the off-site keep-policy while it is missing exactly what the
// dropped source was carrying. That destination may by then be the only copy of
// those items left.
func TestASourceThatNeverReachedTheCopyDoesNotAgeTheDestination(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "1111aaaa", Tags: []string{"container:plex"}},
	}}
	svc, st, own, cold := twoRepoDomain(t, eng)
	dest := "rest:http://192.168.1.2:8000/containers"
	eng.snapsByRepo = map[string][]restic.Snapshot{dest: nil}
	// cold was a working repository and is gone now: offsiteReplicationSources
	// puts that in the SKIP list, not in the source list, so the copy loop never
	// hears about it.
	// The marker is keyed by the RESOLVED location, which resolveRepo builds from
	// the slash-spelled mount root - not by filepath.Join's native separators.
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
	s.OffsiteRetentionKeepLast = 3 // a policy, so the retention would have something to do
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
	_ = own
}

// TestAppendOnlyTurnedOnMidDeleteIsHonoured closes the race the round-nine audit
// named and left below its floor.
//
// DeleteBackups asks the protection question twice: once before the domain lock,
// to answer "is there anything here to protect", and once inside it, to answer
// "may this delete happen". The second one used to REPLAY the first answer - so
// an operator who turns append-only on while their own delete is in flight, which
// is exactly the moment they most mean it, still lost the archive. A flag that is
// honoured only if it was already set when the button was pressed is not a
// protection.
func TestAppendOnlyTurnedOnMidDeleteIsHonoured(t *testing.T) {
	eng := &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, st, own, cold := twoRepoDomain(t, eng)
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "abcd1234deadbeef", Tags: []string{"container:plex"}}}
	eng.snapsByRepo[filepath.ToSlash(own)] = nil

	// The repository is NOT protected when the delete starts, so the pre-lock
	// check waves it through. Between that listing and the one inside the lock,
	// the operator switches Append-only on.
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

// TestTheCopyIsOpenedWithTheDestinationsMode makes copiedModes an instrument
// instead of a field nothing reads.
//
// `restic copy` spends its ONE set of backend credentials on the DESTINATION;
// only the repository password has a --from- counterpart. So the mode has to be
// the destination's, whatever the source is - and the named source here carries
// a storage class of its own, which is exactly what a mode built from the wrong
// end would carry into the call.
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

// TestADestinationThatIsAlsoASourceIsNeverAged pins the other half of the same
// blocker, which no fixture reached either.
func TestADestinationThatIsAlsoASourceIsNeverAged(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "1111aaaa", Tags: []string{"container:plex"}},
	}}
	svc, st, own, cold := twoRepoDomain(t, eng)

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	// The off-site destination IS the named repository one container backs up to.
	// Its snapshots therefore have no second copy anywhere.
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
	_ = own
	_ = cold
}

// TestARemotePrimaryIsStillReplicated is the guard for the defect the sixth
// round's own fix introduced.
//
// Excluding "any remote source" instead of "any remote NAMED source" emptied the
// list for a domain whose Backup Path is a restic remote, so every whole-domain
// replication returned an error without attempting a copy - for the documented
// configuration where that replication is the ONLY way to get a second copy.
func TestARemotePrimaryIsStillReplicated(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{{ID: "1111aaaa", Tags: []string{"container:plex"}}}}
	svc, st, _, _ := twoRepoDomain(t, eng)

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	// A remote primary replicated into a second bucket of the same account - the
	// shape docs/offsite-recovery.md describes, and the only way such an install
	// gets a second copy at all.
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

// TestARemotePrimaryWithItsOwnCredentialsIsStillAttempted is the discriminator
// the test above cannot be: there, both ends sit in one bucket of one account,
// so it cannot tell "the credentials match" from "the credentials are never
// asked about".
//
// The comment that used to justify keeping a remote own primary claimed the two
// ends share credentials BY CONSTRUCTION. They do not: primaryModeFor resolves
// the primary-remote row's own CredsRef (#182) and offsiteModeForTarget the
// destination row's, and those are independent store fields. What is true, and
// what this pins, is that such an install is ATTEMPTED and fails loudly rather
// than being silently left without a source - which is what the alternative
// was, and what cost round seven its single high finding.
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
	// The primary's own safety row names a credential set of its own - a different
	// account from the destination's, which is the configuration the "by
	// construction" claim said could not exist.
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

// TestEveryReaderAddressesItsOwnRepository is the instrument the mode conversion
// never had.
//
// Every reader on the fake used to name its restic.Mode parameter `_`, so "is
// each repository addressed with the mode built for IT" was invisible to the
// suite by construction. It stayed invisible long enough for the container
// restore to be missed by the same sweep that converted its VM twin: the two
// lines sit two thousand apart, do the same thing, and only one was changed.
func TestEveryReaderAddressesItsOwnRepository(t *testing.T) {
	eng := &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, _, own, cold := twoRepoDomain(t, eng)
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "abcd1234deadbeef", Tags: []string{"container:plex"}}}
	eng.snapsByRepo[filepath.ToSlash(own)] = nil

	// plex is pointed at the named repository, which carries an upload cap of
	// 4242. Every reader that opens it must carry that cap; the domain's own
	// carries none, so a shared mode is unmistakable.
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

	// …and the restore path, which is the one the sweep missed.
	eng.listedRepos, eng.listedModes = nil, nil
	if _, err := svc.StartRestore(context.Background(), "plex", "abcd1234deadbeef", "local", true); err != nil {
		// A restore can fail for reasons unrelated to the mode (no definition, no
		// docker). What matters is the mode of whatever it DID open.
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

// TestEveryAppendOnlyGateRefusesALocalNamedRepository closes the last of the
// instrument gaps the seventh round named.
//
// Six gates can destroy data, and the toggle promises in 42 languages that none
// of them may. Three of them - the bulk deletes - had no test at all, which is
// how five of the six came to short-circuit past the flag on a local path while
// the suite stayed green.
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

	// The bulk delete of the item that lives there.
	if err := svc.DeleteBackups(context.Background(), "plex", ""); err == nil {
		t.Error("DeleteBackups went ahead on an append-only repository")
	}
	// …and the retention that runs after every backup, which is the gate that
	// actually repacked one before the reorder.
	if len(eng.forgotRepos) != 0 {
		t.Errorf("something was forgotten from %v despite the append-only flag", eng.forgotRepos)
	}
	if len(eng.prunedRepos) != 0 {
		t.Errorf("something was pruned in %v despite the append-only flag", eng.prunedRepos)
	}
}

// TestAnEmptyContainerRowCanStillBeCleared is the other half: the flag protects
// SNAPSHOTS, and with none there is nothing to protect.
//
// While an entry has backups, "Delete all backups" is the only removal its card
// offers, so refusing here left a container with no snapshots left at all stuck
// in the "not installed" list, with no way out but switching the whole
// repository's protection off - which drops it for every other item sharing
// that repository.
func TestAnEmptyContainerRowCanStillBeCleared(t *testing.T) {
	eng := &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, st, own, cold := twoRepoDomain(t, eng)
	// No snapshots anywhere.
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
