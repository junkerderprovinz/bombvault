package relay

import (
	"bytes"
	"encoding/hex"
	"net/http"
	"testing"
	"time"
)

func TestSealedCallRoundTrips(t *testing.T) {
	key := DeriveFrameKey([]byte("a secret"))
	want := ProxyCall{
		Method: http.MethodPost,
		Path:   "/api/links",
		Body:   []byte(`{"url":"https://example.invalid/holiday-photos.zip"}`),
	}
	sealed, err := SealCall(key, "r1", "bravo", want)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	got, err := OpenCall(key, "r1", "bravo", sealed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if got.Method != want.Method || got.Path != want.Path {
		t.Errorf("opened %+v, want %+v", got, want)
	}
	if !bytes.Equal(got.Body, want.Body) {
		t.Errorf("body opened as %s, want it byte for byte", got.Body)
	}
	if got.ID != "r1" || time.Since(time.Unix(got.Sent, 0)) > time.Minute {
		t.Errorf("opened id %q sent at %d, want r1 stamped now", got.ID, got.Sent)
	}
}

// TestSealedFrameHidesItsContents checks the claim the connection card makes:
// nothing a relay operator would want appears in the bytes crossing the relay.
func TestSealedFrameHidesItsContents(t *testing.T) {
	key := DeriveFrameKey([]byte("a secret"))
	sealed, err := SealCall(key, "r1", "bravo", ProxyCall{
		Method: http.MethodPost,
		Path:   "/api/links",
		Body:   []byte(`{"url":"https://example.invalid/holiday-photos.zip"}`),
	})
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	for _, secret := range []string{
		"/api/links",
		"holiday-photos",
		"example.invalid",
		http.MethodPost,
	} {
		if bytes.Contains(sealed, []byte(secret)) {
			t.Errorf("the sealed frame contains %q in the clear", secret)
		}
	}
}

// TestRelayKeyDoesNotYieldTheFrameKey: the relay receives DeriveKey's output
// in every hello, so the frame key must not be computable from it.
func TestRelayKeyDoesNotYieldTheFrameKey(t *testing.T) {
	secret := []byte("a secret")
	relayKey := DeriveKey(secret)
	frameKey := DeriveFrameKey(secret)

	// The same hash without a separate domain would give the same bytes.
	if hex.EncodeToString(frameKey) == relayKey {
		t.Fatal("the frame key and the relay key are the same value, so the relay would hold both")
	}
	if len(frameKey) != 32 {
		t.Fatalf("frame key is %d bytes, want 32 for AES-256", len(frameKey))
	}
}

// TestSealIsBoundToItsRouting: RequestID and Target travel in the clear, and
// binding them into the AEAD stops a relay from delivering a sealed call to
// another instance and having it open there.
func TestSealIsBoundToItsRouting(t *testing.T) {
	key := DeriveFrameKey([]byte("a secret"))
	sealed, err := SealCall(key, "r1", "bravo", ProxyCall{Method: "GET", Path: "/api/tasks"})
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if _, err := OpenCall(key, "r1", "charlie", sealed); err == nil {
		t.Error("a call addressed to bravo opened as one addressed to charlie")
	}
	if _, err := OpenCall(key, "r2", "bravo", sealed); err == nil {
		t.Error("a call opened under a request id it was not sealed with")
	}
}

// TestSealedResultIsNotAcceptedAsACall: the directions use different labels
// in their additional data, so a relay cannot replay a peer's reply as a
// request addressed back at it.
func TestSealedResultIsNotAcceptedAsACall(t *testing.T) {
	key := DeriveFrameKey([]byte("a secret"))
	sealed, err := SealResult(key, "r1", ProxyResult{Status: 200, Body: []byte("ok")})
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if _, err := OpenCall(key, "r1", "bravo", sealed); err == nil {
		t.Error("a sealed result opened as a sealed call")
	}
}

// TestWrongKeyAndTamperingFail: a peer without the group's secret and a relay
// that edits a frame in flight both produce nothing that opens.
func TestWrongKeyAndTamperingFail(t *testing.T) {
	key := DeriveFrameKey([]byte("a secret"))
	other := DeriveFrameKey([]byte("a different secret"))
	sealed, err := SealCall(key, "r1", "bravo", ProxyCall{Method: "GET", Path: "/api/tasks"})
	if err != nil {
		t.Fatalf("seal: %v", err)
	}

	if _, err := OpenCall(other, "r1", "bravo", sealed); err == nil {
		t.Error("a frame opened under a key it was not sealed with")
	}

	for _, at := range []int{0, nonceLen, len(sealed) - 1} {
		tampered := bytes.Clone(sealed)
		tampered[at] ^= 0xff
		if _, err := OpenCall(key, "r1", "bravo", tampered); err == nil {
			t.Errorf("a frame with byte %d flipped still opened", at)
		}
	}

	if _, err := OpenCall(key, "r1", "bravo", sealed[:nonceLen-1]); err == nil {
		t.Error("a frame too short to hold a nonce still opened")
	}
	if _, err := OpenCall(key, "r1", "bravo", nil); err == nil {
		t.Error("an absent frame opened; an unsealed call must never look like a valid one")
	}
}

// TestNonceIsNotReused: a repeated nonce under AES-GCM repeats the keystream
// and leaks the authentication key.
func TestNonceIsNotReused(t *testing.T) {
	key := DeriveFrameKey([]byte("a secret"))
	call := ProxyCall{Method: "GET", Path: "/api/tasks"}
	seen := map[string]bool{}
	for i := 0; i < 128; i++ {
		sealed, err := SealCall(key, "r1", "bravo", call)
		if err != nil {
			t.Fatalf("seal: %v", err)
		}
		nonce := string(sealed[:nonceLen])
		if seen[nonce] {
			t.Fatal("the same nonce was used twice for the same key")
		}
		seen[nonce] = true
	}
}

// TestFrameKeyVectors pins the derivation to fixed bytes, so a change to the
// domain string or the hash breaks here instead of silently orphaning every
// sealed call in flight. Computed independently with sha256sum:
//
//	printf 'bombvault/relay/frame-key/v1' | sha256sum
//	printf 'bombvault/relay/frame-key/v1bombvault' | sha256sum
func TestFrameKeyVectors(t *testing.T) {
	cases := []struct {
		secret string
		want   string
	}{
		{"", "4b52777d3d6ac6d86936ab8d1d2aa30ef026740a28aaf46f1279b6675a45250d"},
		{"bombvault", "11d2cefdb6b133871a70aaede7c185dd71df9103e3f34fad3747ea206889970f"},
	}
	for _, tc := range cases {
		got := hex.EncodeToString(DeriveFrameKey([]byte(tc.secret)))
		if got != tc.want {
			t.Errorf("DeriveFrameKey(%q) = %s, want %s", tc.secret, got, tc.want)
		}
	}
}
