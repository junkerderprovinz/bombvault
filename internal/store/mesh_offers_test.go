package store_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestMeshOfferEmptyRepoRejected pins the empty-repo guard on Create: an
// offer addressing nowhere is refused with ErrEmptyMeshOffer, writing nothing.
func TestMeshOfferEmptyRepoRejected(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	for _, repo := range []string{"", "   "} {
		if _, err := r.CreateMeshOffer(store.MeshOffer{From: "tower", Repo: repo}); !errors.Is(err, store.ErrEmptyMeshOffer) {
			t.Fatalf("CreateMeshOffer(repo=%q) err = %v, want ErrEmptyMeshOffer", repo, err)
		}
	}
	all, err := r.ListMeshOffers()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("a rejected create must write nothing, got %d rows", len(all))
	}
}

// TestMeshOfferCRUD exercises Create/Get/List/UpdateStatus/Delete and, like
// fleet_peers/received_repos, that the peer-generated REST password round-
// trips as ENCRYPTED bytes. Status defaults to "pending" and can be
// transitioned to "accepted"/"declined".
func TestMeshOfferCRUD(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	appKey := strings.Repeat("ab", 32)
	password := "onetime-rest-server-password"
	enc, err := secret.Encrypt(appKey, []byte(password))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if strings.Contains(string(enc), password) {
		t.Fatal("stored rest_password_enc must not contain the plaintext password")
	}

	in := store.MeshOffer{
		From:            "tower-b",
		SuggestedDomain: "containers",
		Repo:            "rest:http://192.168.1.50:8000/bombvault-containers/containers",
		RESTUser:        "bombvault-containers",
		RESTPasswordEnc: enc,
	}
	got, err := r.CreateMeshOffer(in)
	if err != nil {
		t.Fatalf("CreateMeshOffer: %v", err)
	}
	if got.ID == "" {
		t.Fatal("CreateMeshOffer did not assign an ID")
	}
	if got.ReceivedAt == 0 {
		t.Fatal("CreateMeshOffer did not stamp ReceivedAt")
	}
	if got.Status != "pending" {
		t.Fatalf("want default status 'pending', got %q", got.Status)
	}

	back, ok, err := r.GetMeshOffer(got.ID)
	if err != nil || !ok {
		t.Fatalf("GetMeshOffer: ok=%v err=%v", ok, err)
	}
	if back.From != "tower-b" || back.SuggestedDomain != "containers" || back.Repo != in.Repo || back.RESTUser != in.RESTUser {
		t.Fatalf("GetMeshOffer round-trip mismatch: %+v", back)
	}
	dec, err := secret.Decrypt(appKey, back.RESTPasswordEnc)
	if err != nil || string(dec) != password {
		t.Fatalf("stored password did not decrypt back to the original: dec=%q err=%v", dec, err)
	}

	all, err := r.ListMeshOffers()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].ID != got.ID {
		t.Fatalf("ListMeshOffers = %+v, want exactly the one created offer", all)
	}

	if err := r.UpdateMeshOfferStatus(got.ID, "accepted"); err != nil {
		t.Fatalf("UpdateMeshOfferStatus: %v", err)
	}
	updated, ok, err := r.GetMeshOffer(got.ID)
	if err != nil || !ok {
		t.Fatalf("GetMeshOffer after status update: ok=%v err=%v", ok, err)
	}
	if updated.Status != "accepted" {
		t.Fatalf("want status 'accepted', got %q", updated.Status)
	}
	// Everything else must be untouched by a status-only write.
	if updated.From != in.From || updated.Repo != in.Repo || updated.RESTUser != in.RESTUser {
		t.Fatalf("UpdateMeshOfferStatus must not touch other columns: %+v", updated)
	}

	if err := r.DeleteMeshOffer(got.ID); err != nil {
		t.Fatalf("DeleteMeshOffer: %v", err)
	}
	if _, ok, err := r.GetMeshOffer(got.ID); err != nil || ok {
		t.Fatalf("GetMeshOffer after delete: ok=%v err=%v, want ok=false", ok, err)
	}
	// Deleting a missing id is a harmless no-op.
	if err := r.DeleteMeshOffer("does-not-exist"); err != nil {
		t.Fatalf("DeleteMeshOffer(missing id): %v", err)
	}
}
