package api

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// switchedOffPrimaryService is a containers domain whose only off-site target
// is switched off while the off-site setting still names its location, which
// is what creating the target from a destination and then switching it off
// leaves behind.
func switchedOffPrimaryService(t *testing.T) (*Service, store.Settings) {
	t.Helper()
	st := newSourceSeamStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersEnabled = true
	settings.ContainersOffsite = "rest:http://192.168.20.112:38000/bv/containers"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "sandbox-rest", Repo: settings.ContainersOffsite}); err != nil {
		t.Fatal(err)
	}
	return &Service{store: st}, settings
}

func TestADrillRefusesWhenTheOnlyOffsiteTargetIsSwitchedOff(t *testing.T) {
	s, _ := switchedOffPrimaryService(t)

	drill, err := s.runDRDrill(context.Background(), "containers", "offsite", false)
	if !errors.Is(err, errNoOffsiteRepo) {
		t.Fatalf("runDRDrill = %+v, %v; want errNoOffsiteRepo, as the copy answers", drill, err)
	}
	drills, err := s.store.ListRestoreDrills("containers", "offsite", 0)
	if err != nil || len(drills) != 0 {
		t.Fatalf("recorded drills = %+v, %v; want none", drills, err)
	}
}

func TestASwitchedOffTargetCountsAsNoOffsiteRepoEverywhere(t *testing.T) {
	s, settings := switchedOffPrimaryService(t)
	ctx := context.Background()

	if repo := s.offsiteRepoFor("containers", settings); repo != "" {
		t.Errorf("offsiteRepoFor = %q, want none", repo)
	}
	if targets := s.offsiteReplicationTargets("containers", settings); len(targets) != 0 {
		t.Errorf("offsiteReplicationTargets = %+v, want none", targets)
	}
	if _, err := s.offsiteTargetForSource(settings, "containers", "offsite"); !errors.Is(err, errNoOffsiteRepo) {
		t.Errorf("offsiteTargetForSource = %v, want errNoOffsiteRepo", err)
	}
	if _, err := s.repoFor(settings, "containers", "offsite"); !errors.Is(err, errNoOffsiteRepo) {
		t.Errorf("repoFor = %v, want errNoOffsiteRepo", err)
	}
	places, err := s.backupPlaces(settings, "containers", nil)
	if err != nil || len(places) != 0 {
		t.Errorf("backupPlaces = %+v, %v; want no off-site place", places, err)
	}
	if _, _, err := s.TestOffsite(ctx, "containers"); !errors.Is(err, errNoOffsiteRepo) {
		t.Errorf("TestOffsite = %v, want errNoOffsiteRepo", err)
	}
	if err := s.ReplicateOffsite(ctx, "containers"); !errors.Is(err, errNoOffsiteRepo) {
		t.Errorf("ReplicateOffsite = %v, want errNoOffsiteRepo", err)
	}
	if _, err := s.RunTamperTest(ctx, "containers"); !errors.Is(err, errNoOffsiteRepo) {
		t.Errorf("RunTamperTest = %v, want errNoOffsiteRepo", err)
	}
}

func TestTheOffsiteSettingCountsWhileTheDomainHasNoTargetRow(t *testing.T) {
	st := newSourceSeamStore(t)
	s := &Service{store: st}
	settings := store.Settings{ContainersOffsite: "rest:http://192.168.20.112:38000/bv/containers"}

	if repo := s.offsiteRepoFor("containers", settings); repo != settings.ContainersOffsite {
		t.Fatalf("offsiteRepoFor = %q, want the setting %q", repo, settings.ContainersOffsite)
	}
	if targets := s.offsiteReplicationTargets("containers", settings); len(targets) != 1 || targets[0].Repo != settings.ContainersOffsite {
		t.Fatalf("offsiteReplicationTargets = %+v, want the one target the setting describes", targets)
	}
}

func TestSwitchingTheTargetBackOnBringsTheDrillBack(t *testing.T) {
	s, settings := switchedOffPrimaryService(t)
	targets, err := s.store.OffsiteTargetsForDomain("containers")
	if err != nil || len(targets) != 1 {
		t.Fatalf("targets = %+v, %v", targets, err)
	}
	on := targets[0]
	on.Enabled = true
	if _, err := s.store.UpsertOffsiteTarget(on); err != nil {
		t.Fatal(err)
	}

	got, err := s.offsiteTargetForSource(settings, "containers", "offsite")
	if err != nil || got.ID != on.ID {
		t.Fatalf("offsiteTargetForSource = %+v, %v; want the switched-on row %q", got, err, on.ID)
	}
}
