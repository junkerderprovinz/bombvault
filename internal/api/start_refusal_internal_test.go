package api

import (
	"fmt"
	"testing"
)

// A section held by another task is refused with a code, so the page can say
// so in the person's language and tell it apart from a refusal that lasts.
func TestStartRefusalCodesASectionAnotherTaskHolds(t *testing.T) {
	cases := map[string]error{
		codeDomainBusy:  domainBusyError{op: "prune", domain: "containers"},
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
