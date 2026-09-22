package api

import (
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

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
