package store_test

import (
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestWritePlacementCreatesTheRowOfAContainerWithItsHome(t *testing.T) {
	r := repoChoiceStore(t)
	item := store.ItemRef{Domain: "containers", Key: "nginx"}
	ok, err := r.WritePlacement(item, &store.HomeWrite{Repo: "repo-nas", Choice: store.RepoChosen}, nil, &store.HomeState{})
	if err != nil || !ok {
		t.Fatalf("WritePlacement = %v, %v", ok, err)
	}
	if got := store.HomeOf(t, r, item); got != (store.HomeState{Exists: true, Repo: "repo-nas", Choice: store.RepoChosen}) {
		t.Fatalf("home = %+v", got)
	}
	if _, err := r.GetTargetByContainer("nginx"); err != nil {
		t.Fatalf("the new row must read back as a target: %v", err)
	}
}

func TestWritePlacementNeedsTheFileSetRow(t *testing.T) {
	r := repoChoiceStore(t)
	home := &store.HomeWrite{Choice: store.RepoChosen}
	if _, err := r.WritePlacement(store.ItemRef{Domain: "files", Key: "no-such-set"}, home, nil, nil); err == nil {
		t.Fatal("a location for a file set that does not exist was accepted")
	}
}

func TestWritePlacementLeavesARowAloneThatChangedSinceItWasRead(t *testing.T) {
	r := repoChoiceStore(t)
	item := store.ItemRef{Domain: "vms", Key: "win11"}
	if _, err := r.UpsertVMTarget(store.VMTarget{Name: "win11"}); err != nil {
		t.Fatal(err)
	}
	stale := store.HomeOf(t, r, item)
	if _, err := r.WritePlacement(item, &store.HomeWrite{Repo: "repo-a", Choice: store.RepoChosen}, nil, nil); err != nil {
		t.Fatal(err)
	}
	ok, err := r.WritePlacement(item, &store.HomeWrite{Repo: "repo-b", Choice: store.RepoChosen}, &store.CopiesWrite{Skip: []string{store.SkipAll}}, &stale)
	if err != nil || ok {
		t.Fatalf("WritePlacement over a changed row = %v, %v, want false", ok, err)
	}
	if got := store.HomeOf(t, r, item); got.Repo != "repo-a" {
		t.Fatalf("repo = %q, want the value written in between", got.Repo)
	}
	if _, found := store.RuleSkip(t, r, "vms", "vm:win11"); found {
		t.Fatal("the rule was written although the row had changed")
	}
}

func TestWritePlacementResetsLocationAndRuleTogether(t *testing.T) {
	r := repoChoiceStore(t)
	item := store.ItemRef{Domain: "containers", Key: "nginx"}
	if _, err := r.WritePlacement(item, &store.HomeWrite{Repo: "repo-nas", Choice: store.RepoChosen}, &store.CopiesWrite{Skip: []string{store.SkipAll}}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := r.WritePlacement(item, &store.HomeWrite{Choice: store.RepoOpen}, &store.CopiesWrite{Follow: true}, nil); err != nil {
		t.Fatal(err)
	}
	if got := store.HomeOf(t, r, item); got.Repo != "" || got.Choice != store.RepoOpen {
		t.Fatalf("home = %+v, want open", got)
	}
	if _, found := store.RuleSkip(t, r, "containers", "container:nginx"); found {
		t.Fatal("the rule survived the reset")
	}
}

func TestWritePlacementKeysAFileSetRuleByTheSetName(t *testing.T) {
	r := repoChoiceStore(t)
	set, err := r.CreateFileSet(store.FileSet{Name: "Photos", Path: "user/photos", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.WritePlacement(store.ItemRef{Domain: "files", Key: set.ID}, nil, &store.CopiesWrite{Skip: []string{store.SkipAll}}, nil); err != nil {
		t.Fatal(err)
	}
	skip, found := store.RuleSkip(t, r, "files", "fileset:Photos")
	if !found || len(skip) != 1 || skip[0] != store.SkipAll {
		t.Fatalf("rule = %v, %v, want [*] under the set's name", skip, found)
	}
}

func TestWritePlacementRefusesAnOpenLocationWithARepository(t *testing.T) {
	r := repoChoiceStore(t)
	_, err := r.WritePlacement(store.ItemRef{Domain: "containers", Key: "nginx"}, &store.HomeWrite{Repo: "repo-nas", Choice: store.RepoOpen}, nil, nil)
	if !errors.Is(err, store.ErrRepoChoice) {
		t.Fatalf("err = %v, want ErrRepoChoice", err)
	}
}

func TestWritePlacementNeedsSomethingToWrite(t *testing.T) {
	r := repoChoiceStore(t)
	if _, err := r.WritePlacement(store.ItemRef{Domain: "containers", Key: "nginx"}, nil, nil, nil); err == nil {
		t.Fatal("an empty write was accepted")
	}
}

func TestTheHomeOfAMissingRowIsOpen(t *testing.T) {
	r := repoChoiceStore(t)
	if got := store.HomeOf(t, r, store.ItemRef{Domain: "containers", Key: "nobody"}); got != (store.HomeState{}) {
		t.Fatalf("home = %+v, want the zero state", got)
	}
}
