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
	if err := r.MarkOffsiteRunAgingOnly("t1", 2000); err != nil {
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
	if err := r.MarkOffsiteRunAgingOnly("t1", 500); err != nil {
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
	if err := r.MarkOffsiteRunAgingOnly("t1", 5); err == nil {
		t.Fatal("marking a run nobody recorded succeeded")
	}
}
