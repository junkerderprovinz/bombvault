package store

import (
	"errors"
	"testing"
)

func TestAnUnreadableSkipIsAnErrorAndNotAnEmptyRule(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := New(db)
	for _, raw := range []string{"not json", "null", `[""]`, `["*","t1"]`, `{"a":1}`} {
		if _, err := db.Exec(`INSERT OR REPLACE INTO offsite_copy_rules (domain, identity, skip)
			VALUES ('containers', 'container:nginx', ?)`, raw); err != nil {
			t.Fatal(err)
		}
		if _, _, err := r.CopyRuleFor("containers", "container:nginx"); !errors.Is(err, ErrBadSkip) {
			t.Errorf("CopyRuleFor with skip %s = %v, want ErrBadSkip", raw, err)
		}
		if _, err := r.CopyRulesForDomain("containers"); !errors.Is(err, ErrBadSkip) {
			t.Errorf("CopyRulesForDomain with skip %s = %v, want ErrBadSkip", raw, err)
		}
	}
}
