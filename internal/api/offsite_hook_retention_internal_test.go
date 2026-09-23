package api

// ---------------------------------------------------------------------------
// The post-backup off-site hook and the destination's retention.
//
// The hook copies exactly ONE repository: the one the item it just backed up
// wrote. The retention at the far end is tag-scoped PER IDENTITY over whatever
// the destination holds, so that single copy ages the off-site copies of every
// item in the domain. That is harmless while the other sources merely were not
// copied this pass - they still exist, so the destination is not the last copy
// of anything - and it is not harmless when one of them is gone.
//
// The whole-domain pass was taught that in the eighth round. Its sibling, this
// hook, was not, and on an install with no separate off-site schedule this hook
// IS the replication - so the defect stayed reachable by pressing "Back up now".
//
// Internal, because replicateOffsite is unexported and giving production code an
// exported test shim to reach it would be a worse trade than a fixture here.
// ---------------------------------------------------------------------------

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// hookFakeEngine records the two calls the destination's retention makes and
// leaves everything else inert.
type hookFakeEngine struct {
	ResticEngine
	copied []string
	forgot []string
	pruned []string
	// snapsByRepo lets the discovery tests below share this fixture: keyed by the
	// slash-spelled location, because resolveRepo builds its answers from the
	// slash-spelled mount root rather than from filepath.Join's separators.
	snapsByRepo map[string][]restic.Snapshot
}

func (f *hookFakeEngine) RepoOpens(context.Context, string, restic.Mode) bool { return true }

func (f *hookFakeEngine) RepoOpensErr(context.Context, string, restic.Mode) error { return nil }

func (f *hookFakeEngine) EnsureRepo(context.Context, string, restic.Mode) error { return nil }

func (f *hookFakeEngine) Unlock(context.Context, string, bool, restic.Mode) error { return nil }

func (f *hookFakeEngine) Snapshots(_ context.Context, repo string, _ restic.Mode) ([]restic.Snapshot, error) {
	return f.snapsByRepo[filepath.ToSlash(repo)], nil
}

func (f *hookFakeEngine) Copy(_ context.Context, dest, src string, _ []string, _ restic.Limits, _ restic.Mode) error {
	f.copied = append(f.copied, src+"->"+dest)
	return nil
}

func (f *hookFakeEngine) ForgetPolicy(_ context.Context, repo string, p restic.RetentionPolicy, _ restic.Mode, _ []string, _ bool) error {
	if p.Any() {
		f.forgot = append(f.forgot, repo)
	}
	return nil
}

func (f *hookFakeEngine) Prune(_ context.Context, repo string, _ restic.Mode) error {
	f.pruned = append(f.pruned, repo)
	return nil
}

func (f *hookFakeEngine) Stats(context.Context, string, string, restic.Mode) (restic.StatsResult, error) {
	return restic.StatsResult{}, nil
}

// hookSvc builds a containers domain with its own repository plus a named one an
// item points at, and an off-site destination with a keep-policy - the shape
// where the retention has something to do.
func hookSvc(t *testing.T, eng *hookFakeEngine) (*Service, *store.Repo, string, string) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)

	own := filepath.Join(dir, "backups", "containers")
	cold := filepath.Join(dir, "backups", "cold")
	for _, p := range []string{own, cold} {
		if err := os.MkdirAll(p, 0o755); err != nil { //nolint:gosec // G301: test temp dir
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "config"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersPath = "backups/containers"
	s.ContainersOffsite = "rest:http://192.168.1.2:8000/containers"
	s.OffsiteRetentionKeepLast = 3
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	named, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RoleRepo, Name: "Cold", Repo: "backups/cold", Enabled: true,
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

	svc := &Service{store: st, engine: eng, progress: progress.NewStore()}
	svc.cfg.HostMountRoot = root
	svc.cfg.AppKey = strings.Repeat("a", 64)
	svc.cfg.DataDir = dir
	return svc, st, own, cold
}

// TestTheHookAgesAnAllNamedDomainsDestination is the discriminator the healthy
// test above cannot be.
//
// There both repositories are present, so offsiteReplicationSources' "was this
// location ever a repository" switch is never entered at all - and its
// repoNeverEstablished arm, the one that has to stay SILENT, is exactly what the
// install issue #204 exists for hits on every pass: the domain's own repository
// is never created there. Turning that arm into a skip would stop the off-site
// retention of every all-named install for good, which is the sixth round's own
// mistake one level down.
func TestTheHookAgesAnAllNamedDomainsDestination(t *testing.T) {
	eng := &hookFakeEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, st, own, cold := hookSvc(t, eng)
	// The named repository has to hold something OF THIS DOMAIN, or the pass
	// correctly accounts for nothing and refuses the retention for that reason
	// instead of the one under test.
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "1111aaaa", Tags: []string{"container:plex"}}}
	// The domain's own repository was never created - every item lives on the
	// named one, and each backup only ever creates the location it writes to.
	if err := os.RemoveAll(own); err != nil {
		t.Fatal(err)
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	svc.replicateOffsite(context.Background(), "containers", settings, filepath.Join(filepath.Dir(own), "cold"), "container:plex")

	if len(eng.forgot) == 0 {
		t.Error("an all-named domain's off-site destination was never aged.\n" +
			"Its own repository does not exist and never did, so there is nothing unreachable\n" +
			"about it and nothing it could be the last copy of. Reporting that absence is how\n" +
			"the sixth round condemned this exact configuration to fail every night.")
	}
}

