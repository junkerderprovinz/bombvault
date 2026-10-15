package api

import (
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A zvol goes into restic as a zfs send stream, which restic stores whole on
// every backup even when the disk did not change. That is no rewrite of the
// VM's data, while the same figures from file disks still are one.
func TestAVMThatStoresItsZvolsWholeIsNotHeldForARewrite(t *testing.T) {
	first := mkRun("first", anomalyNow-anomalyDay, "success", 20<<30, withSource(20<<30, 2), withSnapshot("s-first"))
	second := mkRun("second", anomalyNow, "success", 20<<30, withSource(20<<30, 2), withSnapshot("s-second"))
	in := newDataInput(after(second, []store.SeriesRun{first}), first.StartedAt)
	in.Domain = anomalyDomainVM
	isRewrite := func(f finding) bool { return f.Metric == metricNewDataRewrite }

	if found, _ := runNewData(in); !slices.ContainsFunc(found, isRewrite) {
		t.Fatalf("found %+v, want a rewrite for a VM whose file disks were stored again", found)
	}
	in.WholeCopies = true
	if found, _ := runNewData(in); slices.ContainsFunc(found, isRewrite) {
		t.Fatalf("found %+v, want no rewrite for a VM that stores its zvols whole", found)
	}
}

func TestOnlyAVMWithZvolDisksStoresWholeCopies(t *testing.T) {
	s := vmRestoreSvc(t, nil)
	for _, tg := range []store.VMTarget{
		vmTargetJSON(t, "zvolvm", vmrunDomainXML, []string{mixedvmDisk}, ""),
		vmTargetJSON(t, "filevm", vmDomainXML, []string{"/host/user/pool/domains/win10/win10.qcow2"}, ""),
	} {
		if _, err := s.store.UpsertVMTarget(tg); err != nil {
			t.Fatal(err)
		}
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	items, err := s.readAnomalyItems(settings)
	if err != nil {
		t.Fatal(err)
	}
	whole := map[string]bool{}
	for _, it := range items {
		if it.Domain == anomalyDomainVM {
			whole[it.Name] = it.WholeCopies
		}
	}
	if !whole["zvolvm"] || whole["filevm"] {
		t.Fatalf("whole copies by VM = %v, want only zvolvm", whole)
	}
}
