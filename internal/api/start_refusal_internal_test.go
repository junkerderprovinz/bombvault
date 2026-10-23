package api

import (
	"context"
	"fmt"
	"testing"
)

// A section held by another task is refused with a code, so the page can say
// so in the person's language and tell it apart from a refusal that lasts.
func TestStartRefusalCodesASectionAnotherTaskHolds(t *testing.T) {
	cases := map[string]error{
		codeDomainBusy:  domainBusyError{op: "prune", domain: "containers"},
		codeOffsiteBusy: domainBusyError{op: offsiteLockReason, domain: "containers"},
		codeNoPlace:     ErrNoPlace,
		codeNoLocation:  &noLocationErr{domain: "flash"},
		codeReplicaBusy: fmt.Errorf("replicate tank: %w", errZFSReplicaBusy),
	}
	for want, err := range cases {
		if got := startRefusal(err)["code"]; got != want {
			t.Errorf("code of %v = %v, want %q", err, got, want)
		}
	}
}

// A refusal that lasts only while another run holds the thing carries a code
// on every route, not only on the ones that start a backup.
func TestAFailureBecauseSomethingElseHoldsItIsCoded(t *testing.T) {
	cases := map[string]error{
		codeReplicaBusy: errZFSReplicaBusy,
		codeCheckBusy:   errReceiverCheckBusy,
		codeDomainBusy:  domainBusyError{op: "prune", domain: "zfs"},
	}
	for want, err := range cases {
		if got := failEnvelope(err)["code"]; got != want {
			t.Errorf("code of %v = %v, want %q", err, got, want)
		}
	}
}

// A backup that has written its snapshot keeps the backup guard while it
// copies the snapshot off site. A start in that time is told so, not that a
// backup is still running.
func TestAStartDuringTheOffsiteCopyOfTheLastBackupSaysSo(t *testing.T) {
	h, _, sets := newMCPStartHandler(t, "docs")
	id := sets["docs"].ID
	h.svc.batchActive.Store(true)
	defer h.svc.batchActive.Store(false)
	unlock := h.svc.lockDomain("files")
	defer unlock()

	if started, err := h.svc.StartBackupFileSet(context.Background(), id); started || err != nil {
		t.Fatalf("while the backup itself runs: started=%v err=%v, want a plain refusal", started, err)
	}

	relabelled := h.svc.relabelDomain("files", offsiteLockReason)
	_, err := h.svc.StartBackupFileSet(context.Background(), id)
	if got := startRefusal(err)["code"]; got != codeOffsiteBusy {
		t.Errorf("during the off-site copy the code is %v, want %q", got, codeOffsiteBusy)
	}
	if !mcpStartBusy(err) {
		t.Errorf("an assistant reads %v as a failure, not as busy", err)
	}

	relabelled()
	if op, _ := h.svc.domainBusy("files"); op != "backup" {
		t.Errorf("after the copy the lock is labelled %q, want backup", op)
	}
}

// Only a lock someone holds is relabelled, or the domain would read busy
// after it is free.
func TestRelabellingAFreeDomainLeavesItFree(t *testing.T) {
	h, _, _ := newMCPStartHandler(t)
	h.svc.relabelDomain("files", offsiteLockReason)()
	if op, busy := h.svc.domainBusy("files"); busy {
		t.Errorf("a free domain reads busy with %q", op)
	}
}
