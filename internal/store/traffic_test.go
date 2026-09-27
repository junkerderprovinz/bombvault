package store_test

import (
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestTrafficSettingsStartAtTheDefaults(t *testing.T) {
	r := newRepo(t)
	got, err := r.TrafficSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, store.DefaultTrafficSettings()) {
		t.Fatalf("got %+v, want the defaults", got)
	}
	if got.MediaServers != nil {
		t.Fatal("an install that never chose has a media server list")
	}
}

func TestTrafficSettingsRoundTrip(t *testing.T) {
	r := newRepo(t)
	want := store.TrafficSettings{StreamThrottle: true, MediaServers: []string{"plex", "jellyfin"}, StreamMbit: 8, StreamLimitKiB: 256, StreamHoldMin: 10}
	if err := r.SetTrafficSettings(want); err != nil {
		t.Fatal(err)
	}
	got, err := r.TrafficSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestAnEmptyMediaServerListStaysAChoice(t *testing.T) {
	r := newRepo(t)
	s := store.DefaultTrafficSettings()
	s.MediaServers = []string{}
	if err := r.SetTrafficSettings(s); err != nil {
		t.Fatal(err)
	}
	got, err := r.TrafficSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.MediaServers == nil || len(got.MediaServers) != 0 {
		t.Fatalf("media servers = %#v, want an empty list", got.MediaServers)
	}
}

func TestIdleWaitHoursAreKeptPerTargetAndZeroSwitchesThemOff(t *testing.T) {
	r := newRepo(t)
	if err := r.SetIdleWaitHours("t1", 4); err != nil {
		t.Fatal(err)
	}
	if err := r.SetIdleWaitHours("t2", 2); err != nil {
		t.Fatal(err)
	}
	if err := r.SetIdleWaitHours("t2", 6); err != nil {
		t.Fatal(err)
	}
	got, err := r.IdleWaitHours()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, map[string]int{"t1": 4, "t2": 6}) {
		t.Fatalf("got %v", got)
	}
	if err := r.SetIdleWaitHours("t1", 0); err != nil {
		t.Fatal(err)
	}
	got, _ = r.IdleWaitHours()
	if _, ok := got["t1"]; ok {
		t.Fatalf("t1 still waits: %v", got)
	}
}

func TestIdleSettingsRoundTrip(t *testing.T) {
	r := newRepo(t)
	s := store.DefaultTrafficSettings()
	s.IdleCPUPct, s.IdleNetMbit, s.IdleQuietMin = 25, 4, 10
	if err := r.SetTrafficSettings(s); err != nil {
		t.Fatal(err)
	}
	got, err := r.TrafficSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.IdleCPUPct != 25 || got.IdleNetMbit != 4 || got.IdleQuietMin != 10 {
		t.Fatalf("got %+v", got)
	}
}

func TestAContainerWithoutBackupsCanWaitForIdle(t *testing.T) {
	r := newRepo(t)
	if err := r.SetContainerIdleWait("plex", 3); err != nil {
		t.Fatal(err)
	}
	tg, err := r.GetTargetByContainer("plex")
	if err != nil {
		t.Fatalf("no target row: %v", err)
	}
	got, _ := r.IdleWaitHours()
	if got[tg.ID] != 3 {
		t.Fatalf("hours = %v", got)
	}
}
