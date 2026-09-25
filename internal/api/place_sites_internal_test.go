package api

import (
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// lanRestServer is a rest-server in the same house, answered "here" to
// "Where is the device?".
func lanRestServer() store.Place {
	return store.Place{
		Name: "NAS rest-server", Provider: "rest-server", Kind: "rest", Base: "rest:http://nas.lan:8000/bv",
		Folders: map[string]string{"containers": "containers"}, Enabled: true,
	}
}

func TestAnInHouseRestServerAsDomainPathIsNoSiteOfItsOwn(t *testing.T) {
	f := newPlacementFixture(t)
	f.settings(func(s *store.Settings) { s.ContainersPath = "rest:http://nas.lan:8000/bv/containers" })
	f.storePlace(lanRestServer(), "containers")
	f.container("nginx", "")

	card := f.cardOf("containers", "nginx", 1_758_000_000)
	if !card.Plan.NoCopy || !card.Plan.Warn {
		t.Errorf("plan = %+v, want the warning that nothing leaves the premises", card.Plan)
	}
	if o := card.Observed; o.Sites != 1 || o.Rule321 != "one-copy" {
		t.Errorf("observed = %+v, want one site and one backup", o)
	}
	if f.domainStatus("containers").OffPremisesCovered {
		t.Error("a rest-server in the house covers nothing off the premises")
	}
}

func TestADomainPathAtAPlaceOffThePremisesCoversTheDomain(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(store.Place{
		Name: "NAS at a friend's", Provider: "share", Kind: "local", Base: "backups",
		Folders: map[string]string{"containers": "containers"}, OffPremises: true, Enabled: true,
	}, "containers")
	f.container("nginx", "")

	if plan := f.cardOf("containers", "nginx", 0).Plan; plan.NoCopy || plan.Warn {
		t.Errorf("plan = %+v, want no warning for a home at another site", plan)
	}
	if !f.domainStatus("containers").OffPremisesCovered {
		t.Error("a domain path at a place off the premises covers the domain")
	}
}

func TestADirectRepositoryAtAnInHousePlaceIsNoSiteOfItsOwn(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.target("containers", "NAS", "rest:http://nas.lan:8000/bv/containers")
	direct := f.direct(nas)
	lan := f.storePlace(lanRestServer())
	f.linkRow(nas.ID, lan, "containers", "")
	f.linkRow(direct.ID, lan, "containers", "-direct")
	f.container("nginx", direct.ID)

	card := f.cardOf("containers", "nginx", 1_758_000_000)
	if !card.Plan.NoCopy || !card.Plan.Warn {
		t.Errorf("plan = %+v, want the warning that nothing leaves the premises", card.Plan)
	}
	if o := card.Observed; o.Sites != 1 {
		t.Errorf("observed = %+v, want one site", o)
	}
	if f.domainStatus("containers").OffPremisesCovered {
		t.Error("a direct repository in the house covers nothing off the premises")
	}
}

func TestScoreCallsTwoCopiesOnThePremisesNoOffsiteCopy(t *testing.T) {
	home := observedPlace{Place: "local", State: "counts", Counts: true}
	nas := observedPlace{Place: "offsite:nas", State: "counts", Counts: true}
	b2 := observedPlace{Place: "offsite:b2", State: "unknown", Stale: true}

	o := &placementObserved{Places: []observedPlace{home, nas}}
	o.score(map[string]string{"local": "host", "offsite:nas": "host"})
	if o.Sites != 1 || o.Rule321 != "no-off-site" || o.Tone != "warn" {
		t.Errorf("two copies in the house = %+v, want one site, no-off-site, warn", o)
	}

	o = &placementObserved{Places: []observedPlace{home, nas, b2}}
	o.score(map[string]string{"local": "host", "offsite:nas": "host"})
	if o.Rule321 != "unconfirmed" || o.Tone != "unconfirmed" {
		t.Errorf("two copies in the house and an unconfirmed one off site = %+v, want unconfirmed", o)
	}

	o = &placementObserved{Places: []observedPlace{home, nas}}
	o.score(map[string]string{"local": "host"})
	if o.Sites != 2 || o.Rule321 != "met" {
		t.Errorf("a target without a place = %+v, want a site of its own and 3-2-1 met", o)
	}
}

func TestTwoCopiesInTheHouseAreNoOffsiteCopy(t *testing.T) {
	f := newPlacementFixture(t)
	dailyContainerBackups(f)
	nas := f.target("containers", "NAS", "remotes/nas/bv/containers")
	here := f.storePlace(nasKeller())
	f.linkRow(nas.ID, here, "containers", "")
	f.container("nginx", "")
	now := time.Now().Unix()
	f.listing("containers", nas.ID, now-hour, copiesRow("container:nginx", 20, now-2*hour))

	o := f.cardOf("containers", "nginx", now-2*hour).Observed
	if got := placeAt(o, "offsite:"+nas.ID); !got.Counts {
		t.Fatalf("NAS = %+v, want its copy to count", got)
	}
	if o.Sites != 1 || o.Rule321 != "no-off-site" || o.Tone != "warn" {
		t.Fatalf("observed = %+v, want one site and no copy off the premises", o)
	}

	here.OffPremises = true
	f.storePlace(here)
	if o := f.cardOf("containers", "nginx", now-2*hour).Observed; o.Sites != 2 || o.Rule321 != "met" {
		t.Fatalf("with the NAS at another site = %+v, want two sites and 3-2-1 met", o)
	}
}

func TestAPlanCopiedOnlyInTheHouseWarnsThatNothingLeaves(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.target("containers", "NAS", "remotes/nas/bv/containers")
	here := f.storePlace(nasKeller())
	f.linkRow(nas.ID, here, "containers", "")
	f.container("nginx", "")

	if plan := f.cardOf("containers", "nginx", 0).Plan; len(plan.Targets) != 1 || !plan.NoCopy || !plan.Warn {
		t.Fatalf("plan = %+v, want the NAS named and the warning that nothing leaves the premises", plan)
	}

	here.OffPremises = true
	f.storePlace(here)
	if plan := f.cardOf("containers", "nginx", 0).Plan; plan.NoCopy || plan.Warn {
		t.Fatalf("with the NAS at another site = %+v, want no warning", plan)
	}
}
