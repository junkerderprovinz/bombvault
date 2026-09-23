package api

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLegacyDefsMigrateOnlyIntoDomainFolder: the legacy folder holds the
// definitions of a whole domain, while writeDefToStorage may write one item's
// definition to a named repository. Migrating into that repository would move
// every other item's definition out of Discover's sight and then delete the
// legacy folder.
func TestLegacyDefsMigrateOnlyIntoDomainFolder(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "bombvault-defs")
	domain := filepath.Join(root, "repo", "def")
	named := filepath.Join(root, "cold", "def")
	for _, d := range []string{legacy, domain, named} {
		if err := os.MkdirAll(d, 0o755); err != nil { //nolint:gosec // G301: test temp dir
			t.Fatal(err)
		}
	}
	// One definition for the item being backed up, one for another item.
	for _, n := range []string{"plex.def", "sonarr.def"} {
		if err := os.WriteFile(filepath.Join(legacy, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// A named repository: nothing moves.
	migrateLegacyDefsIfDomain(named, domain, legacy)
	for _, n := range []string{"plex.def", "sonarr.def"} {
		if _, err := os.Stat(filepath.Join(legacy, n)); err != nil {
			t.Fatalf("%s left the legacy folder on a write to a named repository: %v\n"+
				"That folder holds the WHOLE domain, so moving it because one item lives\n"+
				"elsewhere hides every other item's definition from Discover.", n, err)
		}
		if _, err := os.Stat(filepath.Join(named, n)); err == nil {
			t.Errorf("%s was moved into the named repository", n)
		}
	}

	// The domain folder: both move and the legacy folder is removed.
	migrateLegacyDefsIfDomain(domain, domain, legacy)
	for _, n := range []string{"plex.def", "sonarr.def"} {
		if _, err := os.Stat(filepath.Join(domain, n)); err != nil {
			t.Errorf("%s did not reach the domain folder: %v", n, err)
		}
	}
	if _, err := os.Stat(legacy); err == nil {
		t.Error("the emptied legacy folder was left behind")
	}
}
