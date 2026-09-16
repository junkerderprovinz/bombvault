package api_test

// Sealing the recovery kit.
//
// The kit is the master secret: the APP_KEY, the derived restic password and
// the real repository locations. It is also the one document whose whole job is
// to survive the machine, which means it gets stored somewhere else: a password
// manager, a printout, a USB stick in a drawer. Those are exactly the places
// where "plaintext file holding the master key" is the wrong shape.
//
// So when export encryption is on, the kit is sealed to the configured age
// recipients like the other plain exports. Two properties matter more than the
// feature itself, and both are pinned below: the sealed kit must actually be
// openable with the private key, and a misconfiguration must fail loudly rather
// than quietly handing out the plaintext it was asked to protect.

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"filippo.io/age"
	"filippo.io/age/armor"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// enableExportEncryption switches sealing on with the given recipient string.
func enableExportEncryption(t *testing.T, st *store.Repo, recipients string) {
	t.Helper()
	s, err := st.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	s.ExportEncryptEnabled = true
	s.ExportAgeRecipients = recipients
	if err := st.UpdateSettings(s); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
}

// TestRecoveryKitIsSealedAndOpensWithThePrivateKey is the round trip. A sealed
// artifact nobody can open is worse than no sealing at all, so the assertion is
// not "it looks encrypted" but "it decrypts, and the plaintext is the kit".
func TestRecoveryKitIsSealedAndOpensWithThePrivateKey(t *testing.T) {
	h, st, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})

	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}
	enableExportEncryption(t, st, id.Recipient().String())
	cookie := loginCookie(t, h, "correct horse battery staple")

	w := getRaw(t, h, "/api/recovery-kit", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, ".age") {
		t.Fatalf("a sealed kit must not be offered under the plaintext .md name: %s", cd)
	}

	body := w.Body.String()
	if strings.Contains(body, "APP_KEY") && strings.Contains(body, "restic") {
		t.Fatalf("the kit was served in the clear despite encryption being on")
	}

	// ASCII armor, so the kit can still be pasted into a password manager or
	// printed. A binary blob would technically be sealed and practically
	// unusable for the one job this document has.
	if !strings.HasPrefix(body, armor.Header) {
		t.Fatalf("the sealed kit is not ASCII-armored, so it cannot be pasted or printed: %.40q", body)
	}

	ar := armor.NewReader(strings.NewReader(body))
	plain, err := age.Decrypt(ar, id)
	if err != nil {
		t.Fatalf("the sealed kit does not decrypt with its own recipient's key: %v", err)
	}
	out, err := io.ReadAll(plain)
	if err != nil {
		t.Fatalf("read decrypted kit: %v", err)
	}
	if !strings.Contains(string(out), "APP_KEY") {
		t.Fatalf("the decrypted kit is not the recovery kit: %.200q", out)
	}
}

// TestRecoveryKitRefusesRatherThanFallBackToPlaintext is the property the whole
// thing rests on. Encryption on plus no usable recipient is a misconfiguration,
// and the one response that must never follow is the plaintext master key.
func TestRecoveryKitRefusesRatherThanFallBackToPlaintext(t *testing.T) {
	for _, recipients := range []string{"", "not-an-age-recipient"} {
		h, st, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
		enableExportEncryption(t, st, recipients)
		cookie := loginCookie(t, h, "correct horse battery staple")

		w := getRaw(t, h, "/api/recovery-kit", cookie)
		body := w.Body.String()
		if strings.Contains(body, "APP_KEY") {
			t.Fatalf("recipients=%q: the master key was served in the clear after a failed seal", recipients)
		}
		if strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
			t.Fatalf("recipients=%q: a refusal must not stream a file", recipients)
		}
		if !strings.Contains(body, "recipient") {
			t.Fatalf("recipients=%q: the refusal must say what is wrong, got %s", recipients, body)
		}
	}
}

// TestRecoveryKitStaysPlainWhenEncryptionIsOff: the gate is on the SETTING, not
// on the kit. With encryption off the download is byte-for-byte what it always
// was, so nobody's existing habit breaks.
func TestRecoveryKitStaysPlainWhenEncryptionIsOff(t *testing.T) {
	h, _, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	cookie := loginCookie(t, h, "correct horse battery staple")

	w := getRaw(t, h, "/api/recovery-kit", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/markdown") {
		t.Fatalf("content type = %q, want text/markdown", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, ".md") || strings.Contains(cd, ".age") {
		t.Fatalf("filename = %q, want the plain .md name", cd)
	}
	if !strings.Contains(w.Body.String(), "APP_KEY") {
		t.Fatalf("the plain kit is missing its own content")
	}
}
