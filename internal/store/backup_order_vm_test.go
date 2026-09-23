package store

import "testing"

// TestSortVMTargetsForRun expects VMs with an explicit backup_order first, in
// ascending order, and the unordered ones (0) in their incoming order.
func TestSortVMTargetsForRun(t *testing.T) {
	vms := []VMTarget{
		{Name: "b", BackupOrder: 0},
		{Name: "z", BackupOrder: 2},
		{Name: "a", BackupOrder: 0},
		{Name: "y", BackupOrder: 1},
	}
	SortVMTargetsForRun(vms)

	got := []string{vms[0].Name, vms[1].Name, vms[2].Name, vms[3].Name}
	want := []string{"y", "z", "b", "a"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SortVMTargetsForRun = %v, want %v", got, want)
		}
	}
}
