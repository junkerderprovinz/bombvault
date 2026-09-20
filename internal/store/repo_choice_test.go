package store_test

import (
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func repoChoiceStore(t *testing.T) *store.Repo {
	t.Helper()
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return store.New(db)
}

func TestNewItemRowsStartOpen(t *testing.T) {
	r := repoChoiceStore(t)
	tg, err := r.UpsertTarget(store.Target{ContainerName: "nginx"})
	if err != nil {
		t.Fatal(err)
	}
	vm, err := r.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	created, err := r.CreateFileSet(store.FileSet{Name: "docs", Path: "user/docs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	set, err := r.GetFileSet(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	for what, got := range map[string]store.RepoChoice{"container": tg.RepoChosen, "vm": vm.RepoChosen, "file set": set.RepoChosen} {
		if got != store.RepoOpen {
			t.Errorf("a new %s row reads %d, want open", what, got)
		}
	}
}

func TestUpsertWritesTheLocationOnlyWhenItCreatesTheRow(t *testing.T) {
	r := repoChoiceStore(t)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx", Repo: "repo-nas", RepoChosen: store.RepoChosen}); err != nil {
		t.Fatal(err)
	}
	tg, err := r.UpsertTarget(store.Target{ContainerName: "nginx", Definition: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	if tg.Repo != "repo-nas" || tg.RepoChosen != store.RepoChosen {
		t.Fatalf("a later upsert moved the location: repo %q, choice %d", tg.Repo, tg.RepoChosen)
	}
}

func TestARepositoryWithoutAChoiceIsRefused(t *testing.T) {
	r := repoChoiceStore(t)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx", Repo: "repo-nas"}); !errors.Is(err, store.ErrRepoChoice) {
		t.Errorf("UpsertTarget with a repository and no choice: %v, want ErrRepoChoice", err)
	}
	if _, err := r.UpsertVMTarget(store.VMTarget{Name: "win11", Repo: "repo-nas", RepoChosen: store.RepoChosenUnread}); !errors.Is(err, store.ErrRepoChoice) {
		t.Errorf("UpsertVMTarget with a repository marked unread: %v, want ErrRepoChoice", err)
	}
	if _, err := r.CreateFileSet(store.FileSet{Name: "docs", Path: "user/docs", Repo: "repo-nas"}); !errors.Is(err, store.ErrRepoChoice) {
		t.Errorf("CreateFileSet with a repository and no choice: %v, want ErrRepoChoice", err)
	}
}
