package store_test

import (
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestTheDatabaseRefusesALineBreakInWhatTheKitPrints(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	refused := func(what string, err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), store.ErrLineBreakText) {
			t.Errorf("%s with a line break: %v", what, err)
		}
	}

	_, err := r.CreateMeshOffer(store.MeshOffer{From: "attic", Repo: "rest:https://box:8000/files\n## Encryption"})
	refused("a mesh offer", err)
	_, err = r.CreateOffsiteTarget(store.OffsiteTarget{Domain: "files", Name: "box", Repo: "rest:https://box:8000/files\r## Encryption", Enabled: true})
	refused("a repository location", err)
	_, err = r.CreateOffsiteTarget(store.OffsiteTarget{Domain: "files", Name: "box\nmore", Repo: "rest:https://box:8000/files", Enabled: true})
	refused("a repository name", err)
	_, err = r.CreateFileSet(store.FileSet{Name: "photos\n## Encryption", Path: "photos"})
	refused("a folder set name", err)
	_, err = r.MutateSettings(func(s *store.Settings) error { s.FilesOffsite = "rest:https://box:8000/files\nmore"; return nil })
	refused("a section's location", err)

	target, err := r.CreateOffsiteTarget(store.OffsiteTarget{Domain: "files", Name: "box", Repo: "rest:https://box:8000/files", Enabled: true})
	if err != nil {
		t.Fatalf("a location on one line: %v", err)
	}
	target.Repo = "rest:https://box:8000/files\nmore"
	_, err = r.UpsertOffsiteTarget(target)
	refused("a changed location", err)
}

func TestAValueFromBeforeTheGuardDoesNotBlockOtherChanges(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	if _, err := db.Exec(`DROP TRIGGER one_line_settings_update`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE settings SET files_offsite = ?`, "rest:https://box:8000/files\nmore"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE name = 'one_line_names_and_locations'`); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}

	if _, err := r.MutateSettings(func(s *store.Settings) error { s.InstanceName = "tower"; return nil }); err != nil {
		t.Fatalf("another setting while the old value stands: %v", err)
	}
	if _, err := r.MutateSettings(func(s *store.Settings) error { s.FilesOffsite = "rest:https://box:8000/files"; return nil }); err != nil {
		t.Fatalf("correcting the old value: %v", err)
	}
	if _, err := r.MutateSettings(func(s *store.Settings) error { s.FilesOffsite = "a\nb"; return nil }); err == nil {
		t.Fatal("a new line break went through")
	}
}
