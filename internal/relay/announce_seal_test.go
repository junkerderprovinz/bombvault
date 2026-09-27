package relay

// The relay server reads Announce.InstanceID only, matching a reconnect,
// addressing a presence frame or picking a call target, and never the
// identity behind it. These tests pin that a peer's name only ever reaches a
// sibling that holds the same frame key, and never reaches the relay at all.

import (
	"bytes"
	"testing"
)

func TestIdentitySealRoundTrips(t *testing.T) {
	id := Identity{Name: "BOTTICH", Version: "1.4.0"}
	sealed, err := SealIdentity(testFrameKey, "alpha", id)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	got, err := OpenIdentity(testFrameKey, "alpha", sealed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if got != id {
		t.Errorf("opened %+v, want %+v", got, id)
	}
}

// TestTheSealedIdentityIsNotInTheEncodedFrame searches the marshalled bytes
// rather than comparing structs, since an empty field is also what a
// forgotten json tag produces.
func TestTheSealedIdentityIsNotInTheEncodedFrame(t *testing.T) {
	sealed, err := SealIdentity(testFrameKey, "alpha", Identity{Name: "jdp-workstation", Version: "2.1.0"})
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	frame, err := Encode(TypeAnnounce, Announce{InstanceID: "alpha", Sealed: sealed})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	for _, secret := range []string{"jdp-workstation", "2.1.0"} {
		if bytes.Contains(frame, []byte(secret)) {
			t.Errorf("the encoded announce contains %q in the clear:\n%s", secret, frame)
		}
	}
	if !bytes.Contains(frame, []byte("alpha")) {
		t.Errorf("the encoded announce lost the id the relay routes on:\n%s", frame)
	}
}

// TestAnIdentityMovedToAnotherInstanceDoesNotOpen checks the binding to the
// instance id: a relay that attaches alpha's sealed identity to another
// connection's announce gets a tag failure, not a machine listed under a
// false name.
func TestAnIdentityMovedToAnotherInstanceDoesNotOpen(t *testing.T) {
	sealed, err := SealIdentity(testFrameKey, "alpha", Identity{Name: "the NAS"})
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if _, err := OpenIdentity(testFrameKey, "bravo", sealed); err == nil {
		t.Error("an identity sealed for alpha opened under bravo's instance id")
	}
}

func TestWrongFrameKeyDoesNotOpenTheIdentity(t *testing.T) {
	sealed, err := SealIdentity(testFrameKey, "alpha", Identity{Name: "the NAS"})
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	other := make([]byte, 32)
	for i := range other {
		other[i] = 0x5a
	}
	if _, err := OpenIdentity(other, "alpha", sealed); err == nil {
		t.Error("an identity opened under a frame key it was not sealed with")
	}
}

// TestTwoRealClientsStillSeeEachOthersNames runs both sides end to end, which
// catches a seal applied on one side only.
func TestTwoRealClientsStillSeeEachOthersNames(t *testing.T) {
	addr, _ := relayOn(t, "127.0.0.1:0")
	const key = "shared-relay-test-key-0123456789ab"

	alpha := startClient(t, addr, key, "alpha", nil)
	bravo := startClient(t, addr, key, "bravo", nil)
	waitFor(t, "alpha to connect", alpha.Connected)
	waitFor(t, "bravo to connect", bravo.Connected)

	named := func(c *Client, id string) func() bool {
		return func() bool {
			for _, s := range c.Siblings() {
				if s.InstanceID == id {
					// startClient announces Name == id.
					return s.Name == id
				}
			}
			return false
		}
	}
	waitFor(t, "alpha to see bravo by name", named(alpha, "bravo"))
	waitFor(t, "bravo to see alpha by name", named(bravo, "alpha"))
}

// TestAnAnnounceThatDoesNotOpenIsDropped: a sibling that joins under a
// different frame key still joins the relay group, since the relay holds no
// frame key of its own, but a client that cannot open its identity must not
// list it as a sibling at all.
func TestAnAnnounceThatDoesNotOpenIsDropped(t *testing.T) {
	addr, _ := relayOn(t, "127.0.0.1:0")
	const key = "shared-relay-test-key-0123456789ab"
	alpha := startClient(t, addr, key, "alpha", nil)
	waitFor(t, "alpha to connect", alpha.Connected)

	wrongFrameKey := DeriveFrameKey([]byte("a different connection phrase"))
	sealed, err := SealIdentity(wrongFrameKey, "bravo", Identity{Name: "bravo"})
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	bravo := rawDial(t, "ws://"+addr+connectPath)
	writeFrame(t, bravo, TypeHello, Hello{Key: key, Announce: Announce{InstanceID: "bravo", Sealed: sealed}})
	readFrame(t, bravo, TypeAnnounce) // bravo's own view of alpha

	// charlie joins after bravo, so alpha seeing charlie proves it already
	// worked through bravo's announce and dropped it.
	startClient(t, addr, key, "charlie", nil)
	waitFor(t, "alpha to see charlie", func() bool {
		for _, s := range alpha.Siblings() {
			if s.InstanceID == "charlie" {
				return true
			}
		}
		return false
	})

	for _, s := range alpha.Siblings() {
		if s.InstanceID == "bravo" {
			t.Fatal("an announce that did not open under alpha's frame key was still listed as a sibling")
		}
	}
}
