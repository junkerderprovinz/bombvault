package api

import "testing"

// A section held by another task is refused with a code, so the page can say
// so in the person's language and tell it apart from a refusal that lasts.
func TestStartRefusalCodesASectionAnotherTaskHolds(t *testing.T) {
	cases := map[string]error{
		codeDomainBusy: domainBusyError{op: "prune", domain: "containers"},
		codeNoPlace:    ErrNoPlace,
		codeNoLocation: &noLocationErr{domain: "flash"},
	}
	for want, err := range cases {
		if got := startRefusal(err)["code"]; got != want {
			t.Errorf("code of %v = %v, want %q", err, got, want)
		}
	}
}
