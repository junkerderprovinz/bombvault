package api

import (
	"os"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Reader-side contract for the "!" prefix in the stored flat selection
// (01-RESEARCH.md R4). The readers are unexported, so these tests live in the
// internal package next to empty_backup_guard_internal_test.go (whose
// guardService/existingDir helpers are reused).

// readersService is guardService with the split-root config, so
// configuredBackupPaths resolves auto-detection exactly like production.
func readersService(t *testing.T) (*Service, *store.Repo) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	return &Service{
		cfg: config.Config{
			HostSourceRoot:   "/mnt",
			HostMountRoot:    "/host/user",
			DataRootSegments: []string{"appdata"},
		},
		store: st,
	}, st
}

// TestConfiguredBackupPathsSplitsExclusions pins the split: the
// explicit-vs-auto test stays on the RAW stored list (an exclusions-only list
// is non-empty and therefore an explicit choice), while the RETURN is the
// includes half only — exclusions never become backup sources, and they are
// never transformed into anything else either (L1/L14).
func TestConfiguredBackupPathsSplitsExclusions(t *testing.T) {
	s, st := readersService(t)
	in := model.Inspect{Mounts: []model.Mount{
		{Type: "bind", Source: "/mnt/user/appdata/plex", Destination: "/config"},
	}}
	auto := s.resolveAppdataPaths("plex", in) // /host/user/user/appdata/plex

	// Mixed stored selection → includes only; the excluded branch is dropped,
	// not turned into a positional nor an exclude pattern.
	if err := st.SetBackupPaths("plex", []string{
		auto[0], "!/host/user/user/appdata/plex/transcoding",
	}); err != nil {
		t.Fatal(err)
	}
	if got, want := s.configuredBackupPaths("plex", in), []string{auto[0]}; !reflect.DeepEqual(got, want) {
		t.Fatalf("configuredBackupPaths = %v, want %v (includes only)", got, want)
	}

	// Exclusions-only: the raw list is non-empty, so the selection still counts
	// as explicit — the answer is the empty includes set, NOT the
	// auto-detection fallback (which would silently resurrect the folder the
	// user just deselected).
	if err := st.SetBackupPaths("plex", []string{"!/host/user/user/appdata/plex/transcoding"}); err != nil {
		t.Fatal(err)
	}
	if got := s.configuredBackupPaths("plex", in); len(got) != 0 {
		t.Fatalf("configuredBackupPaths = %v, want empty (explicit-none, not auto-detect)", got)
	}
}

// TestStoredDataIsGoneClassifiesExplicitNone pins the #181 guard extension
// (threat T-01-03): a non-empty stored selection whose includes are all
// exclusions is a deliberate deselect — the data may be right there on disk —
// so it is never "gone" and the container is never refused as "not reachable".
// The legacy prefix-free behavior is pinned unchanged right next to it.
func TestStoredDataIsGoneClassifiesExplicitNone(t *testing.T) {
	s, st := guardService(t)

	// Exclusions-only: never gone, whatever the disk says.
	if err := st.SetBackupPaths("myapp", []string{"!/host/user/appdata/gone-branch"}); err != nil {
		t.Fatal(err)
	}
	if s.storedDataIsGone("myapp") {
		t.Fatal("an exclusions-only selection is explicit-none, not data-gone")
	}

	// A mixed list whose includes still exist is equally not gone: the
	// "!"-prefixed entries never stat, and they must not invert the
	// measurement of the includes that do.
	there := existingDir(t, "appdata")
	if err := st.SetBackupPaths("myapp", []string{there, "!" + there + "/deselected"}); err != nil {
		t.Fatal(err)
	}
	if s.storedDataIsGone("myapp") {
		t.Fatal("a mixed selection with existing includes is not data-gone")
	}

	// The explicit-none classification is checked BEFORE any stat and is a
	// decision about the selection's shape, not an accident of measuring
	// nothing: even when the captured fallback data (AppdataPaths) has also
	// vanished, an exclusions-only selection stays "not gone" — the user said
	// no, and the disk has nothing to prove about a deliberate deselect.
	captured := existingDir(t, "captured")
	seedCaptured(t, st, "myapp", captured)
	if err := os.RemoveAll(captured); err != nil {
		t.Fatal(err)
	}
	if err := st.SetBackupPaths("myapp", []string{"!/host/user/appdata/gone-branch"}); err != nil {
		t.Fatal(err)
	}
	if s.storedDataIsGone("myapp") {
		t.Fatal("an exclusions-only selection stays explicit-none even when the captured data also vanished")
	}

	// The converse classification, now on the includes-only view: a mixed
	// selection whose include vanished IS gone. The stat loop measures
	// includesOnly(...) — "!"-entries are classes, not paths, and as literals
	// they only ever voted "gone" — so dropping them from the measurement must
	// not mask a genuinely vanished include.
	vanished := existingDir(t, "share")
	if err := st.SetBackupPaths("myapp", []string{vanished, "!" + vanished + "/deselected"}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(vanished); err != nil {
		t.Fatal(err)
	}
	if !s.storedDataIsGone("myapp") {
		t.Fatal("a mixed selection whose include vanished must still be reported gone")
	}

	// Legacy prefix-free behavior unchanged: a prefix-free selection whose
	// folder vanished IS gone (what an unmounted share looks like).
	gone := existingDir(t, "share2")
	if err := st.SetBackupPaths("myapp", []string{gone}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}
	if !s.storedDataIsGone("myapp") {
		t.Fatal("a vanished prefix-free selection must still be reported gone")
	}
}
