package store_test

import (
	"maps"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestASuccessfulBackupTakesItsEntryOffTheDeclinedList(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	plex, err := r.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}
	// A container that shares its name with the flash target id.
	if _, err := r.UpsertTarget(store.Target{ContainerName: "flash"}); err != nil {
		t.Fatal(err)
	}
	vm, err := r.UpsertVMTarget(store.VMTarget{Name: "Windows11", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	docs, err := r.CreateFileSet(store.FileSet{Name: "Documents", Path: "documents"})
	if err != nil {
		t.Fatal(err)
	}
	tank, err := r.CreateZFSDataset(store.ZFSDataset{Dataset: "tank/data"})
	if err != nil {
		t.Fatal(err)
	}

	entries := map[string]store.BackupEntry{
		plex.ID:              {Kind: "container", Key: "plex"},
		vm.ID:                {Kind: "vm", Key: "Windows11"},
		docs.ID:              {Kind: "files", Key: docs.ID},
		tank.ID:              {Kind: "zfs", Key: tank.ID},
		store.FlashTargetID:  {Kind: "flash", Key: "flash"},
		store.ConfigTargetID: {Kind: "config", Key: "config"},
	}
	bystanders := []store.BackupEntry{{Kind: "container", Key: "flash"}, {Kind: "container", Key: "nobody-added-me"}}
	declineAll := func() {
		t.Helper()
		for _, e := range entries {
			if err := r.DeclineBackup(e); err != nil {
				t.Fatal(err)
			}
		}
		for _, e := range bystanders {
			if err := r.DeclineBackup(e); err != nil {
				t.Fatal(err)
			}
		}
	}
	finish := func(target, kind, status string) {
		t.Helper()
		id, err := r.StartRun(target, kind)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.FinishRun(id, status, "", 0, ""); err != nil {
			t.Fatal(err)
		}
	}
	declined := func() map[store.BackupEntry]bool {
		t.Helper()
		marks, err := r.DeclinedBackups()
		if err != nil {
			t.Fatal(err)
		}
		return marks
	}

	declineAll()
	all := declined()
	if len(all) != len(entries)+len(bystanders) {
		t.Fatalf("declined %d entries, the list has %v", len(entries)+len(bystanders), all)
	}

	for target := range entries {
		finish(target, "backup", "failed")
		finish(target, "restore", "success")
		finish(target, "dbdump", "success")
	}
	if got := declined(); !maps.Equal(got, all) {
		t.Fatalf("a failed backup, a restore or a dump took entries off the list: %v", got)
	}

	for target, entry := range entries {
		finish(target, "backup", "success")
		want := maps.Clone(all)
		delete(want, entry)
		if got := declined(); !maps.Equal(got, want) {
			t.Fatalf("a backup of %v leaves %v, want %v", entry, got, want)
		}
		declineAll()
	}

	finish(docs.ID, "import", "success")
	if declined()[entries[docs.ID]] {
		t.Fatal("an import left its folder set on the list")
	}

	if err := r.AcceptBackup(bystanders[1]); err != nil {
		t.Fatal(err)
	}
	if declined()[bystanders[1]] {
		t.Fatal("AcceptBackup left the entry on the list")
	}
}

func TestDeletingAFolderSetOrZFSEntryDropsItsDecline(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	docs, err := r.CreateFileSet(store.FileSet{Name: "Documents", Path: "documents"})
	if err != nil {
		t.Fatal(err)
	}
	tank, err := r.CreateZFSDataset(store.ZFSDataset{Dataset: "tank/data"})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []store.BackupEntry{{Kind: "files", Key: docs.ID}, {Kind: "zfs", Key: tank.ID}} {
		if err := r.DeclineBackup(e); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.DeleteFileSet(docs.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteZFSDataset(tank.ID); err != nil {
		t.Fatal(err)
	}
	if marks, err := r.DeclinedBackups(); err != nil || len(marks) != 0 {
		t.Fatalf("the deleted entries left the marks %v, %v", marks, err)
	}
}

func TestARemovedContainerOrVMTakesItsDeclineWithIt(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "plex"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpsertVMTarget(store.VMTarget{Name: "Windows11", Method: "graceful"}); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []store.BackupEntry{{Kind: "container", Key: "plex"}, {Kind: "vm", Key: "Windows11"}, {Kind: "container", Key: "sonarr"}} {
		if err := r.DeclineBackup(entry); err != nil {
			t.Fatal(err)
		}
	}

	if err := r.DeleteTarget("plex", nil); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteVMTarget("Windows11", nil); err != nil {
		t.Fatal(err)
	}

	marks, err := r.DeclinedBackups()
	if err != nil {
		t.Fatal(err)
	}
	if want := map[store.BackupEntry]bool{{Kind: "container", Key: "sonarr"}: true}; !maps.Equal(marks, want) {
		t.Fatalf("declined = %v, want only the container nobody removed", marks)
	}
}
