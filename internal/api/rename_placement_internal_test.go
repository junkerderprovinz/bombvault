package api

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// copiedTo lists the snapshot ids the runs so far copied to dest, sorted.
func copiedTo(f *placementFixture, dest string) []string {
	f.t.Helper()
	f.eng.mu.Lock()
	defer f.eng.mu.Unlock()
	var ids []string
	for _, c := range f.eng.copies {
		if c.Dest != dest {
			continue
		}
		if c.IDs == nil {
			f.t.Fatalf("a whole-repository copy from %s to %s", c.Src, dest)
		}
		ids = append(ids, c.IDs...)
	}
	slices.Sort(ids)
	return ids
}

func TestTakeoverAndUnlinkCarryTheCopyRule(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.rule("containers", "container:nginx", store.SkipAll)
	f.dock.installed = map[string]bool{"web": true}
	ctx := context.Background()

	if err := f.svc.TakeOverContainer(ctx, "nginx", "web"); err != nil {
		t.Fatalf("TakeOverContainer: %v", err)
	}
	if skip, ok := ruleOf(t, f, "containers", "container:web"); !ok || !slices.Equal(skip, []string{store.SkipAll}) {
		t.Fatalf("rule of web = %v (found %v), want [*]", skip, ok)
	}
	if _, ok := ruleOf(t, f, "containers", "container:nginx"); ok {
		t.Fatal("the former name kept a rule of its own")
	}

	if err := f.svc.UnlinkContainerAlias(ctx, "nginx"); err != nil {
		t.Fatalf("UnlinkContainerAlias: %v", err)
	}
	if skip, ok := ruleOf(t, f, "containers", "container:nginx"); !ok || !slices.Equal(skip, []string{store.SkipAll}) {
		t.Fatalf("rule of nginx after the unlink = %v (found %v), want [*]", skip, ok)
	}
}

func TestATakeoverOntoANameWithOnlyACopyRuleIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.rule("containers", "container:web", store.SkipAll)
	f.dock.installed = map[string]bool{"web": true}

	err := f.svc.TakeOverContainer(context.Background(), "nginx", "web")
	if !errors.Is(err, store.ErrCopyRuleTaken) {
		t.Fatalf("err = %v, want ErrCopyRuleTaken", err)
	}
	if _, err := f.st.GetTargetByContainer("nginx"); err != nil {
		t.Fatalf("the entry left its name after a refused takeover: %v", err)
	}
}

func TestADeletedEntryKeepsTheHistoryItTookOverOutOfTheTarget(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket/containers")
	f.listing("containers", b2.ID, 50)
	f.container("nginx", "")
	f.container("db", "")
	f.rule("containers", "container:nginx", store.SkipAll)
	if err := f.st.RenameTargetWithAlias("nginx", "web", ""); err != nil {
		t.Fatal(err)
	}
	if err := f.st.DeleteTarget("web"); err != nil {
		t.Fatal(err)
	}
	f.hold(f.domainPath("containers"),
		snap("aaaa0001", 100, "container:nginx"),
		snap("aaaa0002", 300, "container:web"),
		snap("aaaa0003", 300, "container:db"))

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatalf("ReplicateOffsite: %v", err)
	}
	if got, want := copiedTo(f, b2.Repo), []string{"aaaa0003"}; !slices.Equal(got, want) {
		t.Fatalf("copied %v to B2, want %v", got, want)
	}
}
