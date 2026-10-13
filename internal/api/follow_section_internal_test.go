package api

import (
	"context"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// movedScene is a containers section whose Local was switched off: nginx and
// gitea followed it, with backups at Local, jellyfin keeps a NAS it chose for
// itself, and B2's direct repository takes the section's new backups.
type movedScene struct {
	f      *placementFixture
	direct store.OffsiteTarget
	nginx  store.ItemRef
	local  string
}

func moveContainersOffLocal(t *testing.T) movedScene {
	t.Helper()
	f := newPlacementFixture(t)
	ctx := context.Background()
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	direct := f.direct(b2)
	nas := f.namedRepo("NAS", "nas")
	nginx := f.container("nginx", "")
	gitea := f.container("gitea", "")
	jellyfin := f.container("jellyfin", nas.ID)
	for _, id := range []string{nginx.ID, gitea.ID, jellyfin.ID} {
		f.backupRun(id, 1_758_000_000)
	}
	local := f.domainPath("containers")
	f.hold(local,
		snap("a1a1a1a1", 1_757_000_000, "container:nginx"),
		snap("a2a2a2a2", 1_758_000_000, "container:nginx"),
		snap("b1b1b1b1", 1_758_000_000, "container:gitea"))

	change := defaultChange{Home: new(direct.ID), Skip: &[]string{store.SkipAll}}
	impact, err := f.svc.defaultImpactFor(ctx, "containers", change)
	if err != nil {
		t.Fatal(err)
	}
	if impact.Follow != 2 {
		t.Fatalf("impact = %+v, want nginx and gitea to follow", impact)
	}
	change.Expect = &impact
	if _, _, err := f.svc.putDefault(ctx, "containers", change); err != nil {
		t.Fatalf("switching Local off: %v", err)
	}
	if h := f.home(store.ItemRef{Domain: "containers", Key: "jellyfin"}); h.Choice != store.RepoChosen || h.Repo != nas.ID {
		t.Fatalf("jellyfin = %+v, want it to keep the NAS it chose", h)
	}
	return movedScene{f: f, direct: direct, nginx: store.ItemRef{Domain: "containers", Key: "nginx"}, local: local}
}

func TestAnEntryWithBackupsAtLocalFollowsItsSectionOffSite(t *testing.T) {
	sc := moveContainersOffLocal(t)
	settings, err := sc.f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	if h := sc.f.home(sc.nginx); h.Choice != store.RepoOpen {
		t.Fatalf("nginx = %+v, want it to follow its section", h)
	}
	repoID, commit, err := sc.f.svc.settleHome(settings, sc.nginx)
	if err != nil || repoID != sc.direct.ID {
		t.Fatalf("next backup of nginx goes to %q (%v), want B2's direct repository", repoID, err)
	}
	if ok, err := commit(); err != nil || !ok {
		t.Fatalf("settling nginx at B2: %v, %v", ok, err)
	}
	if h := sc.f.home(sc.nginx); h.Repo != sc.direct.ID {
		t.Fatalf("nginx after its next backup = %+v", h)
	}
}

func TestTheBackupsAnEntryLeftAtLocalStayListedAndRestorable(t *testing.T) {
	sc := moveContainersOffLocal(t)
	ctx := context.Background()
	settings, err := sc.f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if _, commit, err := sc.f.svc.settleHome(settings, sc.nginx); err != nil {
		t.Fatal(err)
	} else if _, err := commit(); err != nil {
		t.Fatal(err)
	}

	tl, err := sc.f.svc.Timeline(ctx, "containers", "nginx")
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(tl.Places, func(p timelinePlace) bool { return p.Place == folderPlace })
	if i < 0 || tl.Places[i].Kind != "earlier" || tl.Places[i].State != "read" {
		t.Fatalf("places = %+v, want Local among them as an earlier place", tl.Places)
	}
	if got := rowKeys(tl.Rows); !slices.Equal(got, []string{"a2a2a2a2", "a1a1a1a1"}) {
		t.Fatalf("rows = %v, want both backups nginx left at Local", got)
	}
	markOf(t, tl.Rows, "a1a1a1a1", folderPlace)

	repo, err := sc.f.svc.containerRepoForName(settings, "nginx", folderPlace)
	if err != nil || repo != sc.local {
		t.Fatalf("a restore from Local opens %q (%v), want the section's own folder", repo, err)
	}
	snaps, err := sc.f.svc.Snapshots(ctx, "nginx", normalizeSource(folderPlace))
	if err != nil || len(snaps) != 2 {
		t.Fatalf("Local lists %d backups of nginx (%v), want both", len(snaps), err)
	}
	if _, err := sc.f.svc.containerRepoForName(settings, "jellyfin", folderPlace); err == nil {
		t.Fatal("a restore of jellyfin from Local, where it never was, opened a repository")
	}
}

func TestNothingAnEntryLeftAtLocalIsDeleted(t *testing.T) {
	sc := moveContainersOffLocal(t)
	settings, err := sc.f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.RetentionKeepLast = 1
	if err := sc.f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	c := countCalls(sc.f)

	if _, err := sc.f.svc.PruneDomain(context.Background(), "containers", "local"); err != nil {
		t.Fatal(err)
	}

	if held := sc.f.eng.snaps[sc.local]; len(held) != 3 {
		t.Fatalf("Local holds %d backups after a prune, want all three left as they were", len(held))
	}
	if got := c.at(sc.local); got["forget"] != 0 {
		t.Fatalf("calls at Local = %v, want nothing forgotten there", got)
	}
}

func TestAnEntryThatNeverMovedKeepsItsPlaceUntilALineIsSwitched(t *testing.T) {
	f := newPlacementFixture(t)
	nginx := f.container("nginx", "")
	f.backupRun(nginx.ID, 1_758_000_000)
	ref := store.ItemRef{Domain: "containers", Key: "nginx"}

	if h := f.home(ref); h.Choice != store.RepoChosen || h.Repo != "" {
		t.Fatalf("nginx = %+v, want it where it is", h)
	}
	former, err := f.st.FormerHomes("containers", nginx.ID)
	if err != nil || len(former) != 0 {
		t.Fatalf("former homes = %v, %v, want none", former, err)
	}
	tl, err := f.svc.Timeline(context.Background(), "containers", "nginx")
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(tl.Places, func(p timelinePlace) bool { return p.Kind == "earlier" }) {
		t.Fatalf("places = %+v, want no earlier place", tl.Places)
	}
}
