package store

import "testing"

// Nobody can set the address by hand any more, so one set before has to give
// way to what the browser shows again.
func TestAnAddressSetByHandIsLearnedAgainAfterTheUpgrade(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`UPDATE group_state SET direct_url = 'https://vault.example.org', direct_url_manual = 1`,
		`DELETE FROM schema_migrations WHERE version = 910`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := New(db)
	if err := r.LearnGroupDirectURL("https://192.168.1.20:3443"); err != nil {
		t.Fatal(err)
	}
	if g, err := r.GetGroupState(); err != nil || g.DirectURL != "https://192.168.1.20:3443" {
		t.Fatalf("DirectURL = %q, %v; want the learned address", g.DirectURL, err)
	}
}
