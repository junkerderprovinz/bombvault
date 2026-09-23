package store_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func newMCPRepo(t *testing.T) *store.Repo {
	t.Helper()
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return store.New(db)
}

// addMCPKey creates a key whose digest, hint and check are derived from its id,
// so a test can tell one row's secret material from another's.
func addMCPKey(t *testing.T, r *store.Repo, id, label string, canStart bool, now int64) store.MCPKey {
	t.Helper()
	key, err := r.CreateMCPKey(id, label, "digest-"+id, "h"+id, "check-"+id, canStart, now)
	if err != nil {
		t.Fatalf("CreateMCPKey(%s): %v", id, err)
	}
	return key
}

func TestCreateMCPKeyEnforcesLimitAndActiveLabel(t *testing.T) {
	r := newMCPRepo(t)
	for i := 0; i < store.MCPKeyLimit; i++ {
		addMCPKey(t, r, fmt.Sprintf("id%02d", i), fmt.Sprintf("client %d", i), true, 1000+int64(i))
	}
	_, err := r.CreateMCPKey("one-too-many", "eleven", "digest-x", "hx", "check-x", true, 2000)
	if !errors.Is(err, store.ErrMCPKeyLimit) {
		t.Fatalf("the eleventh create gave %v, want ErrMCPKeyLimit", err)
	}
	active, err := r.ActiveMCPKeys()
	if err != nil {
		t.Fatalf("ActiveMCPKeys: %v", err)
	}
	if len(active) != store.MCPKeyLimit {
		t.Fatalf("%d active keys, want %d", len(active), store.MCPKeyLimit)
	}

	r = newMCPRepo(t)
	addMCPKey(t, r, "laptop", "Laptop", true, 1000)
	_, err = r.CreateMCPKey("second", "laptop", "digest-second", "hs", "check-second", true, 1100)
	if !errors.Is(err, store.ErrMCPKeyLabelTaken) {
		t.Fatalf("a label differing only in case gave %v, want ErrMCPKeyLabelTaken", err)
	}
	if err := r.RevokeMCPKey("laptop", "user", 1200); err != nil {
		t.Fatalf("RevokeMCPKey: %v", err)
	}
	if _, err := r.CreateMCPKey("second", "laptop", "digest-second", "hs", "check-second", true, 1300); err != nil {
		t.Fatalf("a revoked key must release its label, got %v", err)
	}
}

func TestRotateMCPKeyKeepsIdentity(t *testing.T) {
	r := newMCPRepo(t)
	before := addMCPKey(t, r, "laptop", "Laptop", false, 1000)

	after, err := r.RotateMCPKey("laptop", "digest-new", "hnew", "check-new", 2000)
	if err != nil {
		t.Fatalf("RotateMCPKey: %v", err)
	}
	if after.ID != before.ID || after.Label != before.Label || after.CanStartBackups != before.CanStartBackups {
		t.Fatalf("rotation changed the identity: %+v, want id, label and permission of %+v", after, before)
	}
	if after.CreatedAt != before.CreatedAt {
		t.Fatalf("CreatedAt = %d, want %d", after.CreatedAt, before.CreatedAt)
	}
	if after.Digest != "digest-new" || after.Hint != "hnew" || after.Check != "check-new" {
		t.Fatalf("secret material = %q/%q/%q, want the new one", after.Digest, after.Hint, after.Check)
	}
	if after.RotatedAt != 2000 {
		t.Fatalf("RotatedAt = %d, want 2000", after.RotatedAt)
	}

	if err := r.RevokeMCPKey("laptop", "user", 3000); err != nil {
		t.Fatalf("RevokeMCPKey: %v", err)
	}
	if _, err := r.RotateMCPKey("laptop", "digest-later", "hl", "check-later", 4000); !errors.Is(err, store.ErrMCPKeyNotFound) {
		t.Fatalf("rotating a revoked key gave %v, want ErrMCPKeyNotFound", err)
	}
}

