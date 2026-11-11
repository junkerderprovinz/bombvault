package api

import (
	"context"
	"net/http"
	"reflect"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const linkedAt = 1_758_000_050

// renamedKnightLoader is the container knightloader, which went by kl until
// linkedAt.
func renamedKnightLoader(t *testing.T, f *placementFixture) store.Target {
	t.Helper()
	tg := f.container("knightloader", "")
	if _, err := f.st.AddAliasAt("container", "kl", tg.ID, linkedAt); err != nil {
		t.Fatal(err)
	}
	return tg
}

func TestTheOwnBackupsOfAnAppAgeByTheContainersPolicy(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	tg := f.container("knightloader", nas.ID)
	settings := withOwnRetention(t, f.st, 3, map[string]store.RetentionKeep{"containers": {KeepDaily: 7}})

	f.svc.forgetAppExportSeries(context.Background(), f.root+"/nas", settings, restic.Mode{}, "knightloader", tg.ID)
	want := []forgetCall{{Repo: f.root + "/nas", Tags: []string{"appexport:knightloader"}, Policy: restic.RetentionPolicy{KeepDaily: 7}}}
	if !reflect.DeepEqual(f.eng.forgets, want) {
		t.Fatalf("forgets = %+v, want %+v", f.eng.forgets, want)
	}
}

func TestTheOwnBackupsOfAnAppAgeWithTheContainersFormerName(t *testing.T) {
	f := newPlacementFixture(t)
	tg := renamedKnightLoader(t, f)
	f.hold(f.domainPath("containers"), appExportSnap("e1e1e1e1", linkedAt-100, "kl", "knightloader"))
	settings := withOwnRetention(t, f.st, 3, nil)

	f.svc.forgetAppExportSeries(context.Background(), f.domainPath("containers"), settings, restic.Mode{}, "knightloader", tg.ID)
	if len(f.eng.forgets) != 1 || !slices.Equal(f.eng.forgets[0].Tags, []string{"appexport:knightloader", "appexport:kl"}) {
		t.Fatalf("forgets = %+v, want one series over both names", f.eng.forgets)
	}
}

func TestAnAppBackupFollowsItsContainersCopyRule(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.replicated("containers")
	f.rule("containers", "container:plex", store.SkipAll)
	f.hold(f.domainPath("containers"),
		snap("a1", 100, "container:nginx"), appExportSnap("e1", 100, "nginx", "knightloader"),
		snap("p1", 100, "container:plex"), appExportSnap("e2", 100, "plex", "towertorch"))

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if len(f.eng.copies) != 1 || !slices.Equal(f.eng.copies[0].IDs, []string{"a1", "e1"}) {
		t.Fatalf("copies = %+v, want nginx's backup and its app's, and nothing of plex", f.eng.copies)
	}
}

func TestAnAppBackupInANamedRepositoryGoesOffSiteWithItsContainer(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	f.target("containers", "B2", "b2:bucket:containers")
	f.replicated("containers")
	f.hold(f.root+"/nas", snap("a1", 100, "container:nginx"), appExportSnap("e1", 100, "nginx", "knightloader"))

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	var sent []string
	for _, c := range f.eng.copies {
		if c.Src == f.root+"/nas" {
			sent = append(sent, c.IDs...)
		}
	}
	if !slices.Equal(sent, []string{"a1", "e1"}) {
		t.Fatalf("copied %v from NAS, want the backup and its app's", sent)
	}
}

func TestATargetAgesTheAppBackupsAsASeriesOfTheirOwn(t *testing.T) {
	f := newPlacementFixture(t)
	keepLast(t, f, f.target("containers", "B2", "b2:bucket:containers"), 1)
	f.replicated("containers")
	f.hold(f.domainPath("containers"),
		snap("a1", 100, "container:nginx"),
		appExportSnap("e1", 90, "nginx", "knightloader"), appExportSnap("e2", 190, "nginx", "knightloader"))

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(f.eng.forgets, func(c forgetCall) bool {
		return c.Repo == "b2:bucket:containers" && slices.Equal(c.Tags, []string{"appexport:nginx"})
	})
	if i < 0 {
		t.Fatalf("forgets = %+v, want the app backups aged at B2 apart from the container's", f.eng.forgets)
	}
}

func TestAppBackupTagsAreOfTheContainersDomain(t *testing.T) {
	if got := identityDomain("appexport:nginx"); got != "containers" {
		t.Fatalf("identityDomain = %q", got)
	}
	tags := identityTags([]restic.Snapshot{appExportSnap("e1", 100, "nginx", "knightloader")})
	if !slices.Equal(tags, []string{"appexport:nginx"}) {
		t.Fatalf("identityTags = %v", tags)
	}
}

func TestTheAppBackupsOfAFormerNameStayWithTheRenamedContainer(t *testing.T) {
	f := newPlacementFixture(t)
	renamedKnightLoader(t, f)
	f.hold(f.domainPath("containers"),
		snap("a1a1a1a1", linkedAt+100, "container:knightloader"),
		appExportSnap("e1e1e1e1", linkedAt-100, "kl", "knightloader", "appname:KnightLoader"),
		// Taken under the old name after the link: another machine's.
		appExportSnap("e2e2e2e2", linkedAt+50, "kl", "knightloader"))

	tl, err := f.svc.Timeline(context.Background(), "containers", "knightloader")
	if err != nil {
		t.Fatal(err)
	}
	if got := rowKeys(tl.Rows); !slices.Equal(got, []string{"a1a1a1a1", "e1e1e1e1"}) {
		t.Fatalf("rows = %v, want the backup and the app backup from before the rename", got)
	}
	if tl.Rows[1].AppExport == nil || tl.Rows[1].AppExport.Name != "KnightLoader" {
		t.Fatalf("row = %+v, want it marked as KnightLoader's", tl.Rows[1])
	}

	eng := &appExportEngine{placementEngine: f.eng, raw: zipBackup}
	f.svc.engine = eng
	f.signIn()
	if w := f.download("/api/containers/knightloader/appexports/e1e1e1e1/download"); w.Code != http.StatusOK {
		t.Fatalf("download under the former name = %d", w.Code)
	}
	if want := []string{"e1e1e1e1 /appexport/kl/knightloader"}; !slices.Equal(eng.dumped, want) {
		t.Fatalf("streamed %v, want %v", eng.dumped, want)
	}
	if w := f.download("/api/containers/knightloader/appexports/e2e2e2e2/download"); w.Code != http.StatusConflict {
		t.Fatalf("download of another machine's app backup = %d, want 409", w.Code)
	}
}

func TestAReceivedAppBackupBelongsToItsContainer(t *testing.T) {
	snap := appExportSnap("e1", 100, "knightloader", "knightloader", "appname:KnightLoader")
	if got := receiverItemTag(snap); got != "appexport:knightloader" {
		t.Fatalf("receiverItemTag = %q, want the app backup's own item", got)
	}
}

// An app backup comes only while the app runs in the container, so its pause
// says nothing the container's own item does not.
func TestTheDeadManLeavesAppBackupsAlone(t *testing.T) {
	snaps := []restic.Snapshot{
		{Hostname: "tower", Time: "2026-07-01T10:00:00Z", Tags: []string{"container:knightloader", "p1"}},
		{Hostname: "tower", Time: "2026-07-01T09:59:00Z", Tags: []string{"appexport:knightloader", "app:knightloader"}},
	}
	got := receiverDeadManSourcesOf(snaps)
	if len(got) != 1 || got[0].source != "container:knightloader @ tower" {
		t.Fatalf("sources = %+v, want the container alone", got)
	}
}

func TestAFindingOnAContainerHoldsItsAppBackupsAtTheTargets(t *testing.T) {
	f := newPlacementFixture(t)
	renamedKnightLoader(t, f)
	tags := f.svc.itemIdentityTags(anomalyItemRef{Domain: anomalyDomainContainer, Name: "knightloader"})
	for _, want := range []string{"container:knightloader", "appexport:knightloader", "appexport:kl"} {
		if !slices.Contains(tags, want) {
			t.Errorf("held tags = %v, want %s among them", tags, want)
		}
	}
}

func TestDeletingEveryBackupOfAContainerTakesItsAppBackups(t *testing.T) {
	f, _ := appExportFixture(t)

	if err := f.svc.DeleteBackups(context.Background(), "knightloader", "local"); err != nil {
		t.Fatal(err)
	}
	var gone []string
	for _, d := range f.eng.deletes {
		gone = append(gone, d.IDs...)
	}
	slices.Sort(gone)
	if !slices.Equal(gone, []string{"a1a1a1a1", "e1e1e1e1"}) {
		t.Fatalf("deleted %v, want the backup and the app backup", gone)
	}
}

func TestDeletingEveryBackupAtATargetTakesTheAppBackupsThere(t *testing.T) {
	f, _ := appExportFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.hold("b2:bucket:containers",
		copied("b1b1b1b1", "a1a1a1a1", 1_758_000_100, "container:knightloader"),
		copied("f1f1f1f1", "e1e1e1e1", 1_758_000_000, "appexport:knightloader", "app:knightloader"))

	if err := f.svc.DeleteBackups(context.Background(), "knightloader", "offsite:"+b2.ID); err != nil {
		t.Fatal(err)
	}
	var gone []string
	for _, d := range f.eng.deletes {
		gone = append(gone, d.IDs...)
	}
	slices.Sort(gone)
	if !slices.Equal(gone, []string{"b1b1b1b1", "f1f1f1f1"}) {
		t.Fatalf("deleted %v at B2, want the copy of the backup and of the app backup", gone)
	}
}
