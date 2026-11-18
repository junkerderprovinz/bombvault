package api

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	kit "github.com/junkerderprovinz/parleyport/kit/notify"
	"github.com/junkerderprovinz/parleyport/kit/status"
)

type fakeHubFortress struct {
	members []fortress.Member
	caps    map[string][]string
	answers map[string]string
	asked   []string
	status  int
	bodies  []hubMessage
}

func (f *fakeHubFortress) Members() []fortress.Member { return f.members }

func (f *fakeHubFortress) Status(_ context.Context, id, _ string) (status.Report, error) {
	f.status++
	return status.Report{Capabilities: f.caps[id]}, nil
}

func (f *fakeHubFortress) Act(_ context.Context, id, action string, body []byte) ([]byte, error) {
	if action != actionNotifyDeliver {
		return nil, errors.New("unexpected action " + action)
	}
	f.asked = append(f.asked, id)
	var m hubMessage
	_ = json.Unmarshal(body, &m)
	f.bodies = append(f.bodies, m)
	a, ok := f.answers[id]
	if !ok {
		return nil, errors.New("away")
	}
	return []byte(a), nil
}

func tollRoll(id string, online bool, actions ...string) fortress.Member {
	return fortress.Member{ID: id, App: fortress.App{ID: "tollroll"}, Relay: online, Actions: actions}
}

func hubFortress() *fakeHubFortress {
	return &fakeHubFortress{
		members: []fortress.Member{
			{ID: "ship", App: fortress.App{ID: "shiplog"}, Relay: true, Actions: []string{actionNotifyDeliver}},
			tollRoll("asleep", false, actionNotifyDeliver),
			tollRoll("old", true),
			tollRoll("off", true, actionNotifyDeliver),
			tollRoll("first", true, actionNotifyDeliver),
			tollRoll("second", true, actionNotifyDeliver),
		},
		caps: map[string][]string{
			"ship": {capabilityNotifyHub}, "asleep": {capabilityNotifyHub}, "old": {capabilityNotifyHub},
			"first": {capabilityNotifyHub}, "second": {capabilityNotifyHub},
		},
		answers: map[string]string{"first": `{"taken":false,"reason":"hub_off"}`, "second": `{"taken":true}`},
	}
}

func TestOnlyATollRollThatSaysItIsTheHubIsOfferedMessages(t *testing.T) {
	f := hubFortress()
	var looks hubLooks
	got := looks.hubs(context.Background(), f, time.Now())
	if !slices.Equal(got, []string{"first", "second"}) {
		t.Fatalf("hubs = %v, want the online TollRolls with the action and the capability", got)
	}
	asked := f.status
	looks.hubs(context.Background(), f, time.Now().Add(30*time.Second))
	if f.status != asked {
		t.Errorf("the statuses were read again within a minute")
	}
	looks.hubs(context.Background(), f, time.Now().Add(2*time.Minute))
	if f.status == asked {
		t.Errorf("the statuses were not read again after a minute")
	}
}

func TestAMessageGoesToTheNextHubUntilOneTakesIt(t *testing.T) {
	f := hubFortress()
	e := kit.Event{Kind: "snapshot.failed", Severity: kit.SeverityFailure, Title: "snapshot schedule failed", Body: "tank/apps: busy", Key: "snapshot-plan:1a2b"}
	if !deliverToHubs(context.Background(), f, []string{"first", "second"}, e) {
		t.Fatal("the message counts as not taken")
	}
	if !slices.Equal(f.asked, []string{"first", "second"}) {
		t.Errorf("asked %v, want first then second", f.asked)
	}
	want := hubMessage{Kind: "snapshot.failed", Severity: "failure", Title: "snapshot schedule failed", Body: "tank/apps: busy", Key: "snapshot-plan:1a2b"}
	if f.bodies[1] != want {
		t.Errorf("the hub got %+v, want %+v", f.bodies[1], want)
	}
}

func TestNoHubTakingAMessageLeavesItToBombVault(t *testing.T) {
	f := hubFortress()
	f.answers = map[string]string{"first": `{"taken":false}`, "second": `not json`}
	if deliverToHubs(context.Background(), f, []string{"first", "second", "gone"}, kit.Event{Title: "x"}) {
		t.Error("a message no hub took counts as taken")
	}
}
