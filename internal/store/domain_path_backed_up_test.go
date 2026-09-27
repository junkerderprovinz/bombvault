package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestADomainPathCountsOnlyBackupsWrittenToIt(t *testing.T) {
	r := newRepo(t)
	backup := func(targetID, status string) {
		t.Helper()
		id, err := r.StartRun(targetID, "backup")
		if err != nil {
			t.Fatal(err)
		}
		if err := r.FinishRun(id, status, "", 0, ""); err != nil {
			t.Fatal(err)
		}
	}
	nas := store.ItemRef{Domain: "containers", Key: "nextcloud"}
	if _, err := r.WritePlacement(nas, &store.HomeWrite{Repo: "nas-repo", Choice: store.RepoChosen}, nil, nil); err != nil {
		t.Fatal(err)
	}
	elsewhere, err := r.GetTargetByContainer("nextcloud")
	if err != nil {
		t.Fatal(err)
	}
	backup(elsewhere.ID, "success")
	if backed, err := r.DomainPathBackedUp("containers"); err != nil || backed {
		t.Fatalf("after a backup into a named repository = %v, %v, want false", backed, err)
	}
	plain, err := r.UpsertTarget(store.Target{ContainerName: "nginx"})
	if err != nil {
		t.Fatal(err)
	}
	backup(plain.ID, "failed")
	if backed, err := r.DomainPathBackedUp("containers"); err != nil || backed {
		t.Fatalf("after a failed backup into the domain path = %v, %v, want false", backed, err)
	}
	backup(plain.ID, "success")
	if backed, err := r.DomainPathBackedUp("containers"); err != nil || !backed {
		t.Fatalf("after a backup into the domain path = %v, %v, want true", backed, err)
	}
	backup(store.FlashTargetID, "success")
	if backed, err := r.DomainPathBackedUp("flash"); err != nil || !backed {
		t.Fatalf("flash = %v, %v, want true", backed, err)
	}
	if backed, err := r.DomainPathBackedUp("config"); err != nil || backed {
		t.Fatalf("config = %v, %v, want false", backed, err)
	}
}

func TestAVMOrFolderSetWithARepositoryOfItsOwnLeavesTheDomainPathUnused(t *testing.T) {
	r := newRepo(t)
	vm, err := r.UpsertVMTarget(store.VMTarget{Name: "Windows 11", Repo: "nas-repo", RepoChosen: store.RepoChosen})
	if err != nil {
		t.Fatal(err)
	}
	set, err := r.CreateFileSet(store.FileSet{Name: "Docs", Path: "docs", Enabled: true, Repo: "nas-repo", RepoChosen: store.RepoChosen})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{vm.ID, set.ID} {
		run, err := r.StartRun(id, "backup")
		if err != nil {
			t.Fatal(err)
		}
		if err := r.FinishRun(run, "success", "", 0, ""); err != nil {
			t.Fatal(err)
		}
	}
	for _, domain := range []string{"vms", "files"} {
		if backed, err := r.DomainPathBackedUp(domain); err != nil || backed {
			t.Errorf("%s = %v, %v, want false", domain, backed, err)
		}
	}
}

func TestADomainPathOfVMsAndFolderSetsCountsTheirOwnRuns(t *testing.T) {
	r := newRepo(t)
	vm, err := r.UpsertVMTarget(store.VMTarget{Name: "Windows 11"})
	if err != nil {
		t.Fatal(err)
	}
	set, err := r.CreateFileSet(store.FileSet{Name: "Docs", Path: "docs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{vm.ID, set.ID} {
		run, err := r.StartRun(id, "backup")
		if err != nil {
			t.Fatal(err)
		}
		if err := r.FinishRun(run, "success", "", 0, ""); err != nil {
			t.Fatal(err)
		}
	}
	for _, domain := range []string{"vms", "files"} {
		if backed, err := r.DomainPathBackedUp(domain); err != nil || !backed {
			t.Errorf("%s = %v, %v, want true", domain, backed, err)
		}
	}
	if _, err := r.DomainPathBackedUp("snapshots"); err == nil {
		t.Error("an unknown domain gave no error")
	}
}
