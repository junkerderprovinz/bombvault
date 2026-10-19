package store_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func derive(t *testing.T, r *store.Repo, d store.OffsiteTarget, domain string) store.OffsiteTarget {
	t.Helper()
	target, _, err := r.EnsureDestinationTarget(d.ID, domain, d.Repo+"/"+domain)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func targetByID(t *testing.T, r *store.Repo, id string) store.OffsiteTarget {
	t.Helper()
	got, ok, err := r.GetOffsiteTarget(id)
	if err != nil || !ok {
		t.Fatalf("target %s: found=%v err=%v", id, ok, err)
	}
	return got
}

// followed is the part of a row a destination can hand down.
type followed struct {
	last, daily, weekly, monthly, yearly int
	compression                          string
	up, down                             int
	enabled                              bool
}

func followedOf(t store.OffsiteTarget) followed {
	return followed{t.RetentionKeepLast, t.RetentionKeepDaily, t.RetentionKeepWeekly, t.RetentionKeepMonthly,
		t.RetentionKeepYearly, t.Compression, t.LimitUpload, t.LimitDownload, t.Enabled}
}

func TestANewDestinationIsSwitchedOnAndOffThePremises(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv"})
	if !d.Enabled || !d.OffPremises {
		t.Fatalf("new destination = %+v, want it enabled and off the premises", d)
	}
	d.Enabled, d.OffPremises = false, false
	if d = saveDestination(t, r, d); d.Enabled || d.OffPremises {
		t.Fatalf("saved destination = %+v, want both switches off", d)
	}
}

func TestADerivedTargetStartsWithItsDestinationsSettings(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv"})
	d.RetentionKeepDaily, d.RetentionKeepYearly, d.Compression, d.LimitUpload = 7, 2, "max", 512
	d = saveDestination(t, r, d)

	got := derive(t, r, d, "flash")
	if followedOf(got) != followedOf(d) || got.Own != 0 {
		t.Fatalf("derived target = %+v, own %d, want the destination's settings and none of its own", followedOf(got), got.Own)
	}
}

func TestSavingADestinationReachesTheTargetsThatFollowIt(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv"})
	follows := derive(t, r, d, "flash")
	own := derive(t, r, d, "vms")
	own.RetentionKeepWeekly, own.LimitDownload = 4, 100
	own, err := r.UpsertOffsiteTarget(own)
	if err != nil {
		t.Fatal(err)
	}
	if own.Own != store.OwnRetention|store.OwnLimits {
		t.Fatalf("own settings = %d, want retention and limits after saving other values for them", own.Own)
	}

	d.RetentionKeepDaily, d.Compression, d.LimitUpload, d.LimitDownload = 14, "off", 256, 64
	d = saveDestination(t, r, d)

	if got := targetByID(t, r, follows.ID); followedOf(got) != followedOf(d) {
		t.Errorf("following target = %+v, want %+v", followedOf(got), followedOf(d))
	}
	got := targetByID(t, r, own.ID)
	if got.RetentionKeepWeekly != 4 || got.RetentionKeepDaily != 0 || got.LimitDownload != 100 || got.LimitUpload != 0 {
		t.Errorf("target with its own keep-policy and limits = %+v, want them untouched", followedOf(got))
	}
	if got.Compression != "off" {
		t.Errorf("compression = %q, want the destination's, which it still follows", got.Compression)
	}
}

func TestATargetSwitchedOffByItselfStaysOffWhenItsDestinationComesBack(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv"})
	follows := derive(t, r, d, "flash")
	off := derive(t, r, d, "vms")
	off.Enabled = false
	if _, err := r.UpsertOffsiteTarget(off); err != nil {
		t.Fatal(err)
	}

	d.Enabled = false
	d = saveDestination(t, r, d)
	if targetByID(t, r, follows.ID).Enabled {
		t.Fatal("the following target still copies to a destination that is switched off")
	}
	d.Enabled = true
	saveDestination(t, r, d)
	if !targetByID(t, r, follows.ID).Enabled {
		t.Error("the following target stayed off after its destination was switched on again")
	}
	if targetByID(t, r, off.ID).Enabled {
		t.Error("a target that was switched off by itself came back with its destination")
	}
}

func TestThePrimaryTakesOnlyItsDestinationsKeepPolicy(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv", RetentionKeepWeekly: 4})
	primary, err := r.PrimaryFromDestination(d.ID, "flash", "rclone:b2:bv/flash", func(s *store.Settings) { s.FlashOffsite = "rclone:b2:bv/flash" })
	if err != nil {
		t.Fatal(err)
	}
	if primary.Own != 0 || primary.RetentionKeepWeekly != 4 {
		t.Fatalf("new primary = %+v, own %d, want the destination's keep-policy", followedOf(primary), primary.Own)
	}
	for _, setting := range store.FollowedSettings {
		if primary.Keeps(setting) == (setting == store.OwnRetention) {
			t.Fatalf("the primary keeps setting %d: %v", setting, primary.Keeps(setting))
		}
	}
	d.RetentionKeepDaily, d.Enabled = 3, false
	saveDestination(t, r, d)
	if got := targetByID(t, r, primary.ID); got.RetentionKeepDaily != 3 || !got.Enabled {
		t.Fatalf("primary = %+v, want the destination's keep-policy and its own switch", followedOf(got))
	}
}

func TestAPrimaryWithItsOwnKeepPolicyKeepsItThroughADestinationSave(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv"})
	primary, err := r.PrimaryFromDestination(d.ID, "flash", "rclone:b2:bv/flash", func(s *store.Settings) { s.FlashOffsite = "rclone:b2:bv/flash" })
	if err != nil {
		t.Fatal(err)
	}
	primary.RetentionKeepMonthly = 12
	if primary, err = r.UpsertOffsiteTarget(primary); err != nil {
		t.Fatal(err)
	}
	if primary.Own&store.OwnRetention == 0 {
		t.Fatalf("own = %d, want a changed keep-policy held as the primary's own", primary.Own)
	}
	d.RetentionKeepMonthly = 2
	saveDestination(t, r, d)
	if got := targetByID(t, r, primary.ID); got.RetentionKeepMonthly != 12 {
		t.Fatalf("primary = %+v, want its own keep-policy", followedOf(got))
	}
}

func TestAnAdoptedTargetKeepsWhatDiffersFromItsDestination(t *testing.T) {
	r := newRepo(t)
	hand, err := r.CreateOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "By hand", Repo: "rclone:b2:bv/vms",
		Enabled: true, RetentionKeepMonthly: 6})
	if err != nil {
		t.Fatal(err)
	}
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv"})
	got, err := r.AdoptIntoDestination(hand.ID, d.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Own != store.OwnRetention || got.RetentionKeepMonthly != 6 {
		t.Fatalf("adopted = %+v, own %d, want its keep-policy kept as its own", followedOf(got), got.Own)
	}
	if err := r.DetachFromDestination(got.ID); err != nil {
		t.Fatal(err)
	}
	if got = targetByID(t, r, got.ID); got.Own != 0 || got.RetentionKeepMonthly != 6 {
		t.Fatalf("detached = %+v, own %d, want its values and nothing left to follow", followedOf(got), got.Own)
	}
}

