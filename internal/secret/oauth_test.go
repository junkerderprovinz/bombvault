package secret

import (
	"strings"
	"testing"
)

func TestOAuthSecretsCarryTheirPrefixAndDiffer(t *testing.T) {
	a, err := NewOAuthSecret(OAuthAccessPrefix)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewOAuthSecret(OAuthAccessPrefix)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a, OAuthAccessPrefix) || len(a) != len(OAuthAccessPrefix)+43 || a == b {
		t.Fatalf("two access tokens %q and %q", a, b)
	}
}

func TestOAuthDigestsAreSeparatedByKindAndAppKey(t *testing.T) {
	const value = "bvat_same"
	access := HashOAuthSecret("k1", "access", value)
	if access == HashOAuthSecret("k1", "refresh", value) {
		t.Fatal("an access digest matches the refresh digest of the same value")
	}
	if access == HashOAuthSecret("k2", "access", value) {
		t.Fatal("the digest does not depend on APP_KEY")
	}
	if access != HashOAuthSecret("k1", "access", value) {
		t.Fatal("the digest is not stable")
	}
}

func TestPKCES256MatchesTheRFCExample(t *testing.T) {
	// RFC 7636 appendix B.
	const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	const challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if !PKCEMatches(verifier, challenge) {
		t.Fatal("the RFC 7636 example does not match")
	}
	if PKCEMatches(verifier+"x", challenge) {
		t.Fatal("a different verifier matches")
	}
}

func TestPKCEVerifierShapeFollowsRFC7636(t *testing.T) {
	for v, want := range map[string]bool{
		strings.Repeat("a", 42):               false,
		strings.Repeat("a", 43):               true,
		strings.Repeat("a", 128):              true,
		strings.Repeat("a", 129):              false,
		strings.Repeat("a", 42) + "~":         true,
		strings.Repeat("a", 42) + "+":         false,
		strings.Repeat("a", 42) + " ":         false,
		"dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1g": false,
	} {
		if got := ValidPKCEVerifier(v); got != want {
			t.Errorf("ValidPKCEVerifier(%q) = %v, want %v", v, got, want)
		}
	}
}
