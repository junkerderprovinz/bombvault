package api

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const hour = int64(3600)

func (f *placementFixture) settings(change func(*store.Settings)) {
	f.t.Helper()
	s, err := f.st.GetSettings()
	if err != nil {
		f.t.Fatal(err)
	}
	change(&s)
	if err := f.st.UpdateSettings(s); err != nil {
		f.t.Fatal(err)
	}
}

func assertPlan(t *testing.T, got, want *placementPlan) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan = %+v, want %+v", got, want)
	}
}

func TestPlanNamesTheHomeAndItsTargets(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS Keller", "nas/bv")
	f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", nas.ID)
	assertPlan(t, f.cardOf("containers", "nginx", 0).Plan,
		&placementPlan{Kind: "home", Home: "NAS Keller", Targets: []string{"B2"}})
}

func TestPlanWarnsWhenNothingLeavesThePremises(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", "")
	f.rule("containers", "container:nginx", "*")
	assertPlan(t, f.cardOf("containers", "nginx", 0).Plan,
		&placementPlan{Kind: "home", Targets: []string{}, Warn: true, NoCopy: true})
}

func TestPlanCountsAHomeMarkedOffThePremises(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS Keller", "nas/bv")
	f.offPremises(nas.ID)
	f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", nas.ID)
	f.rule("containers", "container:nginx", "*")
	assertPlan(t, f.cardOf("containers", "nginx", 0).Plan,
		&placementPlan{Kind: "home", Home: "NAS Keller", Targets: []string{}})
}

func TestPlanOfAnOpenItemAsksTheLocalDomainPathOncePerList(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS Keller", "nas/bv")
	f.target("containers", "B2", "b2:bucket:containers")
	f.setDefault("containers", nas.ID)
	f.openContainer("nginx")
	f.openContainer("plex")
	f.hold(f.domainPath("containers"), snap("a1", 1_758_000_000, "container:nginx"))

	views := f.views("containers", f.item("containers", "nginx", 0), f.item("containers", "plex", 0))
	assertPlan(t, views["nginx"].Plan, &placementPlan{Kind: "stays-domain", Targets: []string{"B2"}})
	assertPlan(t, views["plex"].Plan, &placementPlan{Kind: "default-home", Home: "NAS Keller", Targets: []string{"B2"}})
	if n := f.eng.lists[f.domainPath("containers")]; n != 1 {
		t.Fatalf("the domain path was listed %d times for one list", n)
	}
}

func TestPlanOfAnOpenItemOnARemoteDomainPathDecidesAtTheFirstBackup(t *testing.T) {
	f := newPlacementFixture(t)
	f.settings(func(s *store.Settings) { s.ContainersPath = "s3:https://s3.example.com/containers" })
	nas := f.namedRepo("NAS Keller", "nas/bv")
	f.setDefault("containers", nas.ID)
	f.openContainer("nginx")
	got := f.cardOf("containers", "nginx", 0).Plan
	if got.Kind != "decides-at-first-backup" || got.NoCopy || got.Warn {
		t.Fatalf("plan = %+v, want decides-at-first-backup without a warning", got)
	}
	if n := f.eng.lists["s3:https://s3.example.com/containers"]; n != 0 {
		t.Fatalf("a remote domain path was listed %d times for a card", n)
	}
}

func TestPlanOfAnOpenItemSaysWhenTheDefaultCannotBeUsed(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS Keller", "nas/bv")
	nas.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(nas); err != nil {
		t.Fatal(err)
	}
	f.setDefault("containers", nas.ID)
	f.openContainer("nginx")
	assertPlan(t, f.cardOf("containers", "nginx", 0).Plan,
		&placementPlan{Kind: "not-backed-up", Home: "NAS Keller", Targets: []string{}, Warn: true, Reason: "default-off"})

	f.setDefault("containers", "0123456789abcdef0123456789abcdef")
	assertPlan(t, f.cardOf("containers", "nginx", 0).Plan,
		&placementPlan{Kind: "not-backed-up", Targets: []string{}, Warn: true, Reason: "default-missing"})
}

