package api

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Call sites use these helpers instead of comparing against the literal
// "offsite", so "offsite:<id>" works everywhere.
func TestOffsiteSourceParsing(t *testing.T) {
	cases := []struct {
		source    string
		isOffsite bool
		id        string
	}{
		{"", false, ""},
		{"local", false, ""},
		{"offsite", true, ""},
		{"offsite:abc123", true, "abc123"},
		{"offsite:", true, ""},
		{"offsiteX", false, ""}, // no colon: not the prefixed form
		{"offsite:deadbeef", true, "deadbeef"},
	}
	for _, c := range cases {
		if got := isOffsiteSource(c.source); got != c.isOffsite {
			t.Errorf("isOffsiteSource(%q) = %v, want %v", c.source, got, c.isOffsite)
		}
		if got := offsiteTargetIDFromSource(c.source); got != c.id {
			t.Errorf("offsiteTargetIDFromSource(%q) = %q, want %q", c.source, got, c.id)
		}
	}

	// validOffsiteTargetID accepts a lowercase hex token like store.newID makes
	// and rejects empty, over-long and non-hex input.
	idOK := []string{"a", "deadbeef", "0123456789abcdef0123456789abcdef"}
	idBad := []string{"", "ABC123", "xyz", "dead-beef", "g", string(make([]byte, 65))}
	for _, id := range idOK {
		if !validOffsiteTargetID(id) {
			t.Errorf("validOffsiteTargetID(%q) = false, want true", id)
		}
	}
	for _, id := range idBad {
		if validOffsiteTargetID(id) {
			t.Errorf("validOffsiteTargetID(%q) = true, want false", id)
		}
	}
}

// TestNormalizeSource pins the ?source= mapping: bare "offsite" stays bare, a
// well-formed "offsite:<id>" is kept, a malformed id becomes "offsite:" with no
// id so the resolver refuses it, and everything else is local.
func TestNormalizeSource(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"", "local"},
		{"local", "local"},
		{"whatever", "local"},
		{"offsite", "offsite"},
		{"offsite:0123456789abcdef0123456789abcdef", "offsite:0123456789abcdef0123456789abcdef"},
		{"offsite:", "offsite:"},
		{"offsite:BAD!!", "offsite:"},
		{"offsite:../etc", "offsite:"},
		{"offsite:deadbeef", "offsite:deadbeef"},
	}
	for _, c := range cases {
		if got := normalizeSource(c.raw); got != c.want {
			t.Errorf("normalizeSource(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

// newSourceSeamStore returns a migrated in-memory store.
func newSourceSeamStore(t *testing.T) *store.Repo {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store.New(db)
}

// TestOffsiteTargetForSource pins the resolver: bare "offsite" is the first
// enabled target or the settings fallback, "offsite:<id>" is that row of the
// domain whether it is switched on or not, and anything else is refused.
func TestOffsiteTargetForSource(t *testing.T) {
	st := newSourceSeamStore(t)
	s := &Service{store: st}

	settingsOnly := store.Settings{ContainersOffsite: "s3:legacy", ContainersOffsiteImmutable: true}
	if got, err := s.offsiteTargetForSource(settingsOnly, "containers", "offsite"); err != nil || got.Repo != "s3:legacy" || !got.Immutable {
		t.Fatalf("settings fallback = %+v, %v; want s3:legacy, append-only", got, err)
	}
	if _, err := s.offsiteTargetForSource(settingsOnly, "containers", "local"); !errors.Is(err, errNoOffsiteRepo) {
		t.Fatalf("local source: err = %v, want errNoOffsiteRepo", err)
	}
	if _, err := s.offsiteTargetForSource(store.Settings{}, "vms", "offsite"); !errors.Is(err, errNoOffsiteRepo) {
		t.Fatalf("nothing configured: err = %v, want errNoOffsiteRepo", err)
	}

	off, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: "s3:b2"})
	if err != nil {
		t.Fatal(err)
	}
	on, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Hetzner", Repo: "sftp:hetzner", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	vms, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "VMs", Repo: "s3:vms", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	if got, err := s.offsiteTargetForSource(store.Settings{}, "containers", "offsite"); err != nil || got.ID != on.ID {
		t.Fatalf("bare offsite = %q, %v; want the first enabled target %q", got.ID, err, on.ID)
	}
	if got, err := s.offsiteTargetForSource(store.Settings{}, "containers", "offsite:"+off.ID); err != nil || got.ID != off.ID {
		t.Fatalf("offsite:<switched off> = %q, %v; want %q", got.ID, err, off.ID)
	}
	for _, source := range []string{"offsite:ffffffffffffffffffffffffffffffff", "offsite:" + vms.ID, "offsite:"} {
		if got, err := s.offsiteTargetForSource(store.Settings{}, "containers", source); !errors.Is(err, errUnknownOffsiteTarget) {
			t.Errorf("%s = %q, %v; want errUnknownOffsiteTarget", source, got.ID, err)
		}
	}
}

