package secret

import (
	"strings"
	"testing"
)

// The password store changed format in v8.6.0, and the thing that must not break
// is the upgrade path: an instance whose hash was written by the old HMAC code
// has to keep letting its owner in, then quietly move to Argon2id.

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

// A hash carrying WEAKER parameters than today's must be flagged, or a future
// parameter bump would silently never take effect on existing installs.
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
		t.Fatal("two hashes of one password must differ — that is what the salt is for")
	}
	if !VerifyPassword(appKey, "same password", a) || !VerifyPassword(appKey, "same password", b) {
		t.Fatal("both must still verify")
	}
}

// The pepper is the property the HMAC scheme had and Argon2id does not have on
// its own: a copied database is worth nothing without the APP_KEY.
func TestWrongAppKeyFailsArgonHash(t *testing.T) {
	h := mustHash(t, appKey, "hunter2")
	if VerifyPassword(otherKey, "hunter2", h) {
		t.Fatal("the right password under the wrong APP_KEY must not verify")
	}
}

// A corrupted stored value must refuse everything. The failure mode worth
// guarding is a parse that quietly falls back to defaults and then matches a
// freshly computed hash.
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

// An empty stored hash means "authentication is off", and the callers check that
// before ever getting here. If one forgets, an empty password must still not
// walk in through the legacy branch.
func TestEmptyStoredHashRejectsEmptyPassword(t *testing.T) {
	if VerifyPassword(appKey, "", "") {
		t.Fatal("an empty stored hash must not verify an empty password")
	}
}

func TestNeedsRehashOnUnparseableIsFalse(t *testing.T) {
	// It can never verify, so there is nothing to upgrade — and rehashing only
	// ever runs after a successful verify.
	if NeedsRehash(argonPrefix + "nonsense") {
		t.Fatal("an unparseable argon value has nothing to rehash")
	}
}
