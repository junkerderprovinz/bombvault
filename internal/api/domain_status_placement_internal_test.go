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

// nasKeller is a place in the house holding a folder for the containers.
func nasKeller() store.Place {
	return store.Place{
		Name: "NAS Keller", Provider: "synology", Kind: "local", Base: "remotes/nas/bv",
		Folders: map[string]string{"containers": "containers"}, Enabled: true,
	}
}

func drillsOn(f *placementFixture) {
	f.settings(func(s *store.Settings) {
		s.ContainersEnabled = true
		s.DrillsEnabled = true
		s.OffsiteDrillsEnabled = true
	})
}

func TestATargetOnThePremisesIsNoOffsiteCopy(t *testing.T) {
	f := newPlacementFixture(t)
	drillsOn(f)
	nas := f.target("containers", "NAS", "remotes/nas/bv/containers")
	f.linkRow(nas.ID, f.storePlace(nasKeller()), "containers", "")
	f.container("nginx", "")

	d := f.domainStatus("containers")
	if d.OffsiteConfigured || d.OffsiteDrillScheduled || d.OffPremisesCovered {
		t.Fatalf("configured %v, drill %v, covered %v; want all false, which shows No off-site copy",
			d.OffsiteConfigured, d.OffsiteDrillScheduled, d.OffPremisesCovered)
	}
}

func TestATargetAtAPlaceOffThePremisesIsAnOffsiteCopy(t *testing.T) {
	f := newPlacementFixture(t)
	drillsOn(f)
	nas := f.target("containers", "NAS", "remotes/friend/bv/containers")
	f.linkRow(nas.ID, f.storePlace(store.Place{
		Name: "NAS at a friend's", Provider: "synology", Kind: "local", Base: "remotes/friend/bv",
		Folders: map[string]string{"containers": "containers"}, OffPremises: true, Enabled: true,
	}), "containers", "")
	f.container("nginx", "")

	if d := f.domainStatus("containers"); !d.OffsiteConfigured || !d.OffsiteDrillScheduled {
		t.Fatalf("configured %v, drill %v; want both true", d.OffsiteConfigured, d.OffsiteDrillScheduled)
	}
}

func TestTargetsInTheHouseAndOffItMakeAScheduledOffsiteDrill(t *testing.T) {
	f := newPlacementFixture(t)
	drillsOn(f)
	nas := f.target("containers", "NAS", "remotes/nas/bv/containers")
	f.linkRow(nas.ID, f.storePlace(nasKeller()), "containers", "")
	f.target("containers", "B2", b2Containers)
	f.container("nginx", "")

	if d := f.domainStatus("containers"); !d.OffsiteDrillScheduled {
		t.Fatal("B2 stands off the premises and takes its turn in the drill, so the off-site drill is scheduled")
	}
}

func TestOnlyADomainTheDrillsJobCoversClaimsAScheduledOffsiteDrill(t *testing.T) {
	f := newPlacementFixture(t)
	f.settings(func(s *store.Settings) {
		s.ConfigEnabled = true
		s.FilesEnabled = false
		s.DrillsEnabled = true
		s.OffsiteDrillsEnabled = true
	})
	f.target("config", "B2", "b2:bucket:config")
	f.target("files", "B2", "b2:bucket:files")

	for _, domain := range []string{"config", "files"} {
		if d := f.domainStatus(domain); !d.OffsiteConfigured || d.OffsiteDrillScheduled {
			t.Errorf("%s: configured %v, drill %v; want an off-site copy and no scheduled drill", domain, d.OffsiteConfigured, d.OffsiteDrillScheduled)
		}
	}
}

func TestOnlyCopiesToAnOffPremisesTargetMakeTheDomainConfigured(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.target("containers", "NAS", "remotes/nas/bv/containers")
	b2 := f.target("containers", "B2", b2Containers)
	f.linkRow(nas.ID, f.storePlace(nasKeller()), "containers", "")
	f.linkRow(b2.ID, f.storePlace(b2Place(0)), "containers", "")
	f.container("nginx", "")
	f.rule("containers", "container:nginx", b2.ID)

	if f.domainStatus("containers").OffsiteConfigured {
		t.Fatal("every item leaves B2 out and is copied only to the NAS in the house, so nothing is off site")
	}
}

func TestTheStatusNamesTheTargetOfTheLastDRDrill(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("flash", "B2", "s3:b2/flash")
	hetzner := f.target("flash", "Hetzner", "sftp:u1@hetzner:/flash")
	if err := f.st.AddRestoreDrill(store.RestoreDrill{Domain: "flash", Source: "offsite", Kind: "dr", At: 100, OK: true, TargetID: hetzner.ID}); err != nil {
		t.Fatal(err)
	}
	if got := f.domainStatus("flash").DrillTarget; got != "Hetzner" {
		t.Fatalf("drill target %q, want Hetzner", got)
	}

	if _, err := f.st.DeleteOffsiteTargetIfUnused(hetzner.ID); err != nil {
		t.Fatal(err)
	}
	if got := f.domainStatus("flash").DrillTarget; got != "" {
		t.Fatalf("drill target %q after the target was removed, want none", got)
	}
}

func TestAProjectFolderCopiedOnlyInTheHouseIsNoOffsiteCopy(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	nas := f.target("containers", "NAS", "remotes/nas/bv/containers")
	here := f.storePlace(nasKeller())
	f.linkRow(nas.ID, here, "containers", "")
	f.container("nginx", "")
	f.rule("containers", "container:nginx", store.SkipAll)
	f.listing("containers", nas.ID, 1_758_000_000, copiesRow("stack:immich", 3, 1_757_900_000))

	if f.domainStatus("containers").OffsiteConfigured {
		t.Fatal("a project folder copied only to the NAS in the house makes nothing off site")
	}

	here.OffPremises = true
	f.storePlace(here)
	if !f.domainStatus("containers").OffsiteConfigured {
		t.Fatal("with the NAS at another site, the project folder's copy there is off site")
	}
}
