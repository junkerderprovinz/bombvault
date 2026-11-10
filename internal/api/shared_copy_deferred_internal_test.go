package api

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A copy the owner of a shared repository holds off while it prunes moved
// nothing, so it must not read as a copy that found the target current.
func TestACopyTheOwnerHoldsOffIsDeferredAndLeavesTheCopyAsOldAsItWas(t *testing.T) {
	f := newPlacementFixture(t)
	tgt := f.target("files", "Shared with NAS", sharedTarget)
	if _, err := f.st.PutSharedRepo(store.SharedRepo{Repo: sharedTarget, Owner: "nas-id"}); err != nil {
		t.Fatal(err)
	}
	f.svc.sharedCopies.hush("nas-id", []string{sharedTarget}, time.Now().Add(time.Hour))

	if err := f.svc.ReplicateOffsite(context.Background(), "files"); err != nil {
		t.Fatalf("a deferred copy failed the pass: %v", err)
	}
	if _, found, err := f.st.LatestSuccessfulOffsiteRunForTarget("files", tgt.ID); err != nil || found {
		t.Fatalf("found a successful copy (%v, %v), want none", found, err)
	}
	run, found, err := f.st.LatestOffsiteRun("files")
	if err != nil || !found || !run.Deferred || run.OK {
		t.Fatalf("the target's run %+v (%v, %v), want one deferred", run, found, err)
	}
	if at, err := f.st.FirstOffsiteFailureAfter(tgt.ID, 0); err != nil || at != 0 {
		t.Fatalf("a failure since %d (%v), want none", at, err)
	}
	runs, err := f.st.ListRuns(5)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(runs, func(r store.Run) bool { return r.Kind == "offsite" })
	if i < 0 || runs[i].Status != "deferred" {
		t.Fatalf("the pass's run %+v, want it deferred", runs)
	}
}

func TestADeferredFirstCopyIsNotOverdue(t *testing.T) {
	f := newPlacementFixture(t)
	dailyContainers(t, f)
	tgt := f.target("containers", "Shared with NAS", sharedTarget)
	nginx := scheduledContainer(t, f, "nginx")
	now := time.Now().Unix()
	f.backupRun(nginx.ID, now-3600)
	id, err := f.st.RecordOffsiteRunForTarget("containers", tgt.ID, now-600)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.DeferOffsiteRun(id, "its owner is tidying it up"); err != nil {
		t.Fatal(err)
	}

	if st := statusFor(t, f, "containers"); st.ReplicationState != "never" {
		t.Fatalf("state %q, want never: the first copy waited and has its chance still", st.ReplicationState)
	}
}

func TestTheMemberCardSaysTheLastCopyWaitedForTheOwner(t *testing.T) {
	in := newInstance(t, "tower", strings.Repeat("b2", 32))
	if err := in.st.SaveSharedJoin(store.SharedJoin{Direction: store.SharedJoinOut, MemberID: "nas-id", MemberName: "nas",
		Host: towerHost, State: store.SharedJoinJoined, Repo: sharedTarget}); err != nil {
		t.Fatal(err)
	}
	tgt, err := in.st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Shared with nas", Repo: sharedTarget, Immutable: true, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	copied, err := in.st.RecordOffsiteRunForTarget("containers", tgt.ID, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if err := in.st.FinishOffsiteRun(copied, true, ""); err != nil {
		t.Fatal(err)
	}
	waited, err := in.st.RecordOffsiteRunForTarget("containers", tgt.ID, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if err := in.st.DeferOffsiteRun(waited, "its owner is tidying it up"); err != nil {
		t.Fatal(err)
	}

	views, err := in.svc.sharedMemberships()
	if err != nil || len(views) != 1 {
		t.Fatalf("memberships %+v, %v", views, err)
	}
	if v := views[0]; v.LastCopyAt != 1000 || v.DeferredAt != 2000 {
		t.Fatalf("last copy %d, deferred %d; want 1000 and 2000", v.LastCopyAt, v.DeferredAt)
	}
}
