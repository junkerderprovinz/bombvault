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

// The scheduled DR drill takes a domain's targets in turn, so the newest drill
// speaks for one target only.
func TestTheDRDrillVerdictIsTheWorstOfEachTargetsLatestDrill(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("flash", "B2", "s3:b2/flash")
	hetzner := f.target("flash", "Hetzner", "sftp:u1@hetzner:/flash")
	drill := func(target string, at int64, detail string) {
		t.Helper()
		d := store.RestoreDrill{Domain: "flash", Source: "offsite", Kind: "dr", At: at, OK: detail == "", Detail: detail, TargetID: target}
		if err := f.st.AddRestoreDrill(d); err != nil {
			t.Fatal(err)
		}
	}
	drill(b2.ID, 100, "verification mismatch")
	drill(hetzner.ID, 200, "")

	d := f.domainStatus("flash")
	if d.LastDRDrillOK || d.LastDRDrillAt != 100 || d.DrillTarget != "B2" || d.DrillDetail != "verification mismatch" {
		t.Fatalf("drill = ok %v at %d on %q (%q), want the B2 failure at 100", d.LastDRDrillOK, d.LastDRDrillAt, d.DrillTarget, d.DrillDetail)
	}

	drill(b2.ID, 300, "")
	if d := f.domainStatus("flash"); !d.LastDRDrillOK || d.LastDRDrillAt != 300 || d.DrillTarget != "B2" {
		t.Fatalf("drill = ok %v at %d on %q, want the passing B2 drill at 300", d.LastDRDrillOK, d.LastDRDrillAt, d.DrillTarget)
	}
}

func TestADRDrillOfASwitchedOffTargetDoesNotSetTheVerdict(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("flash", "B2", "s3:b2/flash")
	hetzner := f.target("flash", "Hetzner", "sftp:u1@hetzner:/flash")
	for _, d := range []store.RestoreDrill{
		{Domain: "flash", Source: "offsite", Kind: "dr", At: 100, Detail: "verification mismatch", TargetID: b2.ID},
		{Domain: "flash", Source: "offsite", Kind: "dr", At: 200, OK: true, TargetID: hetzner.ID},
	} {
		if err := f.st.AddRestoreDrill(d); err != nil {
			t.Fatal(err)
		}
	}
	b2.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(b2); err != nil {
		t.Fatal(err)
	}

	if d := f.domainStatus("flash"); !d.LastDRDrillOK || d.DrillTarget != "Hetzner" {
		t.Fatalf("drill = ok %v on %q, want the Hetzner drill", d.LastDRDrillOK, d.DrillTarget)
	}
}

func TestADRDrillOfATargetInTheHouseProvesNothingOffSite(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.target("containers", "NAS", "remotes/nas/bv/containers")
	f.linkRow(nas.ID, f.storePlace(nasKeller()), "containers", "")
	b2 := f.target("containers", "B2", b2Containers)
	if err := f.st.AddRestoreDrill(store.RestoreDrill{Domain: "containers", Source: "offsite", Kind: "dr", At: 100, OK: true, TargetID: nas.ID}); err != nil {
		t.Fatal(err)
	}
	if d := f.domainStatus("containers"); d.LastDRDrillAt != 0 || d.DrillTarget != "" {
		t.Fatalf("drill at %d on %q, want none: only the NAS in the house was drilled", d.LastDRDrillAt, d.DrillTarget)
	}

	if err := f.st.AddRestoreDrill(store.RestoreDrill{Domain: "containers", Source: "offsite", Kind: "dr", At: 50, OK: true, TargetID: b2.ID}); err != nil {
		t.Fatal(err)
	}
	if d := f.domainStatus("containers"); d.LastDRDrillAt != 50 || d.DrillTarget != "B2" {
		t.Fatalf("drill at %d on %q, want the B2 drill at 50", d.LastDRDrillAt, d.DrillTarget)
	}
}