// TestAnAllNamedDiscoverNamesItsMissingRepositoryWithoutFlaggingIt closes the
// instrument gap on the Note the discovery gate depends on.
//
// The external test asserts only that the missing domain repository is NAMED. It
// stays green with Note:true deleted - and without the Note the sentence becomes
// actionable, so skippedNeedsAction turns true, the Recovery pill goes amber and
// the wizard's own success toast disappears, permanently, for every all-named
// install. Both halves have to be pinned: said, and not flagged.
func TestAnAllNamedDiscoverNamesItsMissingRepositoryWithoutFlaggingIt(t *testing.T) {
	eng := &hookFakeEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, st, own, cold := hookSvc(t, eng)
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "2222bbbb", Tags: []string{"container:sonarr"}}}
	if err := os.RemoveAll(own); err != nil {
		t.Fatal(err)
	}
	_ = st

	res, err := svc.Discover(context.Background(), true)
	skipped := res.Skipped
	if err != nil {
		t.Fatalf("an all-named domain has no repository of its own; that is not a failure: %v", err)
	}
	if len(skipped) == 0 {
		t.Fatal("the missing domain repository was not named at all")
	}
	if n := len(actionableSkips(skipped)); n != 0 {
		t.Errorf("%d of the skips are actionable, want 0.\n"+
			"An all-named install has no repository of its own by design, so flagging its\n"+
			"absence holds the Recovery readability pill amber for good and swallows the\n"+
			"save-success toast behind it - the sixth round's lesson, in the UI.", n)
	}
}

// TestAPartialDiscoverOfFileSetsWithholdsTheAttributionToo closes the other gap:
// the container test cannot see DiscoverFileSets, where the attribution rides in
// the CreateFileSet INSERT rather than in a setter, so deleting the blanking
// there bakes the wrong repository straight into a new row with the suite green.
func TestAPartialDiscoverOfFileSetsWithholdsTheAttributionToo(t *testing.T) {
	eng := &hookFakeEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	svc, st, own, cold := hookSvc(t, eng)
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.FilesPath = "backups/containers" // the files domain's own repository, same folder
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	// A file set whose snapshots are in the named repository, and a domain
	// repository that was there before and is not reachable now.
	eng.snapsByRepo[filepath.ToSlash(cold)] = []restic.Snapshot{{ID: "3333cccc", Tags: []string{"fileset:photos"}}}
	if err := st.MarkRepoEstablished(filepath.ToSlash(own)); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(own); err != nil {
		t.Fatal(err)
	}

	res, dErr := svc.DiscoverFileSets(context.Background(), false)
	n := res.Found
	if dErr == nil {
		t.Fatal("a files repository that was there and is gone must reach the caller as an error")
	}
	if n != 1 {
		t.Fatalf("discovered = %d, want 1", n)
	}
	set, gErr := st.GetFileSetByName("photos")
	if gErr != nil {
		t.Fatalf("the set was not created: %v", gErr)
	}
	if strings.TrimSpace(set.Repo) != "" {
		t.Errorf("the set was created on %q from an incomplete pass.\n"+
			"Here the attribution rides in the INSERT, not in a setter, so it is written and\n"+
			"latched in one statement.", set.Repo)
	}
}

// TestTheHookAgesTheDestinationWhenEverySourceIsThere is the healthy half, and
// the reason the fix is a skip list rather than a blanket suppression: an
// install with no separate off-site schedule replicates ONLY through this hook,
// so switching its retention off would mean the destination is never aged at all.
func TestTheHookAgesTheDestinationWhenEverySourceIsThere(t *testing.T) {
	eng := &hookFakeEngine{}
	svc, st, own, _ := hookSvc(t, eng)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	svc.replicateOffsite(context.Background(), "containers", settings, own, "container:plex")

	if len(eng.copied) != 1 {
		t.Fatalf("the hook copied %v, want exactly the repository the backup wrote", eng.copied)
	}
	if len(eng.forgot) == 0 {
		t.Error("the destination was not aged although every source of this domain is reachable.\n" +
			"On an install with no separate off-site schedule this hook is the only replication\n" +
			"there is, so suppressing its retention lets the destination grow without bound.")
	}
}

