package store_test

import (
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

func TestThePrimaryNeverTakesItsDestinationsSettings(t *testing.T) {
	r := newRepo(t)
	d := saveDestination(t, r, store.OffsiteTarget{Name: "B2", Repo: "rclone:b2:bv"})
	primary, err := r.PrimaryFromDestination(d.ID, "flash", "rclone:b2:bv/flash", func(s *store.Settings) { s.FlashOffsite = "rclone:b2:bv/flash" })
	if err != nil {
		t.Fatal(err)
	}
	for _, setting := range store.FollowedSettings {
		if !primary.Keeps(setting) {
			t.Fatalf("the primary follows its destination for setting %d", setting)
		}
	}
	d.RetentionKeepDaily, d.Enabled = 3, false
	saveDestination(t, r, d)
	if got := targetByID(t, r, primary.ID); got.RetentionKeepDaily != 0 || !got.Enabled {
		t.Fatalf("primary = %+v, want the values the domain's off-site settings gave it", followedOf(got))
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