func TestSwitchingOffTheFailingTargetLeavesTheOtherTargetsVerdict(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("flash", "B2", "s3:b2/flash")
	hetzner := f.target("flash", "Hetzner", "sftp:u1@hetzner:/flash")
	for _, d := range []store.RestoreDrill{
		{Domain: "flash", Source: "offsite", Kind: "dr", At: 200, OK: true, TargetID: hetzner.ID},
		{Domain: "flash", Source: "offsite", Kind: "dr", At: 300, Detail: "verification mismatch", TargetID: b2.ID},
	} {
		if err := f.st.AddRestoreDrill(d); err != nil {
			t.Fatal(err)
		}
	}
	b2.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(b2); err != nil {
		t.Fatal(err)
	}
	if d := f.domainStatus("flash"); !d.LastDRDrillOK || d.LastDRDrillAt != 200 || d.DrillTarget != "Hetzner" {
		t.Fatalf("drill = ok %v at %d on %q, want the Hetzner drill", d.LastDRDrillOK, d.LastDRDrillAt, d.DrillTarget)
	}

	if _, err := f.st.DeleteOffsiteTargetIfUnused(b2.ID); err != nil {
		t.Fatal(err)
	}
	if d := f.domainStatus("flash"); !d.LastDRDrillOK || d.DrillTarget != "Hetzner" {
		t.Fatalf("drill = ok %v on %q after B2 was removed, want the Hetzner drill", d.LastDRDrillOK, d.DrillTarget)
	}
}

func TestADrillFromBeforeTargetsStillCountsWithoutOne(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.st.AddRestoreDrill(store.RestoreDrill{Domain: "flash", Source: "offsite", Kind: "dr", At: 100, OK: true}); err != nil {
		t.Fatal(err)
	}
	if d := f.domainStatus("flash"); !d.LastDRDrillOK || d.LastDRDrillAt != 100 {
		t.Fatalf("drill = ok %v at %d, want the drill at 100", d.LastDRDrillOK, d.LastDRDrillAt)
	}
}

func TestTheScheduledDRDrillTakesOnlyTargetsOffThePremises(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.target("containers", "NAS", "remotes/nas/bv/containers")
	f.linkRow(nas.ID, f.storePlace(nasKeller()), "containers", "")
	b2 := f.target("containers", "B2", b2Containers)

	got, err := f.svc.DRDrillTargets("containers")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != b2.ID {
		t.Fatalf("drill targets = %+v, want B2 alone", got)
	}
}

func TestARunOffSiteDRDrillPicksATargetOffThePremises(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.target("containers", "NAS", "remotes/nas/bv/containers")
	f.linkRow(nas.ID, f.storePlace(nasKeller()), "containers", "")
	b2 := f.target("containers", "B2", b2Containers)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	got, err := f.svc.drDrillTarget(settings, "containers", "offsite")
	if err != nil || got.ID != b2.ID {
		t.Fatalf("target = %+v, %v, want B2", got, err)
	}
	if got, err := f.svc.drDrillTarget(settings, "containers", "offsite:"+nas.ID); err != nil || got.ID != nas.ID {
		t.Fatalf("target = %+v, %v, want the NAS it was asked for", got, err)
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

func TestThePruneStrategyComesFromTheCopyTargets(t *testing.T) {
	f := newPlacementFixture(t)
	f.settings(func(s *store.Settings) { s.OffsiteRetentionKeepDaily = 7 })
	garage := f.storePlace(restPlace("Garage", "http://garage:8000"))
	garage.Immutable = false
	garage = f.storePlace(garage)
	f.placeTarget(garage, "containers", "")
	if f.domainStatus("containers").PruneStrategySet {
		t.Fatal("a copy place that keeps everything counts as a prune strategy through the old global keep-policy")
	}

	f.settings(func(s *store.Settings) { s.OffsiteRetentionKeepDaily = 0 })
	garage.RetentionKeepDaily = 7
	f.storePlace(garage)
	if !f.domainStatus("containers").PruneStrategySet {
		t.Fatal("a copy place keeping 7 daily snapshots does not count as a prune strategy")
	}

	friend := f.storePlace(s3Place("Friend", "s3:https://s3.example.com/friend"))
	f.placeTarget(friend, "containers", "")
	if f.domainStatus("containers").PruneStrategySet {
		t.Fatal("a second copy place that keeps everything is covered by the first place's keep-policy")
	}
	friend.GrowthBudgetGB = 500
	f.storePlace(friend)
	if !f.domainStatus("containers").PruneStrategySet {
		t.Fatal("a copy place with a growth budget does not count as a prune strategy")
	}
}
