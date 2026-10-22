package api

import (
	"context"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The one target a domain copies to need not be the off-site wizard's: its own
// growth budget is the one that counts, also when the wizard's is off.
func TestTheOnlyTargetIsHeldToItsOwnGrowthBudget(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	b2.GrowthBudgetGB = 1
	if _, err := f.st.UpsertOffsiteTarget(b2); err != nil {
		t.Fatal(err)
	}
	f.replicated("containers")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:web"))
	if err := f.st.AddRepoStat(store.RepoStat{
		Domain: "containers", Source: "offsite", At: time.Now().Add(time.Hour).Unix(), RawSize: 2 << 30,
	}); err != nil {
		t.Fatal(err)
	}

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}

	f.svc.budgetMu.Lock()
	over := f.svc.offsiteOverBudget["containers"]
	f.svc.budgetMu.Unlock()
	if !over {
		t.Fatal("2 GiB at a target with a 1 GB budget did not count as over budget")
	}
}
