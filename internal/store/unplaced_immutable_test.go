package store_test

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAnUnplacedTargetTakesItsAppendOnlyFlagAndHandsItToItsDirectRepository(t *testing.T) {
	r := newRepo(t)
	target, err := r.CreateOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "B2", Repo: "b2:bucket:vms", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	direct, err := r.CreateCompanionRepo(target.ID, "B2 direct", "b2:bucket:vms-direct")
	if err != nil {
		t.Fatal(err)
	}

	row, err := r.SetUnplacedImmutable(target.ID, true)
	if err != nil || !row.Immutable {
		t.Fatalf("SetUnplacedImmutable = %+v, %v", row, err)
	}
	if got, _ := r.GetNamedRepo(direct.ID); !got.Immutable {
		t.Fatal("the direct repository did not follow its target")
	}
	if _, err := r.SetUnplacedImmutable(direct.ID, false); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("a direct repository's own flag = %v, want sql.ErrNoRows", err)
	}
}

func TestTheFieldRowOfAnUnplacedTargetCarriesItsFlagIntoTheField(t *testing.T) {
	r := newRepo(t)
	settings, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.VMsOffsite = "b2:bucket:vms"
	if err := r.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	field, err := r.CreateOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "Primary", Repo: "b2:bucket:vms", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.MakeFieldOffsiteTarget(field.ID); err != nil {
		t.Fatal(err)
	}
	other, err := r.CreateOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "Swift", Repo: "swift:box:vms", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := r.SetUnplacedImmutable(other.ID, true); err != nil {
		t.Fatal(err)
	}
	if s, _ := r.GetSettings(); s.VMsOffsiteImmutable {
		t.Fatal("a target behind the field row wrote the field's flag")
	}
	if _, err := r.SetUnplacedImmutable(field.ID, true); err != nil {
		t.Fatal(err)
	}
	if s, _ := r.GetSettings(); !s.VMsOffsiteImmutable {
		t.Fatal("the field row's flag did not reach the field")
	}
}

func TestAPlacedRowKeepsTheFlagItsPlaceWrote(t *testing.T) {
	r := newRepo(t)
	p, err := r.WritePlace(store.PlaceWrite{Place: store.Place{Name: "B2", Provider: "b2", Kind: "s3", Base: "s3:https://s3.example.com/bucket",
		Folders: map[string]string{"vms": "vms"}, Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	placed, err := r.CreatePlaceRepo(p.ID, "vms", "")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := r.SetUnplacedImmutable(placed.ID, true); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SetUnplacedImmutable of a placed row = %v, want sql.ErrNoRows", err)
	}
	if got, _ := r.GetNamedRepo(placed.ID); got.Immutable {
		t.Fatal("a placed row took a flag of its own")
	}
}
