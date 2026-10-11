package keydeposit

import (
	"bytes"
	"testing"
)

func TestAProofHoldsForItsClaimAndItsKeyOnly(t *testing.T) {
	seed, err := NewOwnerSeed()
	if err != nil {
		t.Fatal(err)
	}
	claim := Claim{Action: ProofKeep, OwnerID: "tower", KeeperID: "attic", Counter: 7, Deposit: []byte("sealed"), Salt: []byte("salt")}
	proof := Prove(seed, claim)
	if !Proven(OwnerKey(seed), proof, claim) {
		t.Fatal("a proof does not hold for the claim it was made for")
	}

	for name, change := range map[string]func(*Claim){
		"another action":  func(c *Claim) { c.Action = ProofDrop },
		"another install": func(c *Claim) { c.OwnerID = "barn" },
		"another keeper":  func(c *Claim) { c.KeeperID = "barn" },
		"another counter": func(c *Claim) { c.Counter = 8 },
		"another deposit": func(c *Claim) { c.Deposit = []byte("planted") },
		"another salt":    func(c *Claim) { c.Salt = []byte("pepper") },
		"a moved boundary": func(c *Claim) {
			c.OwnerID, c.KeeperID = "towera", "ttic"
		},
	} {
		other := claim
		change(&other)
		if Proven(OwnerKey(seed), proof, other) {
			t.Errorf("the proof holds for %s", name)
		}
	}

	stranger, err := NewOwnerSeed()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(OwnerKey(stranger), OwnerKey(seed)) || Proven(OwnerKey(stranger), proof, claim) {
		t.Fatal("the proof holds under another key")
	}
	if Proven(nil, proof, claim) {
		t.Fatal("the proof holds under no key")
	}
}