// On a backfilled single-target install, bare "offsite" resolves to the same
// repo as the legacy settings column, and "offsite:<id>" to that target's repo.
func TestRepoForOffsiteMatchesLegacyColumn(t *testing.T) {
	st := newSourceSeamStore(t)
	s := &Service{store: st}

	// The backfill leaves one enabled target whose Repo equals the legacy column.
	primary, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Primary", Repo: "s3:offsite-primary", Enabled: true, SortOrder: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	settings := store.Settings{ContainersOffsite: "s3:offsite-primary"}

	// A remote repo passes through resolveRepo unchanged.
	bare, err := s.repoFor(settings, "containers", "offsite")
	if err != nil {
		t.Fatalf("repoFor(offsite): %v", err)
	}
	if bare != "s3:offsite-primary" {
		t.Fatalf("repoFor(offsite) = %q, want s3:offsite-primary", bare)
	}

	// An install without target rows resolves to the same repo.
	stLegacy := newSourceSeamStore(t)
	sLegacy := &Service{store: stLegacy}
	legacy, err := sLegacy.repoFor(settings, "containers", "offsite")
	if err != nil {
		t.Fatalf("repoFor(offsite, legacy no rows): %v", err)
	}
	if legacy != bare {
		t.Fatalf("legacy repoFor(offsite) = %q, backfilled = %q; must be identical", legacy, bare)
	}

	byID, err := s.repoFor(settings, "containers", "offsite:"+primary.ID)
	if err != nil {
		t.Fatalf("repoFor(offsite:<primary>): %v", err)
	}
	if byID != "s3:offsite-primary" {
		t.Fatalf("repoFor(offsite:<primary>) = %q, want s3:offsite-primary", byID)
	}

	// A second target's id resolves to its own repo, while bare "offsite" stays
	// on the primary.
	second, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Second", Repo: "s3:offsite-second", Enabled: true, SortOrder: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	secondRepo, err := s.repoFor(settings, "containers", "offsite:"+second.ID)
	if err != nil {
		t.Fatalf("repoFor(offsite:<second>): %v", err)
	}
	if secondRepo != "s3:offsite-second" {
		t.Fatalf("repoFor(offsite:<second>) = %q, want s3:offsite-second", secondRepo)
	}
	stillPrimary, err := s.repoFor(settings, "containers", "offsite")
	if err != nil {
		t.Fatalf("repoFor(offsite) after 2nd target: %v", err)
	}
	if stillPrimary != "s3:offsite-primary" {
		t.Fatalf("repoFor(offsite) after adding a 2nd target = %q, want the primary s3:offsite-primary", stillPrimary)
	}

	if _, err := s.repoFor(store.Settings{}, "vms", "offsite"); err == nil {
		t.Fatal("repoFor(offsite) with nothing configured should error")
	}
	// A local source takes the local path, even when that path is a remote.
	if got, err := s.repoFor(store.Settings{VMsPath: "s3:vms-local"}, "vms", "local"); err != nil || got != "s3:vms-local" {
		t.Fatalf("repoFor(local) = %q, err=%v; want s3:vms-local", got, err)
	}
}

