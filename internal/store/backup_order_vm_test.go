package store

import "testing"

// TestSortVMTargetsForRun verifies the VM run ordering (#119, VMs): VMs with an
// explicit backup_order (>0) come first in ascending order, then the unordered
// ones (0) keep their incoming (name-sorted) order.
func TestSortVMTargetsForRun(t *testing.T) {
	vms := []VMTarget{
		{Name: "b", BackupOrder: 0},
		{Name: "z", BackupOrder: 2},
		{Name: "a", BackupOrder: 0},
		{Name: "y", BackupOrder: 1},
	}
	SortVMTargetsForRun(vms)

	got := []string{vms[0].Name, vms[1].Name, vms[2].Name, vms[3].Name}
	want := []string{"y", "z", "b", "a"} // explicit y(1),z(2) first; then unordered b,a in place
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SortVMTargetsForRun = %v, want %v", got, want)
		}
	}
}
