package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestOffsiteTargetCRUD(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// Upsert with an empty ID assigns one and stamps created_at.
	in := store.OffsiteTarget{
		Domain:               "containers",
		Name:                 "Primary",
		Repo:                 "s3:https://example/containers",
		Immutable:            true,
		Schedule:             "daily 03:00",
		RetentionKeepLast:    7,
		RetentionKeepDaily:   14,
		RetentionKeepWeekly:  8,
		RetentionKeepMonthly: 12,
		LimitUpload:          1000,
		LimitDownload:        2000,
		GrowthBudgetGB:       50,
		Enabled:              true,
	}
	got, err := r.UpsertOffsiteTarget(in)
	if err != nil {
		t.Fatalf("UpsertOffsiteTarget: %v", err)
	}
	if got.ID == "" {
		t.Fatal("UpsertOffsiteTarget did not assign an ID")
	}
	if got.CreatedAt == 0 {
		t.Fatal("UpsertOffsiteTarget did not stamp CreatedAt")
	}

	// Get round-trips every field.
	back, ok, err := r.GetOffsiteTarget(got.ID)
	if err != nil || !ok {
		t.Fatalf("GetOffsiteTarget: ok=%v err=%v", ok, err)
	}
	if back.Domain != "containers" || back.Name != "Primary" || back.Repo != in.Repo ||
		!back.Immutable || back.Schedule != "daily 03:00" ||
		back.RetentionKeepLast != 7 || back.RetentionKeepDaily != 14 ||
		back.RetentionKeepWeekly != 8 || back.RetentionKeepMonthly != 12 ||
		back.LimitUpload != 1000 || back.LimitDownload != 2000 ||
		back.GrowthBudgetGB != 50 || !back.Enabled {
		t.Fatalf("GetOffsiteTarget round-trip mismatch: %+v", back)
	}

	// Upsert with the same ID updates in place (no new row).
	back.Repo = "s3:https://example/containers-2"
	back.Enabled = false
	if _, err := r.UpsertOffsiteTarget(back); err != nil {
		t.Fatalf("UpsertOffsiteTarget update: %v", err)
	}
	upd, _, err := r.GetOffsiteTarget(got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if upd.Repo != "s3:https://example/containers-2" || upd.Enabled {
		t.Fatalf("in-place update failed: %+v", upd)
	}

	// A second target in another domain, plus a second containers target with a
	// later sort_order, exercises the List ordering (domain, sort_order, created_at).
	if _, err := r.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "Primary", Repo: "s3:vms", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Second", Repo: "s3:c2", Enabled: true, SortOrder: 5}); err != nil {
		t.Fatal(err)
	}

	all, err := r.ListOffsiteTargets()
	if err != nil {
		t.Fatalf("ListOffsiteTargets: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("ListOffsiteTargets len = %d, want 3", len(all))
	}
	// containers (sort_order 0) < containers (sort_order 5) < vms.
	if all[0].Domain != "containers" || all[0].SortOrder != 0 ||
		all[1].Domain != "containers" || all[1].SortOrder != 5 ||
		all[2].Domain != "vms" {
		t.Fatalf("ListOffsiteTargets order wrong: %+v", all)
	}

	dom, err := r.OffsiteTargetsForDomain("containers")
	if err != nil {
		t.Fatalf("OffsiteTargetsForDomain: %v", err)
	}
	if len(dom) != 2 {
		t.Fatalf("OffsiteTargetsForDomain(containers) len = %d, want 2", len(dom))
	}

	// Delete removes it; a second delete is a no-op.
	if err := r.DeleteOffsiteTarget(got.ID); err != nil {
		t.Fatalf("DeleteOffsiteTarget: %v", err)
	}
	if _, ok, _ := r.GetOffsiteTarget(got.ID); ok {
		t.Fatal("target still present after delete")
	}
	if err := r.DeleteOffsiteTarget(got.ID); err != nil {
		t.Fatalf("DeleteOffsiteTarget (missing) should be a no-op: %v", err)
	}

	// Get on a missing id returns ok=false, no error.
	if _, ok, err := r.GetOffsiteTarget("does-not-exist"); ok || err != nil {
		t.Fatalf("GetOffsiteTarget(missing) = ok:%v err:%v", ok, err)
	}
}
