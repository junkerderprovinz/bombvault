package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestOffsiteSourceParsing pins the pure source-string helpers: the predicate,
// the id extractor and the plausible-id guard. These are the single seam every
// call site uses instead of comparing the literal "offsite", so an "offsite:<id>"
// flows through unchanged.
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

	// validOffsiteTargetID accepts a store.newID-shaped lowercase-hex token and
	// rejects empty / over-long / non-hex input.
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

// TestNormalizeSource pins the ?source= query mapping: bare "offsite" stays
// bare; a well-formed "offsite:<id>" is kept verbatim (dormant); a malformed
// off-site id collapses to bare "offsite" (safe primary) rather than carrying
// garbage; everything else is local.
func TestNormalizeSource(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"", "local"},
		{"local", "local"},
		{"whatever", "local"},
		{"offsite", "offsite"},
		{"offsite:0123456789abcdef0123456789abcdef", "offsite:0123456789abcdef0123456789abcdef"},
		{"offsite:", "offsite"},       // empty id → bare primary
		{"offsite:BAD!!", "offsite"},  // malformed id → bare primary, token dropped
		{"offsite:../etc", "offsite"}, // injection-ish → dropped
		{"offsite:deadbeef", "offsite:deadbeef"},
	}
	for _, c := range cases {
		if got := normalizeSource(c.raw); got != c.want {
			t.Errorf("normalizeSource(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

// newSourceSeamStore spins up a migrated in-memory store for the resolver tests.
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

// TestOffsiteTargetForSource pins the target resolver: bare "offsite" → primary,
// "offsite:<id>" → that target, an unknown-but-well-formed id → primary (the safe
// fallback), and — with no target rows — a settings-synthesized target so an
// un-backfilled install still resolves.
func TestOffsiteTargetForSource(t *testing.T) {
	st := newSourceSeamStore(t)
	s := &Service{store: st}

	// No rows, but a legacy Settings off-site column set: the resolver synthesizes
	// the one target the backfill would have produced.
	settingsOnly := store.Settings{ContainersOffsite: "s3:legacy", ContainersOffsiteImmutable: true}
	got, ok := s.offsiteTargetForSource(settingsOnly, "containers", "offsite")
	if !ok {
		t.Fatal("offsiteTargetForSource(no rows, settings set) should resolve via the settings fallback")
	}
	if got.Repo != "s3:legacy" || !got.Immutable {
		t.Fatalf("settings-fallback target = %+v, want repo s3:legacy immutable=true", got)
	}
	// Non-offsite source never resolves an off-site target.
	if _, ok := s.offsiteTargetForSource(settingsOnly, "containers", "local"); ok {
		t.Fatal("offsiteTargetForSource(local) should be false")
	}
	// No rows AND no settings column: nothing configured.
	if _, ok := s.offsiteTargetForSource(store.Settings{}, "vms", "offsite"); ok {
		t.Fatal("offsiteTargetForSource with nothing configured should be false")
	}

	// Now with real rows: a primary and a second target.
	primary, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Primary", Repo: "s3:primary", Enabled: true, SortOrder: 0, Immutable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Second", Repo: "s3:second", Enabled: true, SortOrder: 1, Immutable: false,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Bare "offsite" → primary (first enabled), ignoring the legacy settings column.
	if got, ok := s.offsiteTargetForSource(store.Settings{}, "containers", "offsite"); !ok || got.ID != primary.ID {
		t.Fatalf("bare offsite = %+v (ok=%v), want primary id %s", got, ok, primary.ID)
	}
	// "offsite:<primaryID>" → primary.
	if got, ok := s.offsiteTargetForSource(store.Settings{}, "containers", "offsite:"+primary.ID); !ok || got.Repo != "s3:primary" {
		t.Fatalf("offsite:<primary> = %+v (ok=%v), want repo s3:primary", got, ok)
	}
	// "offsite:<secondID>" → the second target.
	if got, ok := s.offsiteTargetForSource(store.Settings{}, "containers", "offsite:"+second.ID); !ok || got.Repo != "s3:second" {
		t.Fatalf("offsite:<second> = %+v (ok=%v), want repo s3:second", got, ok)
	}
	// Unknown but well-formed id → falls back to primary (never strands a restore).
	if got, ok := s.offsiteTargetForSource(store.Settings{}, "containers", "offsite:ffffffffffffffffffffffffffffffff"); !ok || got.ID != primary.ID {
		t.Fatalf("offsite:<unknown> = %+v (ok=%v), want primary id %s", got, ok, primary.ID)
	}
}

// TestRepoForOffsiteByteIdentical is the backward-compat guard: bare "offsite"
// resolves to the SAME repo as a backfilled single-target install's legacy
// column, and per-id addressing picks the matching target's repo.
func TestRepoForOffsiteByteIdentical(t *testing.T) {
	st := newSourceSeamStore(t)
	s := &Service{store: st}

	// A backfilled N=1 install: one enabled target whose Repo equals the legacy
	// Settings column (this is exactly what the stage-1 backfill produces).
	primary, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Primary", Repo: "s3:offsite-primary", Enabled: true, SortOrder: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	settings := store.Settings{ContainersOffsite: "s3:offsite-primary"}

	// Bare "offsite" resolves to the primary repo (a remote repo passes through
	// resolveRepo verbatim, so this is the literal repo string).
	bare, err := s.repoFor(settings, "containers", "offsite")
	if err != nil {
		t.Fatalf("repoFor(offsite): %v", err)
	}
	if bare != "s3:offsite-primary" {
		t.Fatalf("repoFor(offsite) = %q, want s3:offsite-primary", bare)
	}

	// And it must equal what a rows-less legacy install resolves to — the property
	// that keeps existing installs byte-identical.
	stLegacy := newSourceSeamStore(t)
	sLegacy := &Service{store: stLegacy}
	legacy, err := sLegacy.repoFor(settings, "containers", "offsite")
	if err != nil {
		t.Fatalf("repoFor(offsite, legacy no rows): %v", err)
	}
	if legacy != bare {
		t.Fatalf("legacy repoFor(offsite) = %q, backfilled = %q; must be identical", legacy, bare)
	}

	// "offsite:<primaryID>" resolves to the same repo.
	byID, err := s.repoFor(settings, "containers", "offsite:"+primary.ID)
	if err != nil {
		t.Fatalf("repoFor(offsite:<primary>): %v", err)
	}
	if byID != "s3:offsite-primary" {
		t.Fatalf("repoFor(offsite:<primary>) = %q, want s3:offsite-primary", byID)
	}

	// A second target: its id resolves to its own repo, while bare "offsite" still
	// resolves to the primary (first enabled).
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

	// Nothing configured → the caller's "no such repo" error (unchanged contract).
	if _, err := s.repoFor(store.Settings{}, "vms", "offsite"); err == nil {
		t.Fatal("repoFor(offsite) with nothing configured should error")
	}
	// A non-offsite source selects the local branch (no off-site parsing): a
	// remote local path passes straight through resolveRepo unchanged.
	if got, err := s.repoFor(store.Settings{VMsPath: "s3:vms-local"}, "vms", "local"); err != nil || got != "s3:vms-local" {
		t.Fatalf("repoFor(local) = %q, err=%v; want s3:vms-local", got, err)
	}
}

// TestOffsiteSourceImmutableGate pins the per-target delete/prune refusal: a
// delete against an immutable target's source is refused, a non-immutable target
// is allowed, and bare "offsite" uses the primary target's flag.
func TestOffsiteSourceImmutableGate(t *testing.T) {
	st := newSourceSeamStore(t)
	s := &Service{store: st}

	// Primary is immutable, the second target is not.
	primary, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Primary", Repo: "s3:p", Enabled: true, SortOrder: 0, Immutable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Second", Repo: "s3:s", Enabled: true, SortOrder: 1, Immutable: false,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Bare "offsite" → primary's flag (immutable → refuse).
	if !s.offsiteSourceImmutable(store.Settings{}, "containers", "offsite") {
		t.Fatal("bare offsite should be immutable (primary target is immutable)")
	}
	// "offsite:<primaryID>" → immutable.
	if !s.offsiteSourceImmutable(store.Settings{}, "containers", "offsite:"+primary.ID) {
		t.Fatal("offsite:<primary> should be immutable")
	}
	// "offsite:<secondID>" → NOT immutable (delete allowed against this target).
	if s.offsiteSourceImmutable(store.Settings{}, "containers", "offsite:"+second.ID) {
		t.Fatal("offsite:<second> should NOT be immutable")
	}

	// No rows: falls back to the legacy per-domain Settings flag so the refusal
	// stays byte-identical for an un-backfilled install.
	legacy := &Service{store: newSourceSeamStore(t)}
	if !legacy.offsiteSourceImmutable(store.Settings{ContainersOffsite: "s3:x", ContainersOffsiteImmutable: true}, "containers", "offsite") {
		t.Fatal("settings-fallback offsite should honor ContainersOffsiteImmutable=true")
	}
	if legacy.offsiteSourceImmutable(store.Settings{ContainersOffsite: "s3:x", ContainersOffsiteImmutable: false}, "containers", "offsite") {
		t.Fatal("settings-fallback offsite should be mutable when ContainersOffsiteImmutable=false")
	}
}
