package store_test

import (
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAnItemThatFollowsItsSectionRemembersWhereItWas(t *testing.T) {
	r := newRepo(t)
	item := store.ItemRef{Domain: "containers", Key: "nginx"}
	if _, err := r.WritePlacement(item, &store.HomeWrite{Choice: store.RepoChosen}, nil, nil); err != nil {
		t.Fatal(err)
	}
	tg, err := r.GetTargetByContainer("nginx")
	if err != nil {
		t.Fatal(err)
	}

	stale := store.HomeState{Exists: true, Repo: "nas", Choice: store.RepoChosen}
	if moved, err := r.FollowSection(item, stale); err != nil || moved {
		t.Fatalf("FollowSection on a row that reads differently = %v, %v, want nothing written", moved, err)
	}
	here := store.HomeState{Exists: true, Choice: store.RepoChosen}
	if moved, err := r.FollowSection(item, here); err != nil || !moved {
		t.Fatalf("FollowSection = %v, %v", moved, err)
	}

	if h, err := r.ItemHome(item); err != nil || h.Choice != store.RepoOpen {
		t.Fatalf("home = %+v, %v, want the item open", h, err)
	}
	if former, err := r.FormerHomes("containers", tg.ID); err != nil || !slices.Equal(former, []string{""}) {
		t.Fatalf("FormerHomes = %q, %v, want the section's own folder", former, err)
	}
}
