package api

import (
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAPlanComesDueAtItsSlotAndNotBefore(t *testing.T) {
	made := time.Date(2026, 11, 17, 2, 0, 0, 0, time.Local)
	p := store.HostSnapshotPlan{ID: "a", Cadence: "daily 03:00", Keep: 3, Enabled: true, CreatedAt: made.Unix()}
	off := p
	off.ID, off.Enabled = "b", false

	if due := dueHostSnapshotPlans([]store.HostSnapshotPlan{p, off}, made.Add(30*time.Minute)); len(due) != 0 {
		t.Errorf("due before the slot: %v", due)
	}
	due := dueHostSnapshotPlans([]store.HostSnapshotPlan{p, off}, made.Add(70*time.Minute))
	if len(due) != 1 || due[0].ID != "a" {
		t.Fatalf("due after the slot = %v, want the switched-on plan", due)
	}
	p.LastRunAt = made.Add(61 * time.Minute).Unix()
	if due := dueHostSnapshotPlans([]store.HostSnapshotPlan{p}, made.Add(5*time.Hour)); len(due) != 0 {
		t.Errorf("due again on the same day: %v", due)
	}
}
