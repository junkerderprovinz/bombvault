package store_test

import (
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestMQTTSettingsStartFromDefaultsAndRoundTrip(t *testing.T) {
	r := newMCPRepo(t)
	got, err := r.GetMQTTSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, store.DefaultMQTTSettings()) {
		t.Fatalf("fresh settings = %+v, want the defaults", got)
	}
	want := store.MQTTSettings{
		Enabled: true, Host: "10.0.0.2", Port: 8883, Username: "bv", PasswordEnc: []byte{1, 2, 3},
		TLS: true, Prefix: "home/bv", Buttons: false, NodeID: "a1b2c3d4",
	}
	for range 2 {
		if err := r.SaveMQTTSettings(want); err != nil {
			t.Fatal(err)
		}
	}
	got, err = r.GetMQTTSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("read back %+v, want %+v", got, want)
	}
}
