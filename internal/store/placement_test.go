package store_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestADomainWithoutADefaultCopiesEverywhereAndIsNotPaused(t *testing.T) {
	r := newRepo(t)
	st, err := r.ReadPlacement("containers")
	if err != nil {
		t.Fatal(err)
	}
	if st.HasDefault || st.Paused() || st.HasRules() || st.Rules == nil {
		t.Fatalf("ReadPlacement = %+v, want no default, no pause, no rules and an empty map", st)
	}
}

func TestAPutDefaultIsConfirmedAndKeepsAPauseItFinds(t *testing.T) {
	r := newRepo(t)
	d := store.SeedDefault(t, r, "vms", "repo-1", "t2")
	if d.Paused() || d.Home != "repo-1" || !slices.Equal(d.Skip, []string{"t2"}) {
		t.Fatalf("new default = %+v, want it confirmed with home and skip", d)
	}
	if started, err := r.PausePlacement("vms"); err != nil || !started {
		t.Fatalf("PausePlacement = %v, %v, want this call to start the pause", started, err)
	}
	d, err := r.PutPlacementDefault("vms", "", nil)
	if err != nil || !d.Paused() || d.Home != "" || len(d.Skip) != 0 {
		t.Fatalf("PutPlacementDefault on a paused domain = %+v, %v, want the pause kept", d, err)
	}
}

func TestOnlyTheCallThatStartsAPauseSaysSo(t *testing.T) {
	r := newRepo(t)
	if first, err := r.PausePlacement("files"); err != nil || !first {
		t.Fatalf("first PausePlacement = %v, %v", first, err)
	}
	if again, err := r.PausePlacement("files"); err != nil || again {
		t.Fatalf("second PausePlacement = %v, %v, want no new pause", again, err)
	}
	d, found, err := r.PlacementDefaultFor("files")
	if err != nil || !found || !d.Paused() || d.Home != "" || len(d.Skip) != 0 {
		t.Fatalf("PlacementDefaultFor = %+v found=%v err=%v, want a paused row on the domain path", d, found, err)
	}
	if _, err := r.PausePlacement("flash"); !errors.Is(err, store.ErrUnknownDomain) {
		t.Errorf("PausePlacement(flash) = %v, want ErrUnknownDomain", err)
	}
}

func TestConfirmingLeavesTheNamedItemsOutAndEndsThePause(t *testing.T) {
	r := newRepo(t)
	if _, err := r.PausePlacement("containers"); err != nil {
		t.Fatal(err)
	}
	if err := r.ConfirmPlacement("containers", []string{"container:old-app"}); err != nil {
		t.Fatalf("ConfirmPlacement: %v", err)
	}
	st, err := r.ReadPlacement("containers")
	if err != nil || st.Paused() || !slices.Equal(st.Rules["container:old-app"].Skip, []string{store.SkipAll}) {
		t.Fatalf("after confirming: %+v, %v", st, err)
	}
	if err := r.ConfirmPlacement("containers", []string{"stack:immich"}); !errors.Is(err, store.ErrStackCopyRule) {
		t.Fatalf("confirming with a project folder left out = %v, want ErrStackCopyRule", err)
	}
	if defaults, err := r.ListPlacementDefaults(); err != nil || len(defaults) != 1 || defaults[0].Domain != "containers" {
		t.Fatalf("ListPlacementDefaults = %v, %v", defaults, err)
	}
}

func TestHasRulesMeansSomethingIsKeptFromATarget(t *testing.T) {
	r := newRepo(t)
	store.SeedDefault(t, r, "containers", "")
	if st, _ := r.ReadPlacement("containers"); st.HasRules() {
		t.Error("a default that copies everywhere is no rule")
	}
	store.SeedDefault(t, r, "containers", "", "t1")
	if st, _ := r.ReadPlacement("containers"); !st.HasRules() {
		t.Error("a default leaving t1 out is a rule")
	}
	store.SeedCopyRule(t, r, "vms", "vm:win11")
	if st, _ := r.ReadPlacement("vms"); !st.HasRules() {
		t.Error("an item's own rule is a rule, even one copying everywhere")
	}
}

func TestDomainHistorySeesBackupsAndReplicationsOfItsDomainOnly(t *testing.T) {
	r := newRepo(t)
	tg, err := r.UpsertTarget(store.Target{ContainerName: "nginx"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := r.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishRun(run, "success", "", 0, ""); err != nil {
		t.Fatal(err)
	}
	copyRun, err := r.RecordOffsiteRun("vms", 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishOffsiteRun(copyRun, true, ""); err != nil {
		t.Fatal(err)
	}
	if backup, offsite, err := r.DomainHasHistory("containers"); err != nil || !backup || offsite {
		t.Errorf("containers: backup=%v offsite=%v err=%v, want a backup and no replication", backup, offsite, err)
	}
	if backup, offsite, err := r.DomainHasHistory("vms"); err != nil || backup || !offsite {
		t.Errorf("vms: backup=%v offsite=%v err=%v, want a replication and no backup", backup, offsite, err)
	}
	if _, _, err := r.DomainHasHistory("flash"); err == nil {
		t.Error("flash has no placement history to ask about")
	}
}

func TestPutPlacementDefaultDoesNotConfirmManually(t *testing.T) {
	r := newRepo(t)
	d := store.SeedDefault(t, r, "vms", "repo-1")
	if d.ConfirmedManually {
		t.Fatalf("a saved default is manually confirmed: %+v", d)
	}
	if d, _, err := r.PlacementDefaultFor("vms"); err != nil || d.ConfirmedManually {
		t.Fatalf("PlacementDefaultFor = %+v, %v, want the marker unset", d, err)
	}
}

// TestSetExcludeRulesRefusesAnUnknownDomain pins the guard its sibling
// ConfirmPlacement already has: with nothing to exclude, the loop that writes
// each rule runs zero times, so without the guard an unknown domain silently
// succeeds instead of naming the problem.
func TestSetExcludeRulesRefusesAnUnknownDomain(t *testing.T) {
	r := newRepo(t)
	if err := r.SetExcludeRules("flash", nil); !errors.Is(err, store.ErrUnknownDomain) {
		t.Errorf("SetExcludeRules(flash, nil) = %v, want ErrUnknownDomain", err)
	}
}

func TestConfirmPlacementSetsTheManualMarker(t *testing.T) {
	r := newRepo(t)
	if _, err := r.PausePlacement("containers"); err != nil {
		t.Fatal(err)
	}
	if err := r.ConfirmPlacement("containers", nil); err != nil {
		t.Fatal(err)
	}
	d, found, err := r.PlacementDefaultFor("containers")
	if err != nil || !found || !d.ConfirmedManually {
		t.Fatalf("PlacementDefaultFor after ConfirmPlacement = %+v, %v, %v, want the manual marker set", d, found, err)
	}
}
