package api

import (
	"context"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// restic forget without a tag selects the whole repository, so an identity
// without one gets no retention pass at all.
func TestApplyRetentionForgetsNothingWithoutAnIdentityTag(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	eng := &lockHealEngine{}
	s := &Service{engine: eng, store: st}

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.RetentionKeepLast = 5 // p.Any() must be true to reach forgetWithLockHeal

	s.applyRetention(context.Background(), "/repo", settings, restic.Mode{}, entryIdentity{}, "containers", anomalyScope{})

	if eng.forgetCalls != 0 {
		t.Fatalf("retention ran %d forgets for an identity without a tag, want none", eng.forgetCalls)
	}
}
