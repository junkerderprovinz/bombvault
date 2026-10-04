package store_test

import "testing"

func TestTheNetworkAnnouncementIsOnUntilSwitchedOff(t *testing.T) {
	r := newMCPRepo(t)
	if on, err := r.MDNSEnabled(); err != nil || !on {
		t.Fatalf("a fresh install reads %v (%v), want on", on, err)
	}
	if err := r.SetMDNSEnabled(false); err != nil {
		t.Fatal(err)
	}
	if on, err := r.MDNSEnabled(); err != nil || on {
		t.Fatalf("after switching it off it reads %v (%v)", on, err)
	}
}
