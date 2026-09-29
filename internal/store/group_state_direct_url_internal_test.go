package store

import "testing"

func TestSetGroupDirectURLMarksItManual(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := New(db)

	if err := r.SetGroupDirectURL("https://bombvault.example.org", true); err != nil {
		t.Fatal(err)
	}
	g, err := r.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	if g.DirectURL != "https://bombvault.example.org" || !g.DirectURLManual {
		t.Fatalf("group state = %+v, want the address stored and marked manual", g)
	}
}

// LearnGroupDirectURL is what an incoming request's Host header feeds, and
// it must never overwrite an address a person set by hand.
func TestLearnGroupDirectURLDoesNotOverwriteAManualAddress(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := New(db)

	if err := r.SetGroupDirectURL("https://vault.example.org", true); err != nil {
		t.Fatal(err)
	}
	if err := r.LearnGroupDirectURL("https://192.168.1.20:3443"); err != nil {
		t.Fatal(err)
	}
	g, err := r.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	if g.DirectURL != "https://vault.example.org" {
		t.Fatalf("DirectURL = %q, want the manual address kept", g.DirectURL)
	}
}

func TestLearnGroupDirectURLStoresWhatIsLearnedUntilSetByHand(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := New(db)

	if err := r.LearnGroupDirectURL("https://192.168.1.20:3443"); err != nil {
		t.Fatal(err)
	}
	g, err := r.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	if g.DirectURL != "https://192.168.1.20:3443" || g.DirectURLManual {
		t.Fatalf("group state = %+v, want the learned address stored and not manual", g)
	}

	if err := r.LearnGroupDirectURL("https://192.168.1.21:3443"); err != nil {
		t.Fatal(err)
	}
	g, err = r.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	if g.DirectURL != "https://192.168.1.21:3443" {
		t.Fatalf("DirectURL = %q, want a newly learned address to replace the old one", g.DirectURL)
	}
}

// A blank manual address clears the override, so the next learned one takes
// over again.
func TestSetGroupDirectURLCanClearTheOverride(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := New(db)

	if err := r.SetGroupDirectURL("https://vault.example.org", true); err != nil {
		t.Fatal(err)
	}
	if err := r.SetGroupDirectURL("", false); err != nil {
		t.Fatal(err)
	}
	if err := r.LearnGroupDirectURL("https://192.168.1.20:3443"); err != nil {
		t.Fatal(err)
	}
	g, err := r.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	if g.DirectURL != "https://192.168.1.20:3443" || g.DirectURLManual {
		t.Fatalf("group state = %+v, want the override cleared and a fresh address learned", g)
	}
}
