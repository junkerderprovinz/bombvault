package store_test

import (
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func movePath(t *testing.T, r *store.Repo, path string) {
	t.Helper()
	if _, err := r.MutateSettings(func(s *store.Settings) error {
		s.ContainersPath = path
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func formerRepos(t *testing.T, r *store.Repo, domain string) []string {
	t.Helper()
	former, err := r.FormerOwnFolders(domain)
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, f := range former {
		out = append(out, f.Repo)
	}
	return out
}

func TestASectionRemembersTheOwnFoldersItMovedAwayFrom(t *testing.T) {
	r := newRepo(t)
	movePath(t, r, "user/first")
	movePath(t, r, "user/second")

	if got := formerRepos(t, r, "containers"); len(got) != 2 || !slices.Contains(got, "user/first") || !slices.Contains(got, "user/bombvault/container") {
		t.Fatalf("former own folders = %q, want the default and user/first", got)
	}
	if got := formerRepos(t, r, "vms"); len(got) != 0 {
		t.Fatalf("vms never moved, former own folders = %q", got)
	}

	movePath(t, r, "user/first/")
	if got := formerRepos(t, r, "containers"); slices.Contains(got, "user/first") || !slices.Contains(got, "user/second") {
		t.Fatalf("after moving back = %q, want user/first current again and user/second left", got)
	}
}

func TestAnOwnFolderMovedOutOfAStorageLocationKeepsItsSignIn(t *testing.T) {
	r := newRepo(t)
	d, err := r.SaveDestination(store.OffsiteTarget{Name: "Box", Repo: "s3:https://box/bv", CredsRef: "box-keys"})
	if err != nil {
		t.Fatal(err)
	}
	inBox := &store.OffsiteTarget{Repo: "s3:https://box/bv/containers", DestinationID: d.ID, CredsRef: "box-keys"}
	if err := r.MoveOwnFolder(store.OwnFolder{Domain: "containers", Row: inBox}, func(s *store.Settings) error {
		s.ContainersPath = inBox.Repo
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.MoveOwnFolder(store.OwnFolder{Domain: "containers"}, func(s *store.Settings) error {
		s.ContainersPath = "user/home"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	former, err := r.FormerOwnFolders("containers")
	if err != nil || len(former) == 0 {
		t.Fatalf("former own folders = %+v, %v", former, err)
	}
	if got := former[0]; got.Repo != inBox.Repo || got.DestinationID != d.ID || got.CredsRef != "box-keys" {
		t.Fatalf("last left = %+v, want the folder in Box with its sign-in", got)
	}

	if err := r.RemoveDestination(d.ID, func(store.OffsiteTarget, store.Settings) bool { return false }, nil); err != nil {
		t.Fatal(err)
	}
	if got := formerRepos(t, r, "containers"); slices.Contains(got, inBox.Repo) {
		t.Fatalf("former own folders = %q, the one in the removed Box cannot be read any more", got)
	}
}

func TestAPlaceChangesAfterABackupWhenItsOwnFolderMovesLater(t *testing.T) {
	r := newRepo(t)
	movePath(t, r, "user/moved")
	former, err := r.FormerOwnFolders("containers")
	if err != nil || len(former) == 0 {
		t.Fatalf("former own folders = %+v, %v", former, err)
	}
	left := former[0].LeftAt

	if changed, err := r.PlaceChangedSince("containers", "any", left-60); err != nil || !changed {
		t.Fatalf("a backup before the move = %v, %v, want its place changed", changed, err)
	}
	if changed, err := r.PlaceChangedSince("containers", "any", left); err != nil || !changed {
		t.Fatalf("a backup in the second of the move = %v, %v, want its place changed", changed, err)
	}
	if changed, err := r.PlaceChangedSince("containers", "any", left+1); err != nil || changed {
		t.Fatalf("a backup after the move = %v, %v, want its place unchanged", changed, err)
	}
	if changed, err := r.PlaceChangedSince("vms", "any", left-60); err != nil || changed {
		t.Fatalf("another section = %v, %v, want its place unchanged", changed, err)
	}
}
