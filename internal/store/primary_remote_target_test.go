package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestOffsiteTargetRoleDefaultsToOffsite pins the normalization guard: a row
// upserted without a Role (every caller/row that predates issue #152) is
// stored as RoleOffsite, so it keeps showing up in the off-site queries
// exactly as before this field existed.
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

// TestPrimaryRemoteTargetIsolatedFromOffsiteQueries is the core invariant of
// the #152 schema reuse: a domain's "primary" row (remote-primary safety
// settings) must never be returned by ListOffsiteTargets, OffsiteTargetsForDomain,
// GetOffsiteTarget or DeleteOffsiteTarget — those are the off-site REPLICATION
// destination surface, and a primary row is not a replication destination. It
// IS reachable via PrimaryRemoteTarget, keyed by domain.
func TestPrimaryRemoteTargetIsolatedFromOffsiteQueries(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// One real off-site destination for "containers" ...
	offsiteTgt, err := r.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Backblaze", Repo: "b2:bucket:path", Enabled: true})
	if err != nil {
		t.Fatalf("UpsertOffsiteTarget: %v", err)
	}
	// ... and a primary-remote safety config for the SAME domain.
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

	// ListOffsiteTargets / OffsiteTargetsForDomain see ONLY the off-site row.
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

	// GetOffsiteTarget must not resolve the primary row's id (the off-site
	// CRUD/test/delete handlers must never reach it via that surface).
	if _, ok, _ := r.GetOffsiteTarget(primaryTgt.ID); ok {
		t.Fatal("GetOffsiteTarget must not resolve a \"primary\"-role id")
	}
	// DeleteOffsiteTarget on the primary row's id must be a no-op — it must
	// survive an off-site-target delete call.
	if err := r.DeleteOffsiteTarget(primaryTgt.ID); err != nil {
		t.Fatalf("DeleteOffsiteTarget(primary id) should be a no-op, got err: %v", err)
	}
	if _, ok, err := r.PrimaryRemoteTarget("containers"); err != nil || !ok {
		t.Fatalf("primary row was deleted via DeleteOffsiteTarget — isolation broken (ok=%v err=%v)", ok, err)
	}

	// PrimaryRemoteTarget round-trips the safety fields.
	back, ok, err := r.PrimaryRemoteTarget("containers")
	if err != nil || !ok {
		t.Fatalf("PrimaryRemoteTarget: ok=%v err=%v", ok, err)
	}
	if !back.Immutable || back.LimitUpload != 500 || back.LimitDownload != 250 || back.GrowthBudgetGB != 100 {
		t.Fatalf("PrimaryRemoteTarget round-trip mismatch: %+v", back)
	}

	// A domain with no primary row configured reports ok=false, not an error.
	if _, ok, err := r.PrimaryRemoteTarget("vms"); err != nil || ok {
		t.Fatalf("PrimaryRemoteTarget(unconfigured domain) = ok:%v err:%v, want ok:false err:nil", ok, err)
	}
}

// TestUpsertPrimaryRemoteTargetUpdatesInPlace pins the id-preserving update
// path (mirrors syncPrimaryOffsiteTarget's off-site counterpart): a second
// UpsertPrimaryRemoteTarget for the same domain updates the SAME row rather
// than creating a second one, and its id/created_at survive the update.
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

	// Still exactly one primary row for the domain.
	got, ok, err := r.PrimaryRemoteTarget("vms")
	if err != nil || !ok {
		t.Fatalf("PrimaryRemoteTarget: ok=%v err=%v", ok, err)
	}
	if got.LimitUpload != 200 {
		t.Fatalf("PrimaryRemoteTarget did not read back the update: %+v", got)
	}
}

// TestDeletePrimaryRemoteTarget pins the clear-safety-settings path (used when
// an operator switches a domain's path back to local).
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
	// A second delete (nothing left to remove) is a no-op.
	if err := r.DeletePrimaryRemoteTarget("flash"); err != nil {
		t.Fatalf("DeletePrimaryRemoteTarget (missing) should be a no-op: %v", err)
	}
}