func TestRevokeMCPKeyKeepsRowAndClearsDigest(t *testing.T) {
	r := newMCPRepo(t)
	addMCPKey(t, r, "laptop", "Laptop", true, 1000)

	if err := r.RevokeMCPKey("laptop", "user", 2000); err != nil {
		t.Fatalf("RevokeMCPKey: %v", err)
	}
	active, err := r.ActiveMCPKeys()
	if err != nil {
		t.Fatalf("ActiveMCPKeys: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("ActiveMCPKeys = %+v, want none", active)
	}
	all, err := r.ListMCPKeys()
	if err != nil {
		t.Fatalf("ListMCPKeys: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("ListMCPKeys returned %d rows, want the revoked one", len(all))
	}
	got := all[0]
	if got.Digest != "" {
		t.Fatalf("Digest = %q, want it cleared", got.Digest)
	}
	if got.Hint != "hlaptop" || got.Label != "Laptop" {
		t.Fatalf("hint %q and label %q, want them kept so the log can name the key", got.Hint, got.Label)
	}
	if got.RevokedAt != 2000 || got.RevokedReason != "user" {
		t.Fatalf("revoked %d/%q, want 2000/user", got.RevokedAt, got.RevokedReason)
	}

	if err := r.RevokeMCPKey("laptop", "user", 3000); !errors.Is(err, store.ErrMCPKeyNotFound) {
		t.Fatalf("a second revoke gave %v, want ErrMCPKeyNotFound", err)
	}
}

func TestRevokeAllMCPKeysCountsOnlyActive(t *testing.T) {
	r := newMCPRepo(t)
	addMCPKey(t, r, "a", "A", true, 1000)
	addMCPKey(t, r, "b", "B", true, 1100)
	addMCPKey(t, r, "c", "C", true, 1200)
	if err := r.RevokeMCPKey("c", "user", 1300); err != nil {
		t.Fatalf("RevokeMCPKey: %v", err)
	}

	n, err := r.RevokeAllMCPKeys("config-restore", 2000)
	if err != nil {
		t.Fatalf("RevokeAllMCPKeys: %v", err)
	}
	if n != 2 {
		t.Fatalf("RevokeAllMCPKeys = %d, want the 2 keys that were still active", n)
	}
	for _, id := range []string{"a", "b"} {
		key, err := r.GetMCPKey(id)
		if err != nil {
			t.Fatalf("GetMCPKey(%s): %v", id, err)
		}
		if key.RevokedAt != 2000 || key.RevokedReason != "config-restore" {
			t.Fatalf("key %s revoked %d/%q, want 2000/config-restore", id, key.RevokedAt, key.RevokedReason)
		}
	}
	kept, err := r.GetMCPKey("c")
	if err != nil {
		t.Fatalf("GetMCPKey(c): %v", err)
	}
	if kept.RevokedReason != "user" {
		t.Fatalf("the already revoked key now reads %q, want its own reason", kept.RevokedReason)
	}
}

func TestPurgeMCPKeyRefusesActiveAndReferenced(t *testing.T) {
	r := newMCPRepo(t)
	addMCPKey(t, r, "active", "Active", true, 1000)
	addMCPKey(t, r, "used", "Used", true, 1100)
	addMCPKey(t, r, "spent", "Spent", true, 1200)

	if err := r.PurgeMCPKey("active"); !errors.Is(err, store.ErrMCPKeyActive) {
		t.Fatalf("purging an active key gave %v, want ErrMCPKeyActive", err)
	}
	for _, id := range []string{"used", "spent"} {
		if err := r.RevokeMCPKey(id, "user", 1300); err != nil {
			t.Fatalf("RevokeMCPKey(%s): %v", id, err)
		}
	}
	if _, err := r.StartRunWith("t1", "backup", store.RunMeta{StartedVia: "mcp", StartedViaKey: "used"}); err != nil {
		t.Fatalf("StartRunWith: %v", err)
	}
	if err := r.PurgeMCPKey("used"); !errors.Is(err, store.ErrMCPKeyInUse) {
		t.Fatalf("purging a key a run names gave %v, want ErrMCPKeyInUse", err)
	}

	if err := r.PurgeMCPKey("spent"); err != nil {
		t.Fatalf("PurgeMCPKey: %v", err)
	}
	if _, err := r.GetMCPKey("spent"); !errors.Is(err, store.ErrMCPKeyNotFound) {
		t.Fatalf("GetMCPKey after the purge gave %v, want ErrMCPKeyNotFound", err)
	}
}

func TestUpdateMCPKeyLabelAndPermission(t *testing.T) {
	r := newMCPRepo(t)
	addMCPKey(t, r, "laptop", "Laptop", true, 1000)
	addMCPKey(t, r, "desktop", "Desktop", true, 1100)

	label := "Work laptop"
	got, err := r.UpdateMCPKey("laptop", &label, nil)
	if err != nil {
		t.Fatalf("UpdateMCPKey: %v", err)
	}
	if got.Label != label || !got.CanStartBackups {
		t.Fatalf("key = %+v, want the new label and the permission untouched", got)
	}

	readOnly := false
	got, err = r.UpdateMCPKey("laptop", nil, &readOnly)
	if err != nil {
		t.Fatalf("UpdateMCPKey: %v", err)
	}
	if got.Label != label || got.CanStartBackups {
		t.Fatalf("key = %+v, want the label untouched and the permission off", got)
	}

	taken := "desktop"
	if _, err := r.UpdateMCPKey("laptop", &taken, nil); !errors.Is(err, store.ErrMCPKeyLabelTaken) {
		t.Fatalf("taking another active key's label gave %v, want ErrMCPKeyLabelTaken", err)
	}

	if err := r.RevokeMCPKey("laptop", "user", 2000); err != nil {
		t.Fatalf("RevokeMCPKey: %v", err)
	}
	if _, err := r.UpdateMCPKey("laptop", &label, nil); !errors.Is(err, store.ErrMCPKeyNotFound) {
		t.Fatalf("updating a revoked key gave %v, want ErrMCPKeyNotFound", err)
	}
}

func TestTouchMCPKey(t *testing.T) {
	r := newMCPRepo(t)
	addMCPKey(t, r, "laptop", "Laptop", true, 1000)
	addMCPKey(t, r, "gone", "Gone", true, 1100)
	if err := r.RevokeMCPKey("gone", "user", 1200); err != nil {
		t.Fatalf("RevokeMCPKey: %v", err)
	}

	if err := r.TouchMCPKey("laptop", 2000, "10.0.0.5"); err != nil {
		t.Fatalf("TouchMCPKey: %v", err)
	}
	key, err := r.GetMCPKey("laptop")
	if err != nil {
		t.Fatalf("GetMCPKey: %v", err)
	}
	if key.LastUsedAt != 2000 || key.LastUsedFrom != "10.0.0.5" {
		t.Fatalf("last use = %d/%q, want 2000/10.0.0.5", key.LastUsedAt, key.LastUsedFrom)
	}

	for _, id := range []string{"gone", "never-existed"} {
		if err := r.TouchMCPKey(id, 3000, "10.0.0.6"); err != nil {
			t.Fatalf("TouchMCPKey(%s) = %v, want no error", id, err)
		}
	}
	revoked, err := r.GetMCPKey("gone")
	if err != nil {
		t.Fatalf("GetMCPKey: %v", err)
	}
	if revoked.LastUsedAt != 0 || revoked.LastUsedFrom != "" {
		t.Fatalf("a revoked key recorded a use: %d/%q", revoked.LastUsedAt, revoked.LastUsedFrom)
	}
}
