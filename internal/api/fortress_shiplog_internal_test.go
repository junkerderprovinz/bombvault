package api

import (
	"testing"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
)

func TestShipLogOnTheSameServerTakesTheUpdatesOver(t *testing.T) {
	shipLogOn := func(device string) fortress.Member {
		return fortress.Member{ID: "s1", App: fortress.App{ID: "shiplog"}, Device: device}
	}
	for _, c := range []struct {
		members []fortress.Member
		want    bool
	}{
		{[]fortress.Member{shipLogOn("Tower")}, true},
		{[]fortress.Member{shipLogOn("tower.lan")}, true},
		{[]fortress.Member{shipLogOn("barn")}, false},
		{[]fortress.Member{{ID: "t1", App: fortress.App{ID: "towertorch"}, Device: "tower"}}, false},
		{nil, false},
	} {
		if got := shipLogUpdatesOn(c.members, "tower"); got != c.want {
			t.Errorf("%+v: %v, want %v", c.members, got, c.want)
		}
	}
	if shipLogUpdatesOn([]fortress.Member{shipLogOn("")}, "") {
		t.Error("a member without a device counted as this server")
	}
}
