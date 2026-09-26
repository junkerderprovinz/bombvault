package store_test

import (
	"database/sql"
	"errors"
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