// TestOffsiteSourceImmutableGate pins the per-target delete and prune refusal:
// each source reads its own target's flag, and a source that names no target is
// an error rather than a mutable target.
func TestOffsiteSourceImmutableGate(t *testing.T) {
	st := newSourceSeamStore(t)
	s := &Service{store: st}
	primary, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Primary", Repo: "s3:p", Enabled: true, Immutable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Second", Repo: "s3:s", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		source string
		want   bool
	}{
		{"offsite", true},
		{"offsite:" + primary.ID, true},
		{"offsite:" + second.ID, false},
	} {
		if got, err := s.offsiteSourceImmutable(store.Settings{}, "containers", c.source); err != nil || got != c.want {
			t.Errorf("offsiteSourceImmutable(%s) = %v, %v; want %v", c.source, got, err, c.want)
		}
	}
	if _, err := s.offsiteSourceImmutable(store.Settings{}, "containers", "offsite:ffffffffffffffffffffffffffffffff"); !errors.Is(err, errUnknownOffsiteTarget) {
		t.Fatalf("unknown id: err = %v, want errUnknownOffsiteTarget", err)
	}

	legacy := &Service{store: newSourceSeamStore(t)}
	for _, flag := range []bool{true, false} {
		settings := store.Settings{ContainersOffsite: "s3:x", ContainersOffsiteImmutable: flag}
		if got, err := legacy.offsiteSourceImmutable(settings, "containers", "offsite"); err != nil || got != flag {
			t.Errorf("settings fallback with flag %v = %v, %v", flag, got, err)
		}
	}
}

// TestASwitchedOffTargetIsReachedByItsIDAndNeverByAnother: Hetzner is off and
// append-only, B2 is on. Every path that names Hetzner reaches Hetzner or
// refuses, and none of them lands on B2.
func TestASwitchedOffTargetIsReachedByItsIDAndNeverByAnother(t *testing.T) {
	st := newSourceSeamStore(t)
	s := &Service{store: st}
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "B2", Repo: "s3:b2", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	hetzner, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "Hetzner", Repo: "sftp:u1@hetzner:/vms", Immutable: true})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	source := "offsite:" + hetzner.ID

	if repo, err := s.vmRepoForName(settings, "win11", source); err != nil || repo != "sftp:u1@hetzner:/vms" {
		t.Fatalf("vmRepoForName = %q, %v; want Hetzner's location", repo, err)
	}
	if err := s.DeleteBackupsVM(ctx, "win11", source); !errors.Is(err, errAppendOnlyOffsiteTarget) {
		t.Fatalf("DeleteBackupsVM = %v, want errAppendOnlyOffsiteTarget", err)
	}
	if err := s.PruneDomain(ctx, "vms", source); !errors.Is(err, errAppendOnlyOffsiteTarget) {
		t.Fatalf("PruneDomain = %v, want errAppendOnlyOffsiteTarget", err)
	}

	unknown := "offsite:ffffffffffffffffffffffffffffffff"
	if _, err := s.vmRepoForName(settings, "win11", unknown); !errors.Is(err, errUnknownOffsiteTarget) {
		t.Fatalf("vmRepoForName(unknown) err = %v, want errUnknownOffsiteTarget", err)
	}
	if err := s.DeleteBackupsVM(ctx, "win11", unknown); !errors.Is(err, errUnknownOffsiteTarget) {
		t.Fatalf("DeleteBackupsVM(unknown) = %v, want errUnknownOffsiteTarget", err)
	}
	if _, err := s.runDRDrill(ctx, "vms", unknown, false); !errors.Is(err, errUnknownOffsiteTarget) {
		t.Fatalf("runDRDrill(unknown) = %v, want errUnknownOffsiteTarget", err)
	}
}
