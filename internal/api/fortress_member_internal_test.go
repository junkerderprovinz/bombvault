package api

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/status"
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

// joinFromConsole writes the membership `fortress join -dir` writes.
func joinFromConsole(t *testing.T, svc *Service) {
	t.Helper()
	secret := bytes.Repeat([]byte{7}, 16)
	if err := fortress.Join(svc.fortressDir(), secret, fortress.Membership{App: "bombvault", Device: "tower", Relay: "off"}); err != nil {
		t.Fatal(err)
	}
}

func TestAMembershipInTheDataDirectoryOpensTheFortress(t *testing.T) {
	svc, _ := newFortressSvc(t)
	joinFromConsole(t, svc)
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
	if !slices.Equal(cfg.Uses, []string{"backup.export", "backup.import", "updates.managed", "ui.open"}) {
		t.Fatalf("uses %v, want backup.export, backup.import, updates.managed and ui.open once each", cfg.Uses)
	}
}

func TestBombVaultTellsTheFortressItsLogoAndServer(t *testing.T) {
	t.Setenv("HOST_HOSTNAME", "Tower")
	svc, _ := newFortressSvc(t)
	cfg := svc.fortressConfig(fortress.Config{}, nil)
	if err := status.CheckIcon(cfg.App.Icon); err != nil || cfg.Host != "Tower" {
		t.Fatalf("icon %v, host %q, want the logo and Tower", err, cfg.Host)
	}

	svc.cfg.FlashDir = t.TempDir()
	if err := os.MkdirAll(filepath.Join(svc.cfg.FlashDir, "config"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svc.cfg.FlashDir, "config", "ident.cfg"), []byte("NAME=\"Cellar\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if cfg := svc.fortressConfig(fortress.Config{}, nil); cfg.Host != "Cellar" {
		t.Fatalf("host %q, want the name on the flash", cfg.Host)
	}
}
