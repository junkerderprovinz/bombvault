package api

import (
	"bytes"
	"slices"
	"testing"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
)

func TestWithoutAMembershipBombVaultIsInNoFortress(t *testing.T) {
	svc, _ := newFortressSvc(t)
	if err := svc.OpenFortress(nil); err != nil {
		t.Fatal(err)
	}
	if svc.fortressNow() != nil {
		t.Fatal("a fortress opened without a membership")
	}
}

func TestAMembershipInTheDataDirectoryOpensTheFortress(t *testing.T) {
	svc, _ := newFortressSvc(t)
	secret := bytes.Repeat([]byte{7}, 16)
	if err := fortress.Join(svc.fortressDir(), secret, fortress.Membership{App: "bombvault", Device: "tower", Relay: "off"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.OpenFortress(nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.CloseFortress)
	if svc.fortressNow() == nil {
		t.Fatal("the membership did not open")
	}
	svc.CloseFortress()
	if svc.fortressNow() != nil {
		t.Fatal("the fortress stayed open after closing")
	}
}

func TestBombVaultOffersItsActionsAndFetchesBackups(t *testing.T) {
	svc, _ := newFortressSvc(t)
	cfg := svc.fortressConfig(fortress.Config{Uses: []string{"backup.export"}}, nil)
	var names []string
	for _, a := range cfg.Actions {
		names = append(names, a.Name)
	}
	if !slices.Equal(names, []string{"backup.last", "backup.request", "snapshots.browse"}) {
		t.Fatalf("actions %v", names)
	}
	if cfg.App.ID != "bombvault" || cfg.Self.Deployment != "container" {
		t.Fatalf("app %+v, self %+v", cfg.App, cfg.Self)
	}
	if !slices.Equal(cfg.Uses, []string{"backup.export"}) {
		t.Fatalf("uses %v, want backup.export once", cfg.Uses)
	}
}
