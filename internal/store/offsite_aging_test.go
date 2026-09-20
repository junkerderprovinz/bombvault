package store_test

import "testing"

func TestARunThatOnlyAgedATargetIsNoReplication(t *testing.T) {
	r := newRepo(t)
	copyRun, err := r.RecordOffsiteRunForTarget("files", "t1", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishOffsiteRun(copyRun, true, ""); err != nil {
		t.Fatal(err)
	}
	aging, err := r.RecordOffsiteRunForTarget("files", "t1", 2000)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.MarkOffsiteRunAgingOnly("files", "t1", 2000); err != nil {
		t.Fatalf("MarkOffsiteRunAgingOnly: %v", err)
	}
	if err := r.FinishOffsiteRun(aging, true, ""); err != nil {
		t.Fatal(err)
	}
	if run, found, err := r.LatestSuccessfulOffsiteRunForTarget("files", "t1"); err != nil || !found || run.StartedAt != 1000 {
		t.Errorf("per target = %+v found=%v err=%v, want the copy at 1000", run, found, err)
	}
	if run, found, err := r.LatestSuccessfulOffsiteRun("files"); err != nil || !found || run.StartedAt != 1000 {
		t.Errorf("per domain = %+v found=%v err=%v, want the copy at 1000", run, found, err)
	}
}

func TestAnAgingRunStillCountsAsHistoryForThePause(t *testing.T) {
	r := newRepo(t)
	id, err := r.RecordOffsiteRunForTarget("vms", "t1", 500)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.MarkOffsiteRunAgingOnly("vms", "t1", 500); err != nil {
		t.Fatal(err)
	}
	if err := r.FinishOffsiteRun(id, true, ""); err != nil {
		t.Fatal(err)
	}
	if _, offsite, err := r.DomainHasHistory("vms"); err != nil || !offsite {
		t.Fatalf("DomainHasHistory offsite = %v, %v, want true", offsite, err)
	}
}

func TestMarkingARunThatDoesNotExistFails(t *testing.T) {
	r := newRepo(t)
	if err := r.MarkOffsiteRunAgingOnly("files", "t1", 5); err == nil {
		t.Fatal("marking a run nobody recorded succeeded")
	}
}

func TestMarkingAnAgingRunStaysInItsOwnDomain(t *testing.T) {
	r := newRepo(t)
	// Both domains use target id "t1" here to stand in for the
	// settings-synthesized N=1 target, whose id is "" for every domain. The
	// files run is inserted first, so a match that ignores domain would take
	// the containers row's higher rowid instead.
	filesRun, err := r.RecordOffsiteRunForTarget("files", "t1", 1000)
	if err != nil {
		t.Fatal(err)
	}
	containersRun, err := r.RecordOffsiteRunForTarget("containers", "t1", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.MarkOffsiteRunAgingOnly("files", "t1", 1000); err != nil {
		t.Fatalf("MarkOffsiteRunAgingOnly: %v", err)
	}
	if err := r.FinishOffsiteRun(filesRun, true, ""); err != nil {
		t.Fatal(err)
	}
	if err := r.FinishOffsiteRun(containersRun, true, ""); err != nil {
		t.Fatal(err)
	}
	if _, found, err := r.LatestSuccessfulOffsiteRunForTarget("files", "t1"); err != nil || found {
		t.Errorf("files found=%v err=%v, want its run marked aging-only and out of the currency", found, err)
	}
	if run, found, err := r.LatestSuccessfulOffsiteRunForTarget("containers", "t1"); err != nil || !found || run.StartedAt != 1000 {
		t.Errorf("containers = %+v found=%v err=%v, want its copy to still count although files shares the same target and second", run, found, err)
	}
}
