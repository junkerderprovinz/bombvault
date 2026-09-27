package store_test

import (
	"fmt"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestSizeBreakdownsKeepTheNewestThreePerItem(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	for i := range 5 {
		b := store.SizeBreakdown{TargetID: "t1", SnapshotID: fmt.Sprintf("s%d", i), Domain: "containers", CreatedAt: int64(i), Tree: "{}"}
		if err := r.PutSizeBreakdown(b); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.PutSizeBreakdown(store.SizeBreakdown{TargetID: "t2", SnapshotID: "x", Domain: "vms", Partial: true, Tree: "{}"}); err != nil {
		t.Fatal(err)
	}
	for i, want := range []bool{false, false, true, true, true} {
		if _, ok, err := r.GetSizeBreakdown("t1", fmt.Sprintf("s%d", i)); err != nil || ok != want {
			t.Errorf("s%d kept = %v (%v), want %v", i, ok, err, want)
		}
	}
	b, ok, err := r.GetSizeBreakdown("t2", "x")
	if err != nil || !ok || !b.Partial || b.Domain != "vms" {
		t.Fatalf("t2 = %+v %v %v", b, ok, err)
	}
	if d, ok, _ := r.SizeBreakdownDomain("t2"); !ok || d != "vms" {
		t.Fatalf("domain %q %v", d, ok)
	}
	if _, ok, _ := r.SizeBreakdownDomain("none"); ok {
		t.Fatal("an item without a breakdown has one")
	}
}
