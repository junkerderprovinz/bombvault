package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestWithdrawingAReceiverSwitchesOffWhatCopiesToItWithoutAskingIt(t *testing.T) {
	st := newTestStore(t)
	svc := &Service{store: st}
	there, err := st.SaveDestination(store.OffsiteTarget{Name: "attic", Repo: "rest:http://tower:secret@192.168.1.21:8000/tower/"})
	if err != nil {
		t.Fatal(err)
	}
	elsewhere, err := st.SaveDestination(store.OffsiteTarget{Name: "cloud", Repo: "rest:http://192.168.1.210:8000/tower"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.NoteReceiverLocation("m1", bareRestAddress("http://192.168.1.21:8000/tower")); err != nil {
		t.Fatal(err)
	}

	off, err := svc.switchOffReceiverDestinations("m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(off) != 1 || off[0].ID != there.ID {
		t.Fatalf("switched off %+v, want only %s", off, there.ID)
	}
	if d, _, _ := st.GetDestination(there.ID); d.Enabled {
		t.Fatal("the location on the receiving server is still on")
	}
	if d, _, _ := st.GetDestination(elsewhere.ID); !d.Enabled {
		t.Fatal("a location on another server was switched off")
	}
	if off, err := svc.switchOffReceiverDestinations("m2"); err != nil || len(off) != 0 {
		t.Fatalf("a member with nothing remembered switched off %+v, %v", off, err)
	}
}

func TestAReceiverThatAnsweredIsRememberedAtTheLocationsOnIt(t *testing.T) {
	st := newTestStore(t)
	svc := &Service{store: st}
	if _, err := st.SaveDestination(store.OffsiteTarget{Name: "attic", Repo: "rest:http://192.168.1.21:8000/tower"}); err != nil {
		t.Fatal(err)
	}
	svc.learnReceiverLocations([]groupReceiverView{
		{MemberID: "m1", URL: "http://192.168.1.21:8000"},
		{MemberID: "m2", URL: "http://192.168.1.2:8000"},
		{MemberID: "m3", NeedsAddress: true},
	})
	locs, err := st.ReceiverLocations()
	if err != nil {
		t.Fatal(err)
	}
	if len(locs) != 1 || len(locs["m1"]) != 1 || locs["m1"][0] != "http://192.168.1.21:8000/tower" {
		t.Fatalf("remembered %v, want the one location on m1's receiving server", locs)
	}
}
