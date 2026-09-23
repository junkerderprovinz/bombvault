package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestOffsiteTargetRoleDefaultsToOffsite expects a row upserted without a Role
// to be stored as RoleOffsite, so it shows up in the off-site queries.
func TestOffsiteTargetRoleDefaultsToOffsite(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	got, err := r.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Primary", Repo: "s3:bucket", Enabled: true})
	if err != nil {
		t.Fatalf("UpsertOffsiteTarget: %v", err)
	}
	if got.Role != store.RoleOffsite {
		t.Fatalf("Role = %q, want %q", got.Role, store.RoleOffsite)
	}
	back, ok, err := r.GetOffsiteTarget(got.ID)
	if err != nil || !ok {
		t.Fatalf("GetOffsiteTarget: ok=%v err=%v", ok, err)
	}
	if back.Role != store.RoleOffsite {
		t.Fatalf("stored Role = %q, want %q", back.Role, store.RoleOffsite)
	}
}

// TestPrimaryRemoteTargetIsolatedFromOffsiteQueries covers a domain's primary
// row (remote-primary safety settings), which shares the table with off-site
// targets but is not a replication destination. The off-site queries must not
// see it; PrimaryRemoteTarget finds it by domain.
func TestPrimaryRemoteTargetIsolatedFromOffsiteQueries(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	offsiteTgt, err := r.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Backblaze", Repo: "b2:bucket:path", Enabled: true})
	if err != nil {
		t.Fatalf("UpsertOffsiteTarget: %v", err)
	}
	primaryTgt, err := r.UpsertPrimaryRemoteTarget("containers", store.OffsiteTarget{
		Repo: "s3:bucket/containers", Immutable: true, LimitUpload: 500, LimitDownload: 250, GrowthBudgetGB: 100, Enabled: true,
	})
	if err != nil {
		t.Fatalf("UpsertPrimaryRemoteTarget: %v", err)
	}
	if primaryTgt.Role != store.RolePrimary {
		t.Fatalf("Role = %q, want %q", primaryTgt.Role, store.RolePrimary)
	}
	if primaryTgt.Domain != "containers" {
		t.Fatalf("Domain = %q, want containers", primaryTgt.Domain)
	}
	if primaryTgt.ID == offsiteTgt.ID {
		t.Fatal("primary row must not share the off-site row's id")
	}

	all, err := r.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].ID != offsiteTgt.ID {
		t.Fatalf("ListOffsiteTargets leaked the primary row: %+v", all)
	}
	dom, err := r.OffsiteTargetsForDomain("containers")
	if err != nil {
		t.Fatal(err)
	}
	if len(dom) != 1 || dom[0].ID != offsiteTgt.ID {
		t.Fatalf("OffsiteTargetsForDomain leaked the primary row: %+v", dom)
	}

	if _, ok, _ := r.GetOffsiteTarget(primaryTgt.ID); ok {
		t.Fatal("GetOffsiteTarget must not resolve a \"primary\"-role id")
	}
	if err := r.DeleteOffsiteTarget(primaryTgt.ID); err != nil {
		t.Fatalf("DeleteOffsiteTarget(primary id) should be a no-op, got err: %v", err)
	}
	if _, ok, err := r.PrimaryRemoteTarget("containers"); err != nil || !ok {
		t.Fatalf("primary row was deleted via DeleteOffsiteTarget; isolation broken (ok=%v err=%v)", ok, err)
	}

	back, ok, err := r.PrimaryRemoteTarget("containers")
	if err != nil || !ok {
		t.Fatalf("PrimaryRemoteTarget: ok=%v err=%v", ok, err)
	}
	if !back.Immutable || back.LimitUpload != 500 || back.LimitDownload != 250 || back.GrowthBudgetGB != 100 {
		t.Fatalf("PrimaryRemoteTarget round-trip mismatch: %+v", back)
	}

	if _, ok, err := r.PrimaryRemoteTarget("vms"); err != nil || ok {
		t.Fatalf("PrimaryRemoteTarget(unconfigured domain) = ok:%v err:%v, want ok:false err:nil", ok, err)
	}
}

// TestUpsertPrimaryRemoteTargetUpdatesInPlace expects a second upsert for the
// same domain to update the existing row and keep its id and created_at.
func TestUpsertPrimaryRemoteTargetUpdatesInPlace(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	first, err := r.UpsertPrimaryRemoteTarget("vms", store.OffsiteTarget{Repo: "s3:vms-1", LimitUpload: 100, Enabled: true})
	if err != nil {
		t.Fatalf("UpsertPrimaryRemoteTarget: %v", err)
	}
	second, err := r.UpsertPrimaryRemoteTarget("vms", store.OffsiteTarget{Repo: "s3:vms-2", LimitUpload: 200, Immutable: true, Enabled: true})
	if err != nil {
		t.Fatalf("UpsertPrimaryRemoteTarget (update): %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("update created a new row: first.ID=%q second.ID=%q", first.ID, second.ID)
	}
	if second.CreatedAt != first.CreatedAt {
		t.Fatalf("update re-stamped CreatedAt: first=%d second=%d", first.CreatedAt, second.CreatedAt)
	}
	if second.LimitUpload != 200 || !second.Immutable {
		t.Fatalf("update did not persist new fields: %+v", second)
	}

	got, ok, err := r.PrimaryRemoteTarget("vms")
	if err != nil || !ok {
		t.Fatalf("PrimaryRemoteTarget: ok=%v err=%v", ok, err)
	}
	if got.LimitUpload != 200 {
		t.Fatalf("PrimaryRemoteTarget did not read back the update: %+v", got)
	}
}

// TestDeletePrimaryRemoteTarget clears a domain's primary row, as happens when
// its path goes back to local.
func TestDeletePrimaryRemoteTarget(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if _, err := r.UpsertPrimaryRemoteTarget("flash", store.OffsiteTarget{Repo: "s3:flash", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := r.DeletePrimaryRemoteTarget("flash"); err != nil {
		t.Fatalf("DeletePrimaryRemoteTarget: %v", err)
	}
	if _, ok, err := r.PrimaryRemoteTarget("flash"); err != nil || ok {
		t.Fatalf("PrimaryRemoteTarget after delete = ok:%v err:%v, want ok:false err:nil", ok, err)
	}
	if err := r.DeletePrimaryRemoteTarget("flash"); err != nil {
		t.Fatalf("DeletePrimaryRemoteTarget (missing) should be a no-op: %v", err)
	}
}