func TestADestinationsSettingsReachTheDirectRepository(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv"})
	target := derive(t, r, d, "containers")
	direct, err := r.CreateCompanionRepo(target.ID, "", "rclone:b2:bv/containers-direct")
	if err != nil {
		t.Fatal(err)
	}
	d.RetentionKeepDaily, d.Compression, d.LimitUpload = 30, "max", 128
	saveDestination(t, r, d)

	got, err := r.GetNamedRepo(direct.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.MirroredEqual(targetByID(t, r, target.ID)) || got.RetentionKeepDaily != 30 {
		t.Fatalf("direct repository = %+v, want what its target took from the destination", got)
	}
}

func TestATargetGivesASettingBackToItsDestination(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv"})
	d.RetentionKeepDaily = 7
	d = saveDestination(t, r, d)
	target := derive(t, r, d, "flash")
	target.RetentionKeepDaily = 1
	target, err := r.UpsertOffsiteTarget(target)
	if err != nil {
		t.Fatal(err)
	}

	target.TakeFrom(d, store.OwnRetention)
	target.Own &^= store.OwnRetention
	if target, err = r.UpsertOffsiteTarget(target); err != nil {
		t.Fatal(err)
	}
	if target.Own != 0 || target.RetentionKeepDaily != 7 {
		t.Fatalf("target = %+v, own %d, want the destination's keep-policy again", followedOf(target), target.Own)
	}
	d.RetentionKeepDaily = 9
	saveDestination(t, r, d)
	if got := targetByID(t, r, target.ID); got.RetentionKeepDaily != 9 {
		t.Fatalf("keep daily = %d, want it to follow the destination to 9", got.RetentionKeepDaily)
	}
}

func TestAValueOfItsOwnOutlivesADestinationThatCatchesUp(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv"})
	target := derive(t, r, d, "flash")
	target.RetentionKeepDaily = 5
	if _, err := r.UpsertOffsiteTarget(target); err != nil {
		t.Fatal(err)
	}
	for _, daily := range []int{5, 8} {
		d.RetentionKeepDaily = daily
		d = saveDestination(t, r, d)
	}
	if got := targetByID(t, r, target.ID); got.RetentionKeepDaily != 5 {
		t.Fatalf("keep daily = %d, want the target's own 5 after the destination passed through it", got.RetentionKeepDaily)
	}
}

func TestRemoveDestinationTakesItsTargetsAndNamesThePrimariesItHeld(t *testing.T) {
	r := newRepo(t)
	d, err := r.SaveDestination(store.OffsiteTarget{Name: "Box", Repo: "rest:https://box:8000/bv"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := r.SaveDestination(store.OffsiteTarget{Name: "Other", Repo: "rest:https://other:8000/bv"})
	if err != nil {
		t.Fatal(err)
	}
	vms, _, err := r.EnsureDestinationTarget(d.ID, "vms", "rest:https://box:8000/bv/vms")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.PrimaryFromDestination(d.ID, "flash", "rest:https://box:8000/bv/flash", func(*store.Settings) {}); err != nil {
		t.Fatal(err)
	}
	stays, _, err := r.EnsureDestinationTarget(other.ID, "vms", "rest:https://other:8000/bv/vms")
	if err != nil {
		t.Fatal(err)
	}

	var primaries []string
	if err := r.RemoveDestination(d.ID, nothingBehind, func(_ *store.Settings, domains []string) { primaries = domains }); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(primaries, []string{"flash"}) {
		t.Fatalf("primaries = %v, want flash alone", primaries)
	}
	if _, ok, _ := r.GetDestination(d.ID); ok {
		t.Fatal("the destination is still there")
	}
	if _, ok, _ := r.GetOffsiteTarget(vms.ID); ok {
		t.Fatal("its target for vms is still there")
	}
	if flash, _ := r.OffsiteTargetsForDomain("flash"); len(flash) != 0 {
		t.Fatalf("flash still has %+v", flash)
	}
	if _, ok, _ := r.GetOffsiteTarget(stays.ID); !ok {
		t.Fatal("the removal took a target of another destination")
	}
	if err := r.RemoveDestination(d.ID, nothingBehind, nil); !errors.Is(err, store.ErrNotDestination) {
		t.Fatalf("removing it again: %v, want it reported as unknown", err)
	}
}

func nothingBehind(store.OffsiteTarget, store.Settings) bool { return false }
