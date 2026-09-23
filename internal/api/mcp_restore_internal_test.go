package api

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// newStoreWithMCPKeys opens a store holding two active keys and returns the
// database handle too, so a test can break the store by closing it.
func newStoreWithMCPKeys(t *testing.T) (*store.Repo, *sql.DB) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)
	for _, k := range []struct{ id, label string }{
		{"0b7e0b7e0b7e0b7e0b7e0b7e0b7e0b7e", "Laptop"},
		{"77aa77aa77aa77aa77aa77aa77aa77aa", "Desktop"},
	} {
		if _, err := st.CreateMCPKey(k.id, k.label, "digest-"+k.id, "x9Qa", "check-"+k.id, true, 1789600000); err != nil {
			t.Fatalf("seed key %s: %v", k.label, err)
		}
	}
	return st, db
}

func TestConfigRestoreRevokesMCPKeys(t *testing.T) {
	st, db := newStoreWithMCPKeys(t)
	now := time.Unix(1789600000, 0)

	RevokeMCPKeysAfterConfigRestore(st, false, now)
	active, err := st.ActiveMCPKeys()
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 2 {
		t.Fatalf("a boot without a restore must leave %d keys alone, %d left", 2, len(active))
	}

	out := captureLog(t, func() { RevokeMCPKeysAfterConfigRestore(st, true, now) })
	active, err = st.ActiveMCPKeys()
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("%d keys still active after a restored configuration", len(active))
	}
	all, err := st.ListMCPKeys()
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range all {
		if k.RevokedReason != "config-restore" {
			t.Fatalf("key %s revoked with reason %q", k.Label, k.RevokedReason)
		}
	}
	if !strings.Contains(out, "revoked 2 MCP key") {
		t.Fatalf("the revoke is not in the log: %q", out)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	out = captureLog(t, func() { RevokeMCPKeysAfterConfigRestore(st, true, now) })
	if !strings.Contains(out, "could not revoke MCP keys") {
		t.Fatalf("a store error must be logged, got %q", out)
	}
}
