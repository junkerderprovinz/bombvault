package api

import (
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/hostsnap"
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

func TestAFailedScheduleIsToldAndItsRecoveryClosesIt(t *testing.T) {
	p := store.HostSnapshotPlan{ID: "1a2b3c4d" + strings.Repeat("0", 24), Kind: "zfs", Source: "tank/apps"}

	ev, send := hostSnapPlanEvent(p, hostsnap.RunResult{Code: "busy", Detail: "dataset is busy"})
	if !send || ev.OK || ev.Key != "snapshot-plan:"+p.ID || !strings.Contains(ev.Message, "tank/apps") || !strings.Contains(ev.Message, "dataset is busy") {
		t.Errorf("failure = %+v, %v", ev, send)
	}
	if _, send := hostSnapPlanEvent(p, hostsnap.RunResult{Code: "ok"}); send {
		t.Error("a run that went through after one that did too is told")
	}
	if _, send := hostSnapPlanEvent(p, hostsnap.RunResult{Code: hostsnap.CodeSkippedAsleep}); send {
		t.Error("a run left out for sleeping disks is told")
	}
	p.LastCode = "busy"
	ev, send = hostSnapPlanEvent(p, hostsnap.RunResult{Code: "ok"})
	if !send || !ev.OK || ev.Key != "snapshot-plan:"+p.ID {
		t.Errorf("recovery = %+v, %v", ev, send)
	}
	ev, send = hostSnapPlanEvent(p, hostsnap.RunResult{Code: hostsnap.CodeKeptSome, Detail: "held"})
	if !send || !ev.OK || !ev.Warning {
		t.Errorf("a run that kept old snapshots = %+v, %v", ev, send)
	}
}
