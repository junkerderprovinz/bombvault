package nodelink

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/group"
)

func sampleHome(t *testing.T) Home {
	return Home{ID: "home-1", Key: homeKey(t), Relay: group.ModeOwn, RelayURL: "https://relay.example.org"}
}

func TestHomeComesBackAsItWasWritten(t *testing.T) {
	appKey := strings.Repeat("e", 64)
	want := sampleHome(t)
	raw, err := EncodeHome(appKey, want)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, "bvhome1.") {
		t.Errorf("NODE_HOME = %q, want it to start with bvhome1.", raw)
	}
	got, err := DecodeHome(appKey, raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != want.ID || !got.Key.Equal(want.Key) || got.Relay != want.Relay || got.RelayURL != want.RelayURL {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if again, err := DecodeHome(appKey, "  "+raw+"\n"); err != nil || again.ID != want.ID {
		t.Errorf("a value with blanks around it gave %+v %v", again, err)
	}
}

func TestHomeRefusesAValueThatWasChanged(t *testing.T) {
	appKey := strings.Repeat("e", 64)
	raw, err := EncodeHome(appKey, sampleHome(t))
	if err != nil {
		t.Fatal(err)
	}
	body, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(raw, "bvhome1."))
	if err != nil {
		t.Fatal(err)
	}
	for i := range body {
		changed := append([]byte(nil), body...)
		changed[i] ^= 1
		if _, err := DecodeHome(appKey, "bvhome1."+base64.RawURLEncoding.EncodeToString(changed)); err == nil {
			t.Fatalf("a value with byte %d changed was taken", i)
		}
	}
	for name, bad := range map[string]string{
		"another node's key": strings.Repeat("f", 64),
	} {
		if _, err := DecodeHome(bad, raw); err == nil {
			t.Errorf("%s took the value", name)
		}
	}
	for name, bad := range map[string]string{
		"another version": "bvhome2." + strings.TrimPrefix(raw, "bvhome1."),
		"no prefix":       strings.TrimPrefix(raw, "bvhome1."),
		"not base64":      "bvhome1.!!!",
		"too short":       "bvhome1.AAAA",
		"empty":           "",
	} {
		if _, err := DecodeHome(appKey, bad); err == nil {
			t.Errorf("%s was taken", name)
		}
	}
}

func TestHomeRefusesToWriteWhatANodeCannotUse(t *testing.T) {
	appKey := strings.Repeat("e", 64)
	for name, h := range map[string]Home{
		"no id":         {Key: homeKey(t), Relay: group.ModeProject},
		"a short key":   {ID: "home-1", Key: homeKey(t)[:8], Relay: group.ModeProject},
		"relay off":     {ID: "home-1", Key: homeKey(t), Relay: group.ModeOff},
		"own, no URL":   {ID: "home-1", Key: homeKey(t), Relay: group.ModeOwn},
		"unknown relay": {ID: "home-1", Key: homeKey(t), Relay: "mars"},
	} {
		if _, err := EncodeHome(appKey, h); err == nil {
			t.Errorf("%s was written", name)
		}
	}
}

func TestANodeIsKnownByTheIDItsLinkCallsIt(t *testing.T) {
	appKey := strings.Repeat("e", 64)
	id, err := NodeID(appKey)
	if err != nil {
		t.Fatal(err)
	}
	home, err := Open(Config{NodeKey: appKey, Seed: homeSeed, Relay: group.ModeOff})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = home.Close() }()
	if home.peer != id {
		t.Errorf("NodeID = %s, the link calls %s", id, home.peer)
	}
}

func TestNewNodeKeysAreAppKeys(t *testing.T) {
	a, err := NewNodeKey()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewNodeKey()
	if len(a) != 64 || strings.Trim(a, "0123456789abcdef") != "" || a == b {
		t.Errorf("NewNodeKey gave %q and %q", a, b)
	}
}
