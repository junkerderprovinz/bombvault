package api

import (
	"testing"

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
