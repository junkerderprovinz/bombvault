package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestNamedRepoOffSiteMarkRoundTrips pins the column behind the "already off
// site" switch on the Repositories card. A mark that is not stored is a switch
// that flips back on the next reload, while replication keeps copying the share
// the operator just said is their second copy.
func TestNamedRepoOffSiteMarkRoundTrips(t *testing.T) {
	st := namedRepoStore(t)
	saved, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RoleRepo, Name: "NAS", Repo: "remotes/nas/cold", Enabled: true, AlreadyOffsite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := st.GetNamedRepo(saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.AlreadyOffsite {
		t.Fatal("the off-site mark was not stored")
	}

	got.AlreadyOffsite = false
	if _, err := st.UpsertOffsiteTarget(got); err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListNamedRepos()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].AlreadyOffsite {
		t.Fatalf("clearing the mark did not stick: %+v", rows)
	}
}
