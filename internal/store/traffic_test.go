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