func TestPlanOfAPausedDomainIsThePause(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", "")
	f.paused("containers")
	assertPlan(t, f.cardOf("containers", "nginx", 0).Plan,
		&placementPlan{Kind: "paused", Targets: []string{}, Warn: true})
}

func dailyContainerBackups(f *placementFixture) {
	f.settings(func(s *store.Settings) {
		s.ContainersSchedule = "daily 03:00"
		s.ContainersOffsiteSchedule = ""
		s.EverythingSchedule = "off"
	})
}

func placeAt(o *placementObserved, place string) observedPlace {
	for _, pl := range o.Places {
		if pl.Place == place {
			return pl
		}
	}
	return observedPlace{}
}

func TestObservedCountsAFreshCopyAtTheTarget(t *testing.T) {
	f := newPlacementFixture(t)
	dailyContainerBackups(f)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", "")
	now := time.Now().Unix()
	f.listing("containers", b2.ID, now-hour, copiesRow("container:nginx", 20, now-2*hour))

	o := f.cardOf("containers", "nginx", now-2*hour).Observed
	if o.Sites != 2 || o.Rule321 != "met" || o.Tone != "ok" {
		t.Fatalf("observed = %+v, want two sites, 3-2-1 met", o)
	}
	want := observedPlace{Place: "offsite:" + b2.ID, Label: "B2", Count: 20, Latest: now - 2*hour, SeenAt: now - hour, State: "counts", Counts: true}
	if got := placeAt(o, "offsite:"+b2.ID); got != want {
		t.Fatalf("B2 = %+v, want %+v", got, want)
	}
}

