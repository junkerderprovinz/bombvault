package api

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestItemBackupsSeesASuccessfulRun(t *testing.T) {
	f := newPlacementFixture(t)
	tg := f.container("web", "")
	f.backupRun(tg.ID, 100)
	got, err := f.svc.itemBackups(context.Background(), store.ItemRef{Domain: "containers", Key: "web"})
	if err != nil || got != backupsPresent {
		t.Fatalf("itemBackups = %v, %v, want present", got, err)
	}
}

func TestItemBackupsOfAnOpenItemReadsTheDomainPath(t *testing.T) {
	f := newPlacementFixture(t)
	f.openContainer("nginx")
	f.hold(f.domainPath("containers"), snap("aaaa0001", 100, "container:nginx"))
	got, err := f.svc.itemBackups(context.Background(), store.ItemRef{Domain: "containers", Key: "nginx"})
	if err != nil || got != backupsPresent {
		t.Fatalf("itemBackups = %v, %v, want present", got, err)
	}
}

func TestItemBackupsKeepsAnUnreadableLocationApart(t *testing.T) {
	f := newPlacementFixture(t)
	f.openVM("win11")
	f.eng.listErr = map[string]error{f.domainPath("vms"): errors.New("wrong password or no key found")}
	got, err := f.svc.itemBackups(context.Background(), store.ItemRef{Domain: "vms", Key: "win11"})
	if got != backupsUnreadable || err == nil {
		t.Fatalf("itemBackups = %v, %v, want unreadable with its cause", got, err)
	}
	if had, err := f.svc.vmHasBackups(context.Background(), "win11"); err != nil || !had {
		t.Fatalf("vmHasBackups = %v, %v, want an unreadable location to count as backed up", had, err)
	}
}

func TestItemBackupsOfAChosenItemReadsOnlyItsOwnRepository(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("web", nas.ID)
	f.hold(f.domainPath("containers"), snap("aaaa0001", 100, "container:web"))
	got, err := f.svc.itemBackups(context.Background(), store.ItemRef{Domain: "containers", Key: "web"})
	if err != nil || got != backupsNone {
		t.Fatalf("itemBackups = %v, %v, want none on NAS", got, err)
	}
	if n := f.eng.lists[f.domainPath("containers")]; n != 0 {
		t.Errorf("the domain path was listed %d times for an item on NAS", n)
	}
}

func TestItemBackupsOfAMissingFileSetIsAnError(t *testing.T) {
	f := newPlacementFixture(t)
	_, err := f.svc.itemBackups(context.Background(), store.ItemRef{Domain: "files", Key: "no-such-set"})
	if !errors.Is(err, errFileSetNotFound) {
		t.Fatalf("err = %v, want errFileSetNotFound", err)
	}
}

func TestEffectiveHomeOfAnOpenItemIsTheDefault(t *testing.T) {
	p := placementRead{Domain: "containers", State: store.PlacementState{
		Domain: "containers", HasDefault: true,
		Default: store.PlacementDefault{Domain: "containers", Home: "repo-nas"},
	}}
	if repo, fromDefault := p.effectiveHome(store.HomeState{}); repo != "repo-nas" || !fromDefault {
		t.Errorf("missing row = %q, %v, want the default", repo, fromDefault)
	}
	if repo, fromDefault := p.effectiveHome(store.HomeState{Exists: true, Choice: store.RepoChosenUnread}); repo != "" || fromDefault {
		t.Errorf("unread row = %q, %v, want the domain path as chosen", repo, fromDefault)
	}
	if repo, fromDefault := p.effectiveHome(store.HomeState{Exists: true, Repo: "repo-old", Choice: store.RepoChosen}); repo != "repo-old" || fromDefault {
		t.Errorf("chosen row = %q, %v, want its own repository", repo, fromDefault)
	}
	if repo, _ := (placementRead{}).effectiveHome(store.HomeState{}); repo != "" {
		t.Errorf("without a default an open item lands on %q, want the domain path", repo)
	}
}

func settle(t *testing.T, f *placementFixture, item store.ItemRef) (string, func() (bool, error), error) {
	t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	return f.svc.settleHome(context.Background(), settings, item)
}

func TestSettleHomeTakesTheDefaultForAnOpenItem(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.setDefault("containers", nas.ID)
	f.openContainer("nginx")
	item := store.ItemRef{Domain: "containers", Key: "nginx"}
	repoID, commit, err := settle(t, f, item)
	if err != nil || repoID != nas.ID {
		t.Fatalf("settleHome = %q, %v, want NAS", repoID, err)
	}
	if got := f.home(item); got.Choice != store.RepoOpen {
		t.Fatalf("the step wrote before its commit: %+v", got)
	}
	if ok, err := commit(); err != nil || !ok {
		t.Fatalf("commit = %v, %v", ok, err)
	}
	if got := f.home(item); got.Repo != nas.ID || got.Choice != store.RepoChosen {
		t.Fatalf("home after commit = %+v, want NAS chosen", got)
	}
}

func TestSettleHomeKeepsAnItemWithHistoryOnTheDomainPath(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.setDefault("containers", nas.ID)
	f.openContainer("nginx")
	f.hold(f.domainPath("containers"), snap("aaaa0001", 100, "container:nginx"))
	item := store.ItemRef{Domain: "containers", Key: "nginx"}
	repoID, commit, err := settle(t, f, item)
	if err != nil || repoID != "" {
		t.Fatalf("settleHome = %q, %v, want the domain path", repoID, err)
	}
	if _, err := commit(); err != nil {
		t.Fatal(err)
	}
	if got := f.home(item); got.Repo != "" || got.Choice != store.RepoChosen {
		t.Fatalf("home = %+v, want the domain path chosen", got)
	}
}

