package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func (f *placementFixture) domainStatus(domain string) DomainStatusEntry {
	f.t.Helper()
	all, err := f.svc.DomainStatus()
	if err != nil {
		f.t.Fatal(err)
	}
	for _, d := range all {
		if d.Domain == domain {
			return d
		}
	}
	f.t.Fatalf("no status for %s", domain)
	return DomainStatusEntry{}
}

func TestOffsiteConfiguredWithoutRulesIsAsBefore(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	if !f.domainStatus("containers").OffsiteConfigured {
		t.Fatal("a domain with a target and no copy rules reads as configured, as it always did")
	}
}

func TestOffsiteConfiguredNeedsSomethingThatIsCopied(t *testing.T) {
	f := newPlacementFixture(t)
	f.settings(func(s *store.Settings) { s.ContainersEnabled = true })
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", "")
	f.rule("containers", "container:nginx", "*")
	d := f.domainStatus("containers")
	if d.OffsiteConfigured || d.Protection != "red" {
		t.Fatalf("every item on Local: configured %v, protection %q; want false and red", d.OffsiteConfigured, d.Protection)
	}

	f.listing("containers", b2.ID, 1_758_000_000, copiesRow("stack:immich", 3, 1_757_900_000))
	if !f.domainStatus("containers").OffsiteConfigured {
		t.Fatal("a project folder that is still copied to B2 makes the domain configured")
	}
}

func TestAMarkedNASWithoutATargetClaimsNoOffsiteCopyAndNoDrill(t *testing.T) {
	f := newPlacementFixture(t)
	f.settings(func(s *store.Settings) {
		s.DrillsEnabled = true
		s.OffsiteDrillsEnabled = true
	})
	nas := f.namedRepo("NAS Keller", "sftp:u@nas:/bv")
	f.offPremises(nas.ID)
	f.container("nginx", nas.ID)

	d := f.domainStatus("containers")
	if !d.OffPremisesCovered || d.OffsiteConfigured || d.OffsiteDrillScheduled {
		t.Fatalf("covered %v, configured %v, drill %v; want true, false, false", d.OffPremisesCovered, d.OffsiteConfigured, d.OffsiteDrillScheduled)
	}

	f.container("plex", "")
	if f.domainStatus("containers").OffPremisesCovered {
		t.Fatal("an item on the domain path is on the premises")
	}
}
