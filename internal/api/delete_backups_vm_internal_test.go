package api

import (
	"context"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestDeleteBackupsVMUnknownEstablishmentKeepsEntry: without a local repository
// DeleteBackupsVM removes only the entry, and only when it knows the repository
// was never created. If the store cannot say, the repository may sit on an
// unmounted share with every snapshot still in it, so the entry stays. Breaking
// the schema is the only way to make that read fail.
func TestDeleteBackupsVMUnknownEstablishmentKeepsEntry(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.VMsPath = "backups/vms" // never created on disk
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"}); err != nil {
		t.Fatal(err)
	}
	svc := NewService(config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}, st, nil, nil, nil)

	// Every "was this repository established?" read fails from here on.
	if _, err := db.Exec("DROP TABLE established_repos"); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteBackupsVM(context.Background(), "win11", ""); err == nil {
		t.Fatal("DeleteBackupsVM must refuse while it cannot tell whether the repository was ever created")
	}
	if _, err := st.GetVMTargetByName("win11"); err != nil {
		t.Fatalf("the entry must stay: %v", err)
	}
}
