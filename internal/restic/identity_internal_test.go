package restic

import "testing"

func TestIdentityIsTheOriginalOfACopyAndTheIDOtherwise(t *testing.T) {
	if got := Identity(Snapshot{ID: "b9", Original: "a1"}); got != "a1" {
		t.Errorf("Identity of a copy = %q, want a1", got)
	}
	if got := Identity(Snapshot{ID: "a1"}); got != "a1" {
		t.Errorf("Identity of a backup = %q, want a1", got)
	}
}

func TestACopyOfACopyIsNotPendingWhereTheFirstCopyIs(t *testing.T) {
	src := []Snapshot{{ID: "c7", Original: "a1"}, {ID: "a2"}}
	dst := []Snapshot{{ID: "b9", Original: "a1"}}
	if got := PendingCopyIDs(src, dst); len(got) != 1 || got[0] != "a2" {
		t.Fatalf("PendingCopyIDs = %v, want [a2]", got)
	}
}
