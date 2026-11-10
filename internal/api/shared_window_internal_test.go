package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// sunday is 4 in the morning on a Sunday in March 2026, inside the weekly
// window holdSharedRepo sets, whose next one opens a week later at 3.
var (
	sunday     = time.Date(2026, time.March, 15, 4, 0, 0, 0, time.Local)
	nextSunday = time.Date(2026, time.March, 22, 3, 0, 0, 0, time.Local).Unix()
)

func TestTheMembersHearWithTheMeasurementWhenTheNextWindowOpens(t *testing.T) {
	eng := &upkeepEngine{whole: 1200, byHost: map[string]int64{towerHost: 1000}}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower)
	members := &fakeMembers{}

	if err := s.keepSharedRepo(context.Background(), sr, members, sunday, false); err != nil {
		t.Fatal(err)
	}
	cad, err := schedule.ParseCadence("weekly sun 03:00")
	if err != nil {
		t.Fatal(err)
	}
	next, _ := cad.NextDue(time.Time{}, time.Now())
	if got := members.told[towerHost]; got.Total != 1200 || got.NextWindow != next.Unix() {
		t.Fatalf("told %+v, want the space and the window of %d", got, next.Unix())
	}
}

func TestAPostponedPruneStillTellsTheMembersTheNextWindow(t *testing.T) {
	quickBusyWait(t)
	eng := &upkeepEngine{}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower, cube)
	if err := s.store.NoteSharedSpace(sr.ID, store.SharedSpace{At: 300, Total: 1200, Hosts: []store.SharedHostSpace{}}); err != nil {
		t.Fatal(err)
	}
	members := &fakeMembers{busy: map[string]int{cubeHost: -1}}

	if err := s.keepSharedRepo(context.Background(), sr, members, sunday, false); err != nil {
		t.Fatal(err)
	}
	if eng.measured != 0 {
		t.Fatal("measured after a postponed prune")
	}
	if got := members.told[towerHost]; got.At != 300 || got.Total != 1200 || got.NextWindow != nextSunday {
		t.Fatalf("told %+v, want the last numbers and the window of %d", got, nextSunday)
	}
}

func TestTheMemberCardShowsTheSpaceAndTheNextWindowTheOwnerTold(t *testing.T) {
	in := newInstance(t, "tower", strings.Repeat("b2", 32))
	if err := in.st.SaveSharedJoin(store.SharedJoin{Direction: store.SharedJoinOut, MemberID: "nas-id", MemberName: "nas",
		Host: towerHost, State: store.SharedJoinJoined, Repo: sharedTarget}); err != nil {
		t.Fatal(err)
	}
	if err := in.st.SetSharedSpaceFrom("nas-id", store.SharedSpace{At: 300, Total: 1200, NextWindow: nextSunday,
		Hosts: []store.SharedHostSpace{{Host: towerHost, Name: "tower", Bytes: 1000}}}); err != nil {
		t.Fatal(err)
	}

	views, err := in.svc.sharedMemberships()
	if err != nil || len(views) != 1 {
		t.Fatalf("memberships %+v, %v", views, err)
	}
	if v := views[0]; v.Space == nil || v.Space.Total != 1200 || v.NextWindow != nextSunday {
		t.Fatalf("membership %+v, want the owner's numbers and its next window", v)
	}
}

func TestAMemberCardWithoutNumbersYetShowsOnlyTheWindow(t *testing.T) {
	in := newInstance(t, "tower", strings.Repeat("b2", 32))
	if err := in.st.SaveSharedJoin(store.SharedJoin{Direction: store.SharedJoinOut, MemberID: "nas-id", MemberName: "nas",
		Host: towerHost, State: store.SharedJoinJoined, Repo: sharedTarget}); err != nil {
		t.Fatal(err)
	}
	if err := in.st.SetSharedSpaceFrom("nas-id", store.SharedSpace{NextWindow: nextSunday, Hosts: []store.SharedHostSpace{}}); err != nil {
		t.Fatal(err)
	}
	views, err := in.svc.sharedMemberships()
	if err != nil || len(views) != 1 || views[0].Space != nil || views[0].NextWindow != nextSunday {
		t.Fatalf("memberships %+v, %v; want the window and no numbers", views, err)
	}
}

func TestSavingTheRulesTellsTheMembersTheNewWindow(t *testing.T) {
	in := newInstance(t, "nas", strings.Repeat("a1", 32))
	self := store.SharedMember{Host: in.svc.resticHost(), Instance: in.id(t), Name: "nas"}
	sr, err := in.st.PutSharedRepo(store.SharedRepo{Repo: "backups/shared", Owner: in.id(t), Members: []store.SharedMember{self}})
	if err != nil {
		t.Fatal(err)
	}
	rules := map[string]any{"keepSet": false, "pruneCadence": "weekly sat 01:30", "pruneHours": 4, "maxUnused": 5, "checkSubset": 5}
	if code, out := in.do(t, http.MethodPut, "/api/shared-repos/"+sr.ID+"/upkeep", rules); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("PUT: %d %v", code, out)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		space, ok, err := in.st.SharedSpaceFrom(in.id(t))
		if err == nil && ok && space.NextWindow > time.Now().Unix() {
			if at := time.Unix(space.NextWindow, 0); at.Weekday() != time.Saturday || at.Hour() != 1 || at.Minute() != 30 {
				t.Fatalf("told the window %v, want a Saturday at 01:30", at)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the members never heard the new window: %+v, %v, %v", space, ok, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