func TestObservedSaysATargetFailedAfterItsLastListing(t *testing.T) {
	f := newPlacementFixture(t)
	dailyContainerBackups(f)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", "")
	now := time.Now().Unix()
	f.listing("containers", b2.ID, now-10*hour, copiesRow("container:nginx", 20, now-11*hour))
	id, err := f.st.RecordOffsiteRunForTarget("containers", b2.ID, now-hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.FinishOffsiteRun(id, false, "connection refused"); err != nil {
		t.Fatal(err)
	}

	o := f.cardOf("containers", "nginx", now-2*hour).Observed
	got := placeAt(o, "offsite:"+b2.ID)
	if got.State != "unreachable" || got.Since != now-hour || got.SeenAt != now-10*hour || got.Counts {
		t.Fatalf("B2 = %+v, want unreachable since the failed run", got)
	}
	if o.Tone != "warn" || o.Rule321 != "one-copy" || o.Sites != 1 {
		t.Fatalf("observed = %+v", o)
	}
}

func TestObservedIsUnconfirmedWhenTheListingIsOlderThanTheGrace(t *testing.T) {
	f := newPlacementFixture(t)
	dailyContainerBackups(f)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", "")
	now := time.Now().Unix()
	f.listing("containers", b2.ID, now-72*hour, copiesRow("container:nginx", 20, now-73*hour))

	o := f.cardOf("containers", "nginx", now-73*hour).Observed
	got := placeAt(o, "offsite:"+b2.ID)
	if got.State != "unknown" || got.Since != now-72*hour || !got.Stale {
		t.Fatalf("B2 = %+v, want state unknown since the last listing", got)
	}
	if o.Rule321 != "unconfirmed" || o.Tone != "unconfirmed" {
		t.Fatalf("observed = %+v, want 3-2-1 unconfirmed", o)
	}
}

func TestObservedCallsACopyTooFarBehindTheLastBackupOld(t *testing.T) {
	f := newPlacementFixture(t)
	dailyContainerBackups(f)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", "")
	now := time.Now().Unix()
	f.listing("containers", b2.ID, now-hour, copiesRow("container:nginx", 20, now-96*hour))

	got := placeAt(f.cardOf("containers", "nginx", now-hour).Observed, "offsite:"+b2.ID)
	if got.State != "old-copy" || !got.Stale || got.Counts {
		t.Fatalf("B2 = %+v, want old-copy", got)
	}
}

func TestObservedSaysATargetHoldsNoCopyOfTheItemYet(t *testing.T) {
	f := newPlacementFixture(t)
	dailyContainerBackups(f)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", "")
	f.container("plex", "")
	now := time.Now().Unix()
	f.listing("containers", b2.ID, now-hour, copiesRow("container:plex", 20, now-2*hour))

	o := f.cardOf("containers", "nginx", now-2*hour).Observed
	want := observedPlace{Place: "offsite:" + b2.ID, Label: "B2", State: "no-copy"}
	if got := placeAt(o, "offsite:"+b2.ID); got != want {
		t.Fatalf("B2 = %+v, want %+v", got, want)
	}
	if o.Sites != 1 || o.Rule321 != "one-copy" || o.Tone != "warn" {
		t.Fatalf("observed = %+v, want one site and 3-2-1 not met", o)
	}
}

func TestObservedHasNoSightingAtAnUnreachableTargetThatHoldsNothing(t *testing.T) {
	f := newPlacementFixture(t)
	dailyContainerBackups(f)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", "")
	f.container("plex", "")
	now := time.Now().Unix()
	f.listing("containers", b2.ID, now-10*hour, copiesRow("container:plex", 20, now-11*hour))
	id, err := f.st.RecordOffsiteRunForTarget("containers", b2.ID, now-hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.FinishOffsiteRun(id, false, "connection refused"); err != nil {
		t.Fatal(err)
	}

	got := placeAt(f.cardOf("containers", "nginx", now-2*hour).Observed, "offsite:"+b2.ID)
	if got.State != "unreachable" || got.Since != now-hour || got.SeenAt != 0 {
		t.Fatalf("B2 = %+v, want unreachable without a sighting of a copy", got)
	}
}

func TestObservedCannotConfirmATargetItHasNeverListed(t *testing.T) {
	f := newPlacementFixture(t)
	dailyContainerBackups(f)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", "")
	now := time.Now().Unix()

	o := f.cardOf("containers", "nginx", now-2*hour).Observed
	got := placeAt(o, "offsite:"+b2.ID)
	if got.State != "unknown" || !got.Stale {
		t.Fatalf("B2 = %+v, want state unknown", got)
	}
	if o.Rule321 != "unconfirmed" || o.Tone != "unconfirmed" {
		t.Fatalf("observed = %+v, want 3-2-1 unconfirmed", o)
	}
}

func TestObservedIsStrictWithoutAnySchedule(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", "")
	f.container("plex", "")
	now := time.Now().Unix()
	f.listing("containers", b2.ID, now-30*24*hour,
		copiesRow("container:nginx", 5, now-31*24*hour),
		copiesRow("container:plex", 5, now-31*24*hour-1),
	)
	views := f.views("containers",
		f.item("containers", "nginx", now-31*24*hour),
		f.item("containers", "plex", now-31*24*hour),
	)
	if got := placeAt(views["nginx"].Observed, "offsite:"+b2.ID); got.State != "counts" {
		t.Fatalf("a copy of the last backup counts however old the listing: %+v", got)
	}
	if got := placeAt(views["plex"].Observed, "offsite:"+b2.ID); got.State != "old-copy" {
		t.Fatalf("a copy one second older than the last backup does not count: %+v", got)
	}
}

func TestObservedListsOlderCopiesAtATargetNoLongerTicked(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.appendOnly(b2.ID)
	f.container("nginx", "")
	f.rule("containers", "container:nginx", "*")
	f.listing("containers", b2.ID, 1_758_000_000, copiesRow("container:nginx", 20, 1_757_900_000))

	o := f.cardOf("containers", "nginx", 1_758_100_000).Observed
	want := []olderCopies{{TargetID: b2.ID, Name: "B2", Count: 20, SeenAt: 1_758_000_000, AppendOnly: true}}
	if !reflect.DeepEqual(o.Older, want) {
		t.Fatalf("older = %+v, want %+v", o.Older, want)
	}
	if len(o.Places) != 1 || o.Sites != 1 || o.Rule321 != "one-copy" {
		t.Fatalf("observed = %+v, want only the home", o)
	}
}

func TestObservedCountsAHomeMarkedOffThePremisesAsASite(t *testing.T) {
	f := newPlacementFixture(t)
	dailyContainerBackups(f)
	nas := f.namedRepo("NAS Keller", "nas/bv")
	f.offPremises(nas.ID)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", nas.ID)
	f.container("plex", nas.ID)
	f.rule("containers", "container:plex", "*")
	now := time.Now().Unix()
	f.listing("containers", b2.ID, now-hour, copiesRow("container:nginx", 20, now-2*hour))

	views := f.views("containers", f.item("containers", "nginx", now-2*hour), f.item("containers", "plex", now-2*hour))
	if o := views["nginx"].Observed; o.Sites != 3 || o.Rule321 != "met" {
		t.Fatalf("NAS off the premises plus B2 = %+v, want three sites, 3-2-1 met", o)
	}
	if o := views["plex"].Observed; o.Sites != 2 || o.Rule321 != "one-copy" {
		t.Fatalf("NAS off the premises alone = %+v, want two sites, one copy", o)
	}
}

func TestObservedDimsASwitchedOffTargetThatStillHoldsCopies(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	b2.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(b2); err != nil {
		t.Fatal(err)
	}
	f.container("nginx", "")
	f.listing("containers", b2.ID, 1_758_000_000, copiesRow("container:nginx", 3, 1_757_900_000))

	got := placeAt(f.cardOf("containers", "nginx", 1_757_900_000).Observed, "offsite:"+b2.ID)
	if got.State != "off" || got.Counts {
		t.Fatalf("B2 = %+v, want off and not counting", got)
	}
}

func TestObservedSaysNoBackupBeforeTheFirstOne(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", "")
	if o := f.cardOf("containers", "nginx", 0).Observed; !o.NoBackup {
		t.Fatalf("observed = %+v, want noBackup", o)
	}
}

// stackListDocker lists the containers in stacks as installed members of
// their compose project.
type stackListDocker struct {
	dockercli.Docker
	stacks map[string]string
}

func (d stackListDocker) List(context.Context) ([]dockercli.ContainerInfo, error) {
	infos := make([]dockercli.ContainerInfo, 0, len(d.stacks))
	for name, stack := range d.stacks {
		infos = append(infos, dockercli.ContainerInfo{Name: name, State: "running", Stack: stack})
	}
	return infos, nil
}

func TestContainerListNamesTheProjectFolderOfInstalledAndRemovedMembers(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.container("immich-server", "")
	removed := f.container("immich-ml", "")
	removed.Definition = `{"inspect":{"Config":{"Labels":{"com.docker.compose.project":"immich"}}}}`
	if _, err := f.st.UpsertTarget(removed); err != nil {
		t.Fatal(err)
	}
	f.rule("containers", "container:immich-server", "*")
	f.rule("containers", "container:immich-ml", "*")
	f.h.docker = stackListDocker{Docker: f.dock, stacks: map[string]string{"immich-server": "immich"}}

	rows := f.do("GET", "/api/containers", nil)["containers"].([]any)
	if len(rows) != 2 {
		t.Fatalf("containers = %v, want the installed and the removed member", rows)
	}
	for _, row := range rows {
		c := row.(map[string]any)
		note, _ := c["placement"].(map[string]any)["stackNote"].(map[string]any)
		if note["project"] != "immich" {
			t.Errorf("%s: stack note = %v, want the immich project folder", c["name"], note)
		}
	}
}

func TestStackNoteAppearsWhenTheProjectFolderIsCopiedElsewhere(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.container("immich-server", "")
	f.rule("containers", "container:immich-server", "*")
	it := f.item("containers", "immich-server", 0)
	it.Stack = "immich"
	got := f.views("containers", it)["immich-server"].StackNote
	want := &stackNote{Project: "immich", Home: "", Targets: []string{"B2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stack note = %+v, want %+v", got, want)
	}
}

func TestStackNoteStaysAwayWhileMemberAndFolderGoToTheSameTargets(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.container("immich-server", "")
	it := f.item("containers", "immich-server", 0)
	it.Stack = "immich"
	if got := f.views("containers", it)["immich-server"].StackNote; got != nil {
		t.Fatalf("stack note = %+v, want none", got)
	}
}