func TestSettleHomeStopsWhenTheDomainPathCannotBeRead(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.setDefault("containers", nas.ID)
	f.openContainer("nginx")
	f.eng.listErr = map[string]error{f.domainPath("containers"): errors.New("share not mounted")}
	item := store.ItemRef{Domain: "containers", Key: "nginx"}
	if _, _, err := settle(t, f, item); err == nil {
		t.Fatal("an unreadable domain path let the first backup pick a location")
	}
	if got := f.home(item); got.Choice != store.RepoOpen {
		t.Fatalf("home = %+v, want it still open", got)
	}
}

func TestSettleHomeRefusesADefaultOnASwitchedOffRepository(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	nas.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(nas); err != nil {
		t.Fatal(err)
	}
	f.setDefault("vms", nas.ID)
	f.openVM("win11")
	_, _, err := settle(t, f, store.ItemRef{Domain: "vms", Key: "win11"})
	if err == nil || !strings.Contains(err.Error(), "default") {
		t.Fatalf("err = %v, want a refusal that names the default", err)
	}
}

func TestSettleHomeWithoutADefaultUsesTheDomainPath(t *testing.T) {
	f := newPlacementFixture(t)
	set := f.openFileSet("docs")
	repoID, _, err := settle(t, f, store.ItemRef{Domain: "files", Key: set.ID})
	if err != nil || repoID != "" {
		t.Fatalf("settleHome = %q, %v, want the domain path", repoID, err)
	}
}

func TestSettleHomeLeavesAChosenItemWhereItIs(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.setDefault("containers", "")
	f.container("web", nas.ID)
	repoID, commit, err := settle(t, f, store.ItemRef{Domain: "containers", Key: "web"})
	if err != nil || repoID != nas.ID {
		t.Fatalf("settleHome = %q, %v, want its own NAS", repoID, err)
	}
	if ok, err := commit(); err != nil || !ok {
		t.Fatalf("commit for a chosen item = %v, %v", ok, err)
	}
	if len(f.eng.lists) != 0 {
		t.Errorf("a chosen item listed repositories: %v", f.eng.lists)
	}
}

func TestSettleHomeCreatesTheRowOfANewContainer(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.setDefault("containers", nas.ID)
	item := store.ItemRef{Domain: "containers", Key: "fresh"}
	_, commit, err := settle(t, f, item)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := commit(); err != nil || !ok {
		t.Fatalf("commit = %v, %v", ok, err)
	}
	if got := f.home(item); got != (store.HomeState{Exists: true, Repo: nas.ID, Choice: store.RepoChosen}) {
		t.Fatalf("home = %+v, want a new row on NAS", got)
	}
}

func TestSettleCommitReportsARowChosenMeanwhile(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.setDefault("containers", nas.ID)
	f.openContainer("nginx")
	item := store.ItemRef{Domain: "containers", Key: "nginx"}
	_, commit, err := settle(t, f, item)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.WritePlacement(item, &store.HomeWrite{Choice: store.RepoChosen}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if ok, err := commit(); err != nil || ok {
		t.Fatalf("commit over a changed row = %v, %v, want false", ok, err)
	}
	if got := f.home(item); got.Repo != "" {
		t.Fatalf("home = %+v, want the choice made in between", got)
	}
}

func TestRecordHomeStartsOverWhenTheRowAppearedMeanwhile(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.setDefault("containers", nas.ID)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	item := store.ItemRef{Domain: "containers", Key: "nginx"}
	step, err := f.svc.prepareHome(context.Background(), settings, item)
	if err != nil {
		t.Fatal(err)
	}
	f.openContainer("nginx")
	step, err = f.svc.recordHome(context.Background(), settings, item, step)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.home(item); got.Repo != nas.ID || got.Choice != store.RepoChosen {
		t.Fatalf("home = %+v, want NAS chosen", got)
	}
	if !strings.HasSuffix(filepath.ToSlash(step.repo), "/nas") {
		t.Fatalf("the backup goes to %q, want NAS", step.repo)
	}
}

// TestRecordHomeGivesUpAfterAFewTurns pins a bound on the retry loop: a row
// under constant contention must not hold the domain lock and spend restic
// calls forever. onSnapshots fires synchronously inside settleHome, right
// after it captures the row a commit will later compare against, so a write
// made from the hook always lands behind that turn's read and the mismatch
// is guaranteed, never raced. A chosen home settles a step unconditionally,
// so the one way to keep every turn genuinely contested is toggling whether
// the open item's row exists at all. The hook stops after a generous number
// of turns, so the unfixed loop still terminates instead of hanging the
// test, just well past any reasonable cap.
func TestRecordHomeGivesUpAfterAFewTurns(t *testing.T) {
	f := newPlacementFixture(t)
	f.setDefault("containers", "")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	item := store.ItemRef{Domain: "containers", Key: "nginx"}
	turns := 0
	f.eng.onSnapshots = func() {
		turns++
		if turns > 6 {
			return
		}
		if turns%2 == 1 {
			if _, err := f.st.WritePlacement(item, &store.HomeWrite{Choice: store.RepoOpen}, nil, nil); err != nil {
				t.Fatal(err)
			}
			return
		}
		if _, err := f.db.Exec(`DELETE FROM targets WHERE container_name = ?`, "nginx"); err != nil {
			t.Fatal(err)
		}
	}
	step, err := f.svc.prepareHome(context.Background(), settings, item)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.recordHome(context.Background(), settings, item, step); !errors.Is(err, errHomeSettleExhausted) {
		t.Fatalf("recordHome = %v, want errHomeSettleExhausted", err)
	}
	if turns > 3 {
		t.Fatalf("recordHome took %d turns against a row in constant contention, want a small bounded number", turns)
	}
}
