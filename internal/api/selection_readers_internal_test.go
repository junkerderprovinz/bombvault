package api

import (
	"os"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// readersService is guardService with the split-root config, so
// configuredBackupPaths auto-detects as in production.
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

// configuredBackupPaths returns only the includes, but decides between explicit
// and auto-detected on the raw list, so an exclusions-only list counts as an
// explicit choice.
func TestConfiguredBackupPathsSplitsExclusions(t *testing.T) {
	s, st := readersService(t)
	in := model.Inspect{Mounts: []model.Mount{
		{Type: "bind", Source: "/mnt/user/appdata/plex", Destination: "/config"},
	}}
	auto := s.resolveAppdataPaths("plex", in) // /host/user/user/appdata/plex

	if err := st.SetBackupPaths("plex", []string{
		auto[0], "!/host/user/user/appdata/plex/transcoding",
	}); err != nil {
		t.Fatal(err)
	}
	if got, want := s.configuredBackupPaths("plex", in), []string{auto[0]}; !reflect.DeepEqual(got, want) {
		t.Fatalf("configuredBackupPaths = %v, want %v (includes only)", got, want)
	}

	// Falling back to auto-detection would bring back the deselected folder.
	if err := st.SetBackupPaths("plex", []string{"!/host/user/user/appdata/plex/transcoding"}); err != nil {
		t.Fatal(err)
	}
	if got := s.configuredBackupPaths("plex", in); len(got) != 0 {
		t.Fatalf("configuredBackupPaths = %v, want empty (explicit-none, not auto-detect)", got)
	}
}

// An exclusions-only selection is an explicit deselect, so storedDataIsGone
// never reports it gone, whatever is on disk.
func TestStoredDataIsGoneClassifiesExplicitNone(t *testing.T) {
	s, st := guardService(t)

	if err := st.SetBackupPaths("myapp", []string{"!/host/user/appdata/gone-branch"}); err != nil {
		t.Fatal(err)
	}
	if s.storedDataIsGone("myapp") {
		t.Fatal("an exclusions-only selection is explicit-none, not data-gone")
	}

	// Exclusions are not statted and must not affect the includes that are.
	there := existingDir(t, "appdata")
	if err := st.SetBackupPaths("myapp", []string{there, "!" + there + "/deselected"}); err != nil {
		t.Fatal(err)
	}
	if s.storedDataIsGone("myapp") {
		t.Fatal("a mixed selection with existing includes is not data-gone")
	}

	// The shape is checked before any stat, so this holds even when the
	// captured AppdataPaths have vanished as well.
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

	// A mixed selection whose include vanished is gone.
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

	// So is a plain selection whose folder vanished, as with an unmounted share.
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