// TestASwitchedOffRepositoryDoesNotStopTheDestinationFromBeingAged is the third
// state, and the one that separates "this failed" from "this is gone".
//
// Switching a named repository off is deliberate and first-class, and it makes
// every domain-wide operation report incomplete - correctly, because the items
// pointing at it are not being backed up. It does NOT make it unreachable: the
// repository sits there with its data, so the off-site destination cannot be the
// last copy of anything because of it. Refusing the retention on it left the
// destination growing for good, and on an install where this hook is the only
// replication nothing else would ever have aged it.
func TestASwitchedOffRepositoryDoesNotStopTheDestinationFromBeingAged(t *testing.T) {
	eng := &hookFakeEngine{}
	svc, st, _, _ := hookSvc(t, eng)
	switchTheNamedRepositoryOff(t, st)

	// The WHOLE-DOMAIN pass, deliberately: it hands the retention gate the full
	// skip list, so it is the path on which the gate's own filter decides. The
	// pass reports incomplete, and that is correct - the items on the
	// switched-off repository are not being backed up at all.
	if err := svc.ReplicateOffsite(context.Background(), "containers"); err == nil {
		t.Fatal("a repository items still point at, switched off, must make the pass report incomplete")
	}
	if len(eng.forgot) == 0 {
		t.Error("a repository the operator switched off stopped the destination from being aged.\n" +
			"It is a failure for an operation asked to cover the whole domain, and it is not\n" +
			"UNREACHABLE - the data is right there - so the destination is not the last copy of\n" +
			"anything. Keying the gate on 'did anything fail' stops that domain's off-site\n" +
			"retention for good over a setting somebody chose on purpose.")
	}
}

// TestTheHookDoesNotReportASwitchedOffRepositoryAsAFailure is the hook's own
// half of the same split, and the reason the hook narrows the list before
// passing it rather than relying on the gate alone.
//
// The list the hook fetches would otherwise become the PASS's error too, and a
// hook that deliberately copies one repository has no business reporting that it
// did not cover the others - it never set out to. Handing the whole list over
// stamped the domain's off-site run row red after every single backup.
func TestTheHookDoesNotReportASwitchedOffRepositoryAsAFailure(t *testing.T) {
	eng := &hookFakeEngine{}
	svc, st, own, _ := hookSvc(t, eng)
	switchTheNamedRepositoryOff(t, st)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	svc.replicateOffsite(context.Background(), "containers", settings, own, "container:plex")

	runs, err := st.ListRuns(20)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range runs {
		if r.Kind == "offsite" && r.Status != "success" {
			t.Errorf("the post-backup hook stamped the off-site run %q: %s\n"+
				"It copied what it set out to copy. A switched-off repository is the whole-domain\n"+
				"pass's report to make, and making it here puts a red row in the activity log and a\n"+
				"line in the weekly digest after every single backup - which is how an operator\n"+
				"learns to ignore the one message that would have mattered.", r.Status, r.Error)
		}
	}
}

// switchTheNamedRepositoryOff turns the fixture's named repository off while an
// item still points at it: a deliberate, first-class state, and the ordinary
// reaction to a share dying.
func switchTheNamedRepositoryOff(t *testing.T, st *store.Repo) {
	t.Helper()
	rows, err := st.ListNamedRepos()
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListNamedRepos = %v, %v", rows, err)
	}
	rows[0].Enabled = false
	if _, err := st.UpsertOffsiteTarget(rows[0]); err != nil {
		t.Fatal(err)
	}
}

// TestTheHookDoesNotAgeTheDestinationBehindAnUnreachableSource is the defect.
func TestTheHookDoesNotAgeTheDestinationBehindAnUnreachableSource(t *testing.T) {
	eng := &hookFakeEngine{}
	svc, st, own, cold := hookSvc(t, eng)
	// cold was a working repository of this domain and is gone now. The hook does
	// not copy it - it copies what the backup just wrote - but the destination
	// holds its items, and they now have no other copy.
	if err := st.MarkRepoEstablished(filepath.ToSlash(cold)); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(cold); err != nil {
		t.Fatal(err)
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	svc.replicateOffsite(context.Background(), "containers", settings, own, "container:plex")

	if len(eng.copied) != 1 {
		t.Fatalf("the hook copied %v, want exactly the repository the backup wrote", eng.copied)
	}
	if len(eng.forgot) != 0 || len(eng.pruned) != 0 {
		t.Errorf("the hook aged the destination (forgot %v, pruned %v) after opening one source,\n"+
			"while a repository this domain uses was unreachable. The retention there is per\n"+
			"identity over what the DESTINATION holds, so it trims the off-site copies of the\n"+
			"items whose only other copy is the repository that went away.", eng.forgot, eng.pruned)
	}
}
