package store_test

import (
	"database/sql"
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAPlaceRepositoryCarriesThePlacesSettings(t *testing.T) {
	r := newRepo(t)
	p, err := r.WritePlace(store.PlaceWrite{Place: store.Place{Name: "NAS", Provider: "synology", Kind: "local", Base: "remotes/nas",
		Folders: map[string]string{"containers": "container"}, RetentionKeepDaily: 7, LimitUpload: 500, OffPremises: true, Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	row, err := r.CreatePlaceRepo(p.ID, "containers", "")
	if err != nil || row.Role != store.RoleRepo || row.Domain != "" || row.Repo != "remotes/nas/container" ||
		row.PlaceID != p.ID || row.PlaceDomain != "containers" || row.PlaceSuffix != "" ||
		row.RetentionKeepDaily != 7 || row.LimitUpload != 500 || !row.OffPremises || !row.Enabled {
		t.Fatalf("CreatePlaceRepo = %+v, %v", row, err)
	}
	if _, err := r.CreatePlaceRepo(p.ID, "vms", ""); !errors.Is(err, store.ErrPlaceDomainUnavailable) {
		t.Fatalf("a repository of a domain the place does not offer: %v", err)
	}
	if _, err := r.CreatePlaceRepo("nosuchplace", "containers", ""); !errors.Is(err, store.ErrPlaceNotFound) {
		t.Fatalf("a repository at an unknown place: %v", err)
	}
}

func TestAdoptingARowMovesItOntoThePlaceAndAttachesIt(t *testing.T) {
	r := newRepo(t)
	p, err := r.WritePlace(store.PlaceWrite{Place: store.Place{Name: "B2", Provider: "b2", Kind: "s3", Base: "s3:https://s3.example.com/bucket",
		Folders: map[string]string{"vms": "vms"}, Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	loose, err := r.CreateOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "B2", Repo: "s3:https://s3.example.com/old/vms", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	row, err := r.AdoptRow(loose.ID, p.ID, "vms", "")
	if err != nil || row.Repo != "s3:https://s3.example.com/bucket/vms" || row.PlaceID != p.ID || row.PlaceDomain != "vms" {
		t.Fatalf("AdoptRow = %+v, %v", row, err)
	}
	if _, err := r.AdoptRow("nosuchrow", p.ID, "vms", ""); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("AdoptRow of an unknown row = %v, want sql.ErrNoRows", err)
	}
}

func b2PlaceIn(t *testing.T, r *store.Repo) store.Place {
	t.Helper()
	p, err := r.WritePlace(store.PlaceWrite{Place: store.Place{Name: "B2", Provider: "b2", Kind: "s3", Base: "s3:https://s3.example.com/bucket",
		Folders: map[string]string{"containers": "container", "flash": "flash"}, CredsRef: "b2-set", RetentionKeepLast: 9, Immutable: true, Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAChipWritesItsNewTargetAndTheSkipTogether(t *testing.T) {
	r := newRepo(t)
	p := b2PlaceIn(t, r)
	skip := []string{"t-old"}

	row, err := r.WriteDomainCopies(store.DomainCopiesWrite{Domain: "containers", PlaceID: p.ID, Skip: &skip})

	if err != nil || row.Repo != "s3:https://s3.example.com/bucket/container" || row.PlaceID != p.ID || row.PlaceDomain != "containers" ||
		row.CredsRef != "b2-set" || row.RetentionKeepLast != 9 || !row.Immutable || !row.Enabled || row.SortOrder != 0 {
		t.Fatalf("WriteDomainCopies = %+v, %v", row, err)
	}
	d, found, err := r.PlacementDefaultFor("containers")
	if err != nil || !found || !slices.Equal(d.Skip, skip) || d.Home != "" || d.Paused() {
		t.Fatalf("default = %+v, %v, %v", d, found, err)
	}
	if s, err := r.GetSettings(); err != nil || s.ContainersOffsite != row.Repo || !s.ContainersOffsiteImmutable {
		t.Fatalf("the field = %q %v, %v, want the new target", s.ContainersOffsite, s.ContainersOffsiteImmutable, err)
	}
}

func TestAChipKeepsTheDefaultsHomeAndPause(t *testing.T) {
	r := newRepo(t)
	p := b2PlaceIn(t, r)
	if _, err := r.PutPlacementDefault("containers", "repo-1", []string{store.SkipAll}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.PausePlacement("containers"); err != nil {
		t.Fatal(err)
	}
	skip := []string{}

	if _, err := r.WriteDomainCopies(store.DomainCopiesWrite{Domain: "containers", PlaceID: p.ID, Skip: &skip}); err != nil {
		t.Fatal(err)
	}

	d, _, err := r.PlacementDefaultFor("containers")
	if err != nil || d.Home != "repo-1" || len(d.Skip) != 0 || !d.Paused() {
		t.Fatalf("default = %+v, %v, want its home and pause kept", d, err)
	}
}

func TestASecondTargetOfADomainQueuesBehindTheFirst(t *testing.T) {
	r := newRepo(t)
	p := b2PlaceIn(t, r)
	first, err := r.WriteDomainCopies(store.DomainCopiesWrite{Domain: "containers", PlaceID: p.ID})
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.WriteDomainCopies(store.DomainCopiesWrite{Domain: "containers", PlaceID: p.ID, Suffix: "-copies"})
	if err != nil || second.SortOrder != 1 || second.Repo != "s3:https://s3.example.com/bucket/container-copies" {
		t.Fatalf("second = %+v, %v", second, err)
	}
	if s, err := r.GetSettings(); err != nil || s.ContainersOffsite != first.Repo {
		t.Fatalf("the field = %q, %v, want the first target", s.ContainersOffsite, err)
	}
}

func TestSwitchingAFlashTargetOffClearsItsField(t *testing.T) {
	r := newRepo(t)
	p := b2PlaceIn(t, r)
	on, err := r.WriteDomainCopies(store.DomainCopiesWrite{Domain: "flash", PlaceID: p.ID})
	if err != nil {
		t.Fatal(err)
	}
	off := false
	row, err := r.WriteDomainCopies(store.DomainCopiesWrite{Domain: "flash", PlaceID: p.ID, TargetID: on.ID, Enabled: &off})
	if err != nil || row.Enabled {
		t.Fatalf("switched off = %+v, %v", row, err)
	}
	if s, err := r.GetSettings(); err != nil || s.FlashOffsite != "" || s.FlashOffsiteImmutable {
		t.Fatalf("the field = %q %v, %v, want it empty", s.FlashOffsite, s.FlashOffsiteImmutable, err)
	}
}

func TestAChipAtAPlaceWithoutTheDomainsFolderWritesNothing(t *testing.T) {
	r := newRepo(t)
	p := b2PlaceIn(t, r)
	skip := []string{store.SkipAll}
	if _, err := r.WriteDomainCopies(store.DomainCopiesWrite{Domain: "vms", PlaceID: p.ID, Skip: &skip}); !errors.Is(err, store.ErrPlaceDomainUnavailable) {
		t.Fatalf("WriteDomainCopies = %v, want ErrPlaceDomainUnavailable", err)
	}
	if _, found, err := r.PlacementDefaultFor("vms"); err != nil || found {
		t.Fatalf("a refused chip wrote the default: %v, %v", found, err)
	}
}
