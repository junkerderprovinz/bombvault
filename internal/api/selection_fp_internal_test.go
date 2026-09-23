package api

import (
	"os"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestSelectionFingerprintFollowsTheConfiguredSelection(t *testing.T) {
	base := itemSelection{
		Kind:     "container",
		Paths:    []string{"/host/appdata/nextcloud", "/host/appdata/nextcloud-db"},
		Excludes: []string{"*.tmp", "cache"},
		Caches:   []string{"/host/appdata/nextcloud"},
	}

	t.Run("order and duplicates do not count", func(t *testing.T) {
		shuffled := itemSelection{
			Kind:     "container",
			Paths:    []string{"/host/appdata/nextcloud-db", "/host/appdata/nextcloud", "/host/appdata/nextcloud"},
			Excludes: []string{"cache", "*.tmp"},
			Caches:   []string{"/host/appdata/nextcloud"},
		}
		if got, want := selectionFingerprint(shuffled), selectionFingerprint(base); got != want {
			t.Fatalf("the same selection in another order fingerprinted %q, want %q", got, want)
		}
	})

	t.Run("an added exclude is a different selection", func(t *testing.T) {
		more := base
		more.Excludes = append(append([]string{}, base.Excludes...), "logs")
		if selectionFingerprint(more) == selectionFingerprint(base) {
			t.Fatal("adding an exclude left the fingerprint unchanged, so an excluded folder would read as data that vanished")
		}
	})

	t.Run("a dropped folder is a different selection", func(t *testing.T) {
		fewer := base
		fewer.Paths = []string{"/host/appdata/nextcloud"}
		if selectionFingerprint(fewer) == selectionFingerprint(base) {
			t.Fatal("dropping a folder left the fingerprint unchanged")
		}
	})

	t.Run("a cache marker is part of the selection", func(t *testing.T) {
		cached := base
		cached.Caches = nil
		if selectionFingerprint(cached) == selectionFingerprint(base) {
			t.Fatal("turning the cache exclusion off left the fingerprint unchanged")
		}
	})

	t.Run("two kinds with the same lists stay apart", func(t *testing.T) {
		asSet := base
		asSet.Kind = "files"
		if selectionFingerprint(asSet) == selectionFingerprint(base) {
			t.Fatal("a folder set and a container with the same paths share a fingerprint")
		}
	})

	t.Run("the fingerprint is sixteen hex characters", func(t *testing.T) {
		fp := selectionFingerprint(base)
		if len(fp) != 16 {
			t.Fatalf("fingerprint %q is %d characters, want 16", fp, len(fp))
		}
		for _, r := range fp {
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
				t.Fatalf("fingerprint %q is not hex", fp)
			}
		}
	})

	t.Run("every item kind fingerprints its own fields", func(t *testing.T) {
		seen := map[string]string{}
		for _, sel := range []itemSelection{
			{Kind: "vm", Paths: []string{"/host/domains/win11/vdisk1.img"}, Excludes: []string{"vdb"}},
			{Kind: "files", Root: "/host/media", Paths: []string{"/host/media/photos"}, Excludes: []string{"*.part"}},
			{Kind: "flash"},
			{Kind: "config"},
			{Kind: "dbdump", Engine: "postgres", Scope: "one-database"},
		} {
			fp := selectionFingerprint(sel)
			if other, clash := seen[fp]; clash {
				t.Fatalf("%s and %s fingerprint the same", sel.Kind, other)
			}
			seen[fp] = sel.Kind
		}
		if selectionFingerprint(itemSelection{Kind: "dbdump", Engine: "postgres", Scope: "one-database"}) ==
			selectionFingerprint(itemSelection{Kind: "dbdump", Engine: "postgres"}) {
			t.Fatal("a dump narrowed to one database fingerprints like a dump of all of them")
		}
	})
}

// A selection is what the item is configured to cover, which is not the same as
// what is on disk right now. An unmounted share must keep the fingerprint it
// had, so the collapse it causes is reported and holds retention instead of
// being written off as a change the user made.
func TestConfiguredPathsSurviveAVanishedFolder(t *testing.T) {
	s, st := guardService(t)
	dir := existingDir(t, "appdata")
	if _, err := st.UpsertTarget(store.Target{ContainerName: "nextcloud", SelectedPaths: []string{dir}}); err != nil {
		t.Fatal(err)
	}

	effective, configured, _ := s.effectiveBackupPathsWithSelection("nextcloud", model.Inspect{})
	if len(effective) != 1 || len(configured) != 1 {
		t.Fatalf("with the folder present: effective %v, configured %v", effective, configured)
	}
	before := selectionFingerprint(itemSelection{Kind: "container", Paths: configured})

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	effective, configured, _ = s.effectiveBackupPathsWithSelection("nextcloud", model.Inspect{})
	if len(effective) != 0 {
		t.Fatalf("the backup would still read %v after the folder disappeared", effective)
	}
	if got := selectionFingerprint(itemSelection{Kind: "container", Paths: configured}); got != before {
		t.Fatalf("the fingerprint changed to %q when the folder disappeared, want %q - the collapse would read as a deliberate change", got, before)
	}
}
