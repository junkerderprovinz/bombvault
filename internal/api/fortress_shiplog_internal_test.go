package api

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
)

// fakeShipLog is a fortress whose ShipLog answers updates.managed with answer.
type fakeShipLog struct {
	members []fortress.Member
	answer  string
	fail    error
	asked   int
}

func (f *fakeShipLog) Members() []fortress.Member { return f.members }

func (f *fakeShipLog) Act(_ context.Context, id, action string, body []byte) ([]byte, error) {
	if action != "updates.managed" || string(body) != "{}" {
		return nil, fmt.Errorf("asked %s %s %s", id, action, body)
	}
	f.asked++
	return []byte(f.answer), f.fail
}

func TestShipLogKeepsTheContainersItUpdatesItself(t *testing.T) {
	ship := &fakeShipLog{
		members: []fortress.Member{{ID: "s1", App: fortress.App{ID: "shiplog"}, Device: "Tower"}},
		answer:  `{"autoUpdate":true,"containers":["Nextcloud","web"]}`,
	}
	ctx := context.Background()
	if !shipLogUpdatesContainer(ctx, ship, server{device: "tower"}, "nextcloud") {
		t.Fatal("a container on ShipLog's list was left to BombVault")
	}
	if shipLogUpdatesContainer(ctx, ship, server{device: "tower"}, "plex") {
		t.Fatal("a container ShipLog does not update was held back")
	}
	if ship.asked != 2 {
		t.Fatalf("asked ShipLog %d times, want once for each step", ship.asked)
	}

	ship.answer = `{"autoUpdate":false,"containers":[]}`
	if shipLogUpdatesContainer(ctx, ship, server{device: "tower"}, "nextcloud") {
		t.Fatal("held back while ShipLog's auto-update is off")
	}
	ship.answer, ship.fail = "", errors.New("unreachable")
	if shipLogUpdatesContainer(ctx, ship, server{device: "tower"}, "nextcloud") {
		t.Fatal("held back while ShipLog does not answer")
	}
	if shipLogUpdatesContainer(ctx, &fakeShipLog{members: ship.members, answer: `{"autoUpdate":true,"containers":["nextcloud"]}`}, server{device: "barn"}, "nextcloud") {
		t.Fatal("ShipLog on another server held this one back")
	}
}

func TestShipLogOnThisServerIsFoundByTheServerWhateverItsName(t *testing.T) {
	ctx := context.Background()
	answer := `{"autoUpdate":true,"containers":["nextcloud"]}`
	renamed := &fakeShipLog{members: []fortress.Member{{ID: "s1", App: fortress.App{ID: "shiplog"}, Device: "Shipyard", Host: "Tower"}}, answer: answer}
	if !shipLogUpdatesContainer(ctx, renamed, server{host: "tower", device: "My vault"}, "nextcloud") {
		t.Fatal("ShipLog on this server was missed under another name")
	}
	elsewhere := &fakeShipLog{members: []fortress.Member{{ID: "s1", App: fortress.App{ID: "shiplog"}, Device: "Tower", Host: "Barn"}}, answer: answer}
	if shipLogUpdatesContainer(ctx, elsewhere, server{host: "tower", device: "Tower"}, "nextcloud") {
		t.Fatal("ShipLog on another server held this one back because the names match")
	}
}
