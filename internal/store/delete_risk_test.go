package store_test

import (
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestARiskIsAcceptedOnlyForATargetOrADestination(t *testing.T) {
	r := newRepo(t)
	target, err := r.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "OneDrive", Repo: "rclone:onedrive:bv", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetDeleteRiskAccepted(target.ID, true); err != nil {
		t.Fatal(err)
	}
	got, ok, err := r.GetOffsiteTarget(target.ID)
	if err != nil || !ok || !got.DeleteRiskAccepted {
		t.Fatalf("target = %+v, ok=%v err=%v, want it accepted", got, ok, err)
	}

	repo, err := r.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: "NAS", Repo: "sftp:u@nas:/bv", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetDeleteRiskAccepted(repo.ID, true); !errors.Is(err, store.ErrNotOffsiteTarget) {
		t.Errorf("accepting a named repository = %v, want ErrNotOffsiteTarget", err)
	}
	if err := r.SetDeleteRiskAccepted("nobody", true); !errors.Is(err, store.ErrNotOffsiteTarget) {
		t.Errorf("accepting an unknown id = %v, want ErrNotOffsiteTarget", err)
	}
}
