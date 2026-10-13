package api

import (
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestTwoCopiesOnTheServerAreNo321(t *testing.T) {
	f := newPlacementFixture(t)
	dailyContainerBackups(f)
	f.settings(func(s *store.Settings) { s.ContainersEnabled = true })
	near := f.target("containers", "Offsite", "/mnt/user/offsite/containers")
	f.container("nginx", "")
	now := time.Now().Unix()
	f.listing("containers", near.ID, now-hour, copiesRow("container:nginx", 20, now-2*hour))

	o := f.cardOf("containers", "nginx", now-2*hour).Observed
	if got := placeAt(o, "offsite:"+near.ID); got.State != "counts" {
		t.Fatalf("the copy on the server = %+v, want it counted as a copy", got)
	}
	if o.Sites != 1 || o.Rule321 != "on-premises" || o.Tone != "warn" {
		t.Fatalf("observed = %+v, want one site and no 3-2-1", o)
	}
	if f.domainStatus("containers").OffPremisesCovered {
		t.Fatal("the status says the containers are covered off the premises by a folder on the server")
	}
}

func TestADestinationDecidesWhetherItsCopiesStandOffThePremises(t *testing.T) {
	f := newPlacementFixture(t)
	dailyContainerBackups(f)
	f.settings(func(s *store.Settings) { s.ContainersEnabled = true })
	dest, err := f.st.SaveDestination(store.OffsiteTarget{Name: "Keller", Repo: "rest:http://nas.lan:8000/bv"})
	if err != nil {
		t.Fatal(err)
	}
	target, _, err := f.st.EnsureDestinationTarget(dest.ID, "containers", "rest:http://nas.lan:8000/bv/containers")
	if err != nil {
		t.Fatal(err)
	}
	f.container("nginx", "")
	f.rule("containers", "container:nginx")
	now := time.Now().Unix()
	f.listing("containers", target.ID, now-hour, copiesRow("container:nginx", 20, now-2*hour))

	if o := f.cardOf("containers", "nginx", now-2*hour).Observed; o.Sites != 2 || o.Rule321 != "met" {
		t.Fatalf("a destination off the premises: observed = %+v, want two sites, 3-2-1 met", o)
	}
	if !f.domainStatus("containers").OffPremisesCovered {
		t.Fatal("the status does not count the copy at a destination off the premises")
	}

	dest.OffPremises = false
	if _, err := f.st.SaveDestination(dest); err != nil {
		t.Fatal(err)
	}
	if o := f.cardOf("containers", "nginx", now-2*hour).Observed; o.Sites != 1 || o.Rule321 != "on-premises" {
		t.Fatalf("a destination in the same house: observed = %+v, want one site and no 3-2-1", o)
	}
	if f.domainStatus("containers").OffPremisesCovered {
		t.Fatal("the status counts a destination in the same house as off the premises")
	}
}

func TestAZFSCopyToAFolderOnTheServerIsNo321(t *testing.T) {
	for _, c := range []struct {
		location string
		rule     string
		sites    int
		covered  bool
	}{
		{"/mnt/user/offsite/zfs", "on-premises", 1, false},
		{"b2:bucket:zfs", "met", 2, true},
	} {
		t.Run(c.location, func(t *testing.T) {
			f := newPlacementFixture(t)
			f.settings(func(s *store.Settings) {
				s.ZFSEnabled = true
				s.ZFSSchedule = "daily 03:00"
			})
			item, _ := f.zfsItem("cache/appdata")
			f.target(zfsDomain, "Offsite", c.location)
			f.replicated(zfsDomain)
			settings, err := f.st.GetSettings()
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().Unix()

			if sites, rule := f.svc.zfsItemSites(now, item, settings, now); rule != c.rule || sites != c.sites {
				t.Errorf("a backup and a current copy at %s: %d sites, %s; want %d, %s", c.location, sites, rule, c.sites, c.rule)
			}
			if got := f.domainStatus(zfsDomain).OffPremisesCovered; got != c.covered {
				t.Errorf("offPremisesCovered = %v with the copy at %s, want %v", got, c.location, c.covered)
			}
		})
	}
}
