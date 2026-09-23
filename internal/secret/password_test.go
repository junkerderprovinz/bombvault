package secret

import (
	"strings"
	"testing"
)

func TestVerifyAcceptsLegacyHMACHash(t *testing.T) {
	legacy := hashPasswordLegacy(appKey, "hunter2")
	if !VerifyPassword(appKey, "hunter2", legacy) {
		t.Fatal("a password stored before v8.6.0 must still verify")
	}
	if VerifyPassword(appKey, "wrong", legacy) {
		t.Fatal("the legacy path must still reject a wrong password")
	}
}

func TestLegacyHashIsFlaggedForRehash(t *testing.T) {
	if !NeedsRehash(hashPasswordLegacy(appKey, "hunter2")) {
		t.Fatal("a legacy hash must be flagged for upgrade")
	}
	fresh := mustHash(t, appKey, "hunter2")
	if NeedsRehash(fresh) {
		t.Fatal("a hash written with the current parameters must not be flagged")
	}
}

// TestWeakerParametersAreFlaggedForRehash checks that a hash with weaker
// parameters is flagged; otherwise raising them would never reach existing
// installs.
func TestWeakerParametersAreFlaggedForRehash(t *testing.T) {
	weak := hashPasswordWithSalt(appKey, "hunter2", []byte("0123456789abcdef"), 8*1024, 1, 1)
	if !VerifyPassword(appKey, "hunter2", weak) {
		t.Fatal("a hash must verify under the parameters recorded in it")
	}
	if !NeedsRehash(weak) {
		t.Fatal("weaker recorded parameters must be flagged for upgrade")
	}
}

func TestSaltMakesTwoHashesOfOnePasswordDiffer(t *testing.T) {
	a := mustHash(t, appKey, "same password")
	b := mustHash(t, appKey, "same password")
	if a == b {
		t.Fatal("two hashes of one password must differ because of the salt")
	}
	if !VerifyPassword(appKey, "same password", a) || !VerifyPassword(appKey, "same password", b) {
		t.Fatal("both must still verify")
	}
}

// TestWrongAppKeyFailsArgonHash checks the pepper, which makes a copied database
// worthless without the APP_KEY.
func TestWrongAppKeyFailsArgonHash(t *testing.T) {
	h := mustHash(t, appKey, "hunter2")
	if VerifyPassword(otherKey, "hunter2", h) {
		t.Fatal("the right password under the wrong APP_KEY must not verify")
	}
}

// TestMalformedStoredHashVerifiesNothing guards against a parse that falls back
// to defaults and then matches a freshly computed hash.
func TestMalformedStoredHashVerifiesNothing(t *testing.T) {
	good := mustHash(t, appKey, "hunter2")
	broken := []string{
		argonPrefix,
		argonPrefix + "19456$2$1$$",
		argonPrefix + "0$2$1$AAAA$AAAA",
		argonPrefix + "19456$2$1$notbase64!$AAAA",
		strings.Replace(good, "argon2id$", "argon2id$x", 1),
		"",
	}
	for _, b := range broken {
		if VerifyPassword(appKey, "hunter2", b) {
			t.Fatalf("malformed stored hash must never verify: %q", b)
		}
		if VerifyPassword(appKey, "", b) {
			t.Fatalf("malformed stored hash must not verify an empty password either: %q", b)
		}
	}
}

// TestEmptyStoredHashRejectsEmptyPassword covers a caller that forgets to check
// for an empty stored hash, which means authentication is off: an empty
// password still must not pass the legacy branch.
func TestEmptyStoredHashRejectsEmptyPassword(t *testing.T) {
	if VerifyPassword(appKey, "", "") {
		t.Fatal("an empty stored hash must not verify an empty password")
	}
}

func TestNeedsRehashOnUnparseableIsFalse(t *testing.T) {
	if NeedsRehash(argonPrefix + "nonsense") {
		t.Fatal("an unparseable argon value has nothing to rehash")
	}
}
