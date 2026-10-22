package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

const passkeyHost = "bombvault.example.com"

// virtualKey is a security key in software: it signs what a browser would
// hand the finish call, with or without having checked a PIN.
type virtualKey struct {
	priv *ecdsa.PrivateKey
	id   []byte
}

func registerVirtualKey(t *testing.T, repo *store.Repo) virtualKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	k := virtualKey{priv: priv, id: []byte("virtual-key-1")}
	pub, err := priv.PublicKey.ECDH()
	if err != nil {
		t.Fatal(err)
	}
	raw := pub.Bytes()
	cose, err := webauthncbor.Marshal(webauthncose.EC2PublicKeyData{
		PublicKeyData: webauthncose.PublicKeyData{KeyType: int64(webauthncose.EllipticKey), Algorithm: int64(webauthncose.AlgES256)},
		Curve:         1,
		XCoord:        raw[1:33],
		YCoord:        raw[33:],
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddPasskey(store.Passkey{Name: "Stick", CredentialID: k.id, PublicKey: cose, RPID: passkeyHost}); err != nil {
		t.Fatal(err)
	}
	return k
}

// answer signs the challenge of a login that began with options.
func (k virtualKey) answer(t *testing.T, challenge string, verified bool) json.RawMessage {
	t.Helper()
	b64 := base64.RawURLEncoding.EncodeToString
	client, err := json.Marshal(map[string]any{"type": "webauthn.get", "challenge": challenge, "origin": "https://" + passkeyHost})
	if err != nil {
		t.Fatal(err)
	}
	rp := sha256.Sum256([]byte(passkeyHost))
	flags := byte(0x01)
	if verified {
		flags |= 0x04
	}
	auth := binary.BigEndian.AppendUint32(append(rp[:], flags), 0)
	clientHash := sha256.Sum256(client)
	digest := sha256.Sum256(append(append([]byte{}, auth...), clientHash[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, k.priv, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(map[string]any{
		"id": b64(k.id), "rawId": b64(k.id), "type": "public-key",
		"response": map[string]any{"clientDataJSON": b64(client), "authenticatorData": b64(auth), "signature": b64(sig)},
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func passkeyCall(t *testing.T, h http.HandlerFunc, body any, remoteAddr string, header http.Header) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/login", strings.NewReader(string(b)))
	r.Host = passkeyHost
	r.RemoteAddr = remoteAddr
	r.Header.Set("Content-Type", "application/json")
	for k, vs := range header {
		r.Header[k] = vs
	}
	w := httptest.NewRecorder()
	h(w, r)
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("answer is not JSON: %s", w.Body.String())
	}
	return w, out
}

// beginPasskeyLogin starts a login and returns its handle and challenge.
func beginPasskeyLogin(t *testing.T, h *Handler, remoteAddr string, header http.Header) (id, challenge, uv string) {
	t.Helper()
	_, out := passkeyCall(t, h.handlePasskeyLoginBegin, map[string]any{}, remoteAddr, header)
	options, _ := out["options"].(map[string]any)
	if out["ok"] != true || options == nil {
		t.Fatalf("begin: %v", out)
	}
	uv, _ = options["userVerification"].(string)
	return out["ceremonyId"].(string), options["challenge"].(string), uv
}

func issuedSession(w *httptest.ResponseRecorder) bool {
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookieName && c.Value != "" {
			return true
		}
	}
	return false
}

func TestAPasskeyThatOnlyProvesATouchCannotSkipAnArmedSecondFactor(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)
	storeTOTP(t, h, repo, "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", nil)
	key := registerVirtualKey(t, repo)

	id, challenge, uv := beginPasskeyLogin(t, h, "10.0.0.1:1", nil)
	if uv != "required" {
		t.Fatalf("the login asked for user verification %q, want required", uv)
	}
	w, out := passkeyCall(t, h.handlePasskeyLoginFinish, map[string]any{"ceremonyId": id, "credential": key.answer(t, challenge, false)}, "10.0.0.1:1", nil)
	if out["code"] != codePasskeyNeedsVerification || issuedSession(w) {
		t.Fatalf("a touch alone with two-factor on: %v", out)
	}

	id, challenge, _ = beginPasskeyLogin(t, h, "10.0.0.1:1", nil)
	w, out = passkeyCall(t, h.handlePasskeyLoginFinish, map[string]any{"ceremonyId": id, "credential": key.answer(t, challenge, true)}, "10.0.0.1:1", nil)
	if out["ok"] != true || !issuedSession(w) {
		t.Fatalf("a key that checked its PIN with two-factor on: %v", out)
	}
}

func TestASecondFactorArmedDuringAPasskeyLoginIsHeldTo(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)
	key := registerVirtualKey(t, repo)

	id, challenge, _ := beginPasskeyLogin(t, h, "10.0.0.1:1", nil)
	storeTOTP(t, h, repo, "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", nil)
	w, out := passkeyCall(t, h.handlePasskeyLoginFinish, map[string]any{"ceremonyId": id, "credential": key.answer(t, challenge, false)}, "10.0.0.1:1", nil)
	if out["code"] != codePasskeyNeedsVerification || issuedSession(w) {
		t.Fatalf("a touch alone once two-factor came on: %v", out)
	}
}

func TestAPasskeyThatOnlyProvesATouchSignsInWithoutASecondFactor(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)
	key := registerVirtualKey(t, repo)

	id, challenge, _ := beginPasskeyLogin(t, h, "10.0.0.1:1", nil)
	w, out := passkeyCall(t, h.handlePasskeyLoginFinish, map[string]any{"ceremonyId": id, "credential": key.answer(t, challenge, false)}, "10.0.0.1:1", nil)
	if out["ok"] != true || !issuedSession(w) {
		t.Fatalf("a touch with two-factor off: %v", out)
	}
}

func TestPasskeyFailuresBehindATrustedProxyCountAgainstTheirOwnClient(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	_, proxies, err := net.ParseCIDR("10.0.0.2/32")
	if err != nil {
		t.Fatal(err)
	}
	h.cfg.TrustedProxies = []net.IPNet{*proxies}
	enableAuth(t, h, repo)
	key := registerVirtualKey(t, repo)
	attacker := http.Header{"X-Forwarded-For": {"203.0.113.7"}}
	operator := http.Header{"X-Forwarded-For": {"203.0.113.8"}}

	for range loginMaxFails {
		passkeyCall(t, h.handlePasskeyLoginFinish, map[string]any{"ceremonyId": "made-up"}, "10.0.0.2:1", attacker)
	}
	id, challenge, _ := beginPasskeyLogin(t, h, "10.0.0.2:1", operator)
	w, out := passkeyCall(t, h.handlePasskeyLoginFinish, map[string]any{"ceremonyId": id, "credential": key.answer(t, challenge, false)}, "10.0.0.2:1", operator)
	if w.Code != http.StatusOK || out["ok"] != true {
		t.Fatalf("the operator after somebody else's bad tries: %d %v", w.Code, out)
	}
	if w, _ := passkeyCall(t, h.handlePasskeyLoginFinish, map[string]any{"ceremonyId": "made-up"}, "10.0.0.2:1", attacker); w.Code != http.StatusTooManyRequests {
		t.Fatalf("the client that made the bad tries: %d", w.Code)
	}
}

func TestAFloodOfPasskeyStartsFromOneClientKeepsAnotherClientsCeremony(t *testing.T) {
	h := &Handler{}
	mine, err := h.beginPasskeyCeremony(&webauthnSessionForTest, passkeyHost, "203.0.113.8")
	if err != nil {
		t.Fatal(err)
	}
	for range passkeyCeremonyMax * 3 {
		if _, err := h.beginPasskeyCeremony(&webauthnSessionForTest, passkeyHost, "203.0.113.7"); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := h.takePasskeyCeremony(mine); !ok {
		t.Fatal("another client's starts pushed out a ceremony somebody was finishing")
	}
}

func TestAFullCeremonyMapDropsFromTheClientHoldingTheMost(t *testing.T) {
	h := &Handler{}
	mine, err := h.beginPasskeyCeremony(&webauthnSessionForTest, passkeyHost, "198.51.100.1")
	if err != nil {
		t.Fatal(err)
	}
	for i := range passkeyCeremonyMax * 2 {
		client := "203.0.113." + string(rune('a'+i%(passkeyCeremonyMax/passkeyCeremoniesPerClient+1)))
		if _, err := h.beginPasskeyCeremony(&webauthnSessionForTest, passkeyHost, client); err != nil {
			t.Fatal(err)
		}
	}
	h.passkeyMu.Lock()
	n := len(h.passkeyCeremonies)
	h.passkeyMu.Unlock()
	if n > passkeyCeremonyMax {
		t.Fatalf("%d ceremonies held, the ceiling is %d", n, passkeyCeremonyMax)
	}
	if _, ok := h.takePasskeyCeremony(mine); !ok {
		t.Fatal("the client holding one ceremony lost it while others held many")
	}
}
