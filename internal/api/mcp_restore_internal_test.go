package api

import (
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/selfrestore"
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

// markRestoreApplied leaves the mark ApplyPending writes when it swaps a
// restored database into place.
func markRestoreApplied(t *testing.T, dataDir string) {
	t.Helper()
	if err := os.WriteFile(selfrestore.AppliedMarkerPath(dataDir), []byte("applied"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestConfigRestoreRevokesMCPKeys(t *testing.T) {
	st, _ := newStoreWithMCPKeys(t)
	dataDir := t.TempDir()
	now := time.Unix(1789600000, 0)

	if err := RevokeMCPKeysAfterConfigRestore(st, dataDir, now); err != nil {
		t.Fatal(err)
	}
	active, err := st.ActiveMCPKeys()
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 2 {
		t.Fatalf("a boot without a restore must leave %d keys alone, %d left", 2, len(active))
	}

	markRestoreApplied(t, dataDir)
	out := captureLog(t, func() {
		if err := RevokeMCPKeysAfterConfigRestore(st, dataDir, now); err != nil {
			t.Fatal(err)
		}
	})
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
	if _, err := os.Stat(selfrestore.AppliedMarkerPath(dataDir)); !os.IsNotExist(err) {
		t.Fatalf("the restore is still marked after the revoke went through: %v", err)
	}
}

// A boot that cannot revoke must not come up with the restored keys active, and
// the next boot has to try again.
func TestConfigRestoreRevokeFailureStopsTheBoot(t *testing.T) {
	st, db := newStoreWithMCPKeys(t)
	dataDir := t.TempDir()
	markRestoreApplied(t, dataDir)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if err := RevokeMCPKeysAfterConfigRestore(st, dataDir, time.Unix(1789600000, 0)); err == nil {
		t.Fatal("a failed revoke was not reported")
	}
	if _, err := os.Stat(selfrestore.AppliedMarkerPath(dataDir)); err != nil {
		t.Fatalf("the mark is gone although the revoke failed, so no later boot revokes: %v", err)
	}
}
