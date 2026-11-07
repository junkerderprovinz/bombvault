package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The empty-backup guard tells apart a container whose folders have gone
// missing (refuse, or an empty backup would look successful and overwrite the
// stored path list) and one the user made stateless by deselecting every
// folder (proceed and capture the definition only). The store cannot separate
// them, because a cleared selection is stored as an empty SelectedPaths just
// like one that was never set, so the guard checks the disk for the paths the
// last backup captured.

func guardService(t *testing.T) (*Service, *store.Repo) {
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
	return &Service{store: st}, st
}

// seedCaptured records a target whose last backup captured the given paths.
func seedCaptured(t *testing.T, st *store.Repo, name string, captured ...string) {
	t.Helper()
	if _, err := st.UpsertTarget(store.Target{ContainerName: name, AppdataPaths: captured}); err != nil {
		t.Fatal(err)
	}
}

// existingDir creates a directory, so onlyExistingPaths reports it as present.
func existingDir(t *testing.T, name string) string {
	t.Helper()
	p := filepath.ToSlash(filepath.Join(t.TempDir(), name))
	if err := os.MkdirAll(p, 0o750); err != nil {
		t.Fatal(err)
	}
	return p
}

// The captured folder is still on disk and the user deselected it, so the
// backup proceeds with the definition only.
func TestEmptyBackupAllowedAfterDeselectingEveryFolder(t *testing.T) {
	s, st := guardService(t)
	stillThere := existingDir(t, "s3store")
	seedCaptured(t, st, "myapp", stillThere)

	if err := st.SetBackupPaths("myapp", []string{}); err != nil {
		t.Fatal(err)
	}

	if s.emptyBackupRefusal("myapp", model.Inspect{}, nil) != nil {
		t.Fatal("refused a backup after the user deselected every folder, with the folder still on disk")
	}
}

// A captured folder missing from disk is what an unmounted share looks like.
func TestEmptyBackupRefusedWhenCapturedDataVanishes(t *testing.T) {
	s, st := guardService(t)
	gone := existingDir(t, "appdata")
	seedCaptured(t, st, "myapp", gone)
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	if s.emptyBackupRefusal("myapp", model.Inspect{}, nil) == nil {
		t.Fatal("accepted an empty backup after the captured data disappeared from disk")
	}
}

func TestEmptyBackupRefusedWhenSelectedPathsVanish(t *testing.T) {
	s, st := guardService(t)
	gone := existingDir(t, "appdata")
	seedCaptured(t, st, "myapp", gone)
	if err := st.SetBackupPaths("myapp", []string{gone}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	if s.emptyBackupRefusal("myapp", model.Inspect{}, nil) == nil {
		t.Fatal("accepted an empty backup while the user's selected folders were missing")
	}
}

// A container without a stored target is on its first backup.
func TestEmptyBackupAllowedForAFirstRun(t *testing.T) {
	s, _ := guardService(t)

	if s.emptyBackupRefusal("fresh", model.Inspect{}, nil) != nil {
		t.Fatal("refused the first backup of a container that has never been backed up")
	}
}

// A stateless container has a target row from earlier definition-only runs but
// no captured path whose absence could mean anything.
func TestEmptyBackupAllowedWhenNothingWasEverCaptured(t *testing.T) {
	s, st := guardService(t)
	seedCaptured(t, st, "stateless")

	if s.emptyBackupRefusal("stateless", model.Inspect{}, nil) != nil {
		t.Fatal("refused a stateless container that never captured any data")
	}
}

func TestNonEmptyBackupNeverRefused(t *testing.T) {
	s, st := guardService(t)
	gone := existingDir(t, "appdata")
	seedCaptured(t, st, "myapp", gone)
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	if s.emptyBackupRefusal("myapp", model.Inspect{}, []string{"/host/user/appdata/myapp"}) != nil {
		t.Fatal("refused a backup that had paths to back up")
	}
}

// hostDataService maps host /mnt onto a temp folder, as Host Data does.
func hostDataService(t *testing.T) (*Service, *store.Repo, string) {
	t.Helper()
	s, st := guardService(t)
	root := filepath.ToSlash(t.TempDir())
	s.cfg = config.Config{HostMountRoot: root, HostSourceRoot: "/mnt", DataRootSegments: []string{"appdata"}}
	return s, st, root
}

func bindsTo(sources ...string) model.Inspect {
	in := model.Inspect{Name: "/arr-gluetun-1"}
	for _, src := range sources {
		in.Mounts = append(in.Mounts, model.Mount{Type: "bind", Source: src, Destination: "/gluetun"})
	}
	return in
}

// seedBackedUp records a last backup that took captured from a container
// mounted as in.
func seedBackedUp(t *testing.T, st *store.Repo, name string, in model.Inspect, captured ...string) {
	t.Helper()
	def, err := json.Marshal(containerDefinition{Inspect: in, AppdataPaths: captured})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: name, AppdataPaths: captured, Definition: string(def)}); err != nil {
		t.Fatal(err)
	}
}

// A stack deployed again from another manager mounts its data from a path
// outside Host Data. The folder the last backup took is gone because the data
// moved, not because a share is missing, so the backup goes ahead and warns.
func TestEmptyBackupAllowedWhenTheDataMovedOutsideHostData(t *testing.T) {
	s, st, root := hostDataService(t)
	before := bindsTo("/mnt/user/appdata/compose/arr/gluetun")
	seedBackedUp(t, st, "arr-gluetun-1", before, root+"/user/appdata/compose/arr/gluetun")

	now := bindsTo("/app/data/stacks/DXP6800/arr/gluetun")
	if err := s.emptyBackupRefusal("arr-gluetun-1", now, s.effectiveBackupPaths("arr-gluetun-1", now)); err != nil {
		t.Fatalf("refused a container whose data moved outside Host Data: %v", err)
	}
}

// The container still mounts the folder its last backup took, and that folder
// is gone: that is an unmounted share.
func TestEmptyBackupRefusedWhileTheContainerStillMountsTheVanishedFolder(t *testing.T) {
	s, st, root := hostDataService(t)
	in := bindsTo("/mnt/user/appdata/gluetun")
	seedBackedUp(t, st, "arr-gluetun-1", in, root+"/user/appdata/gluetun")

	err := s.emptyBackupRefusal("arr-gluetun-1", in, s.effectiveBackupPaths("arr-gluetun-1", in))
	if err == nil {
		t.Fatal("accepted an empty backup while the mounted folder was missing")
	}
	if !strings.Contains(err.Error(), "/mnt/user/appdata/gluetun") {
		t.Fatalf("the refusal does not name the missing folder: %v", err)
	}
}

// A chosen folder that vanished while the container moved its data to a
// folder Host Data reaches would back up nothing of it, so the backup still
// refuses and asks for the folders to be chosen again.
func TestEmptyBackupRefusedWhenTheChosenFoldersAreStale(t *testing.T) {
	s, st, root := hostDataService(t)
	before := bindsTo("/mnt/user/appdata/compose/arr/gluetun")
	seedBackedUp(t, st, "arr-gluetun-1", before, root+"/user/appdata/compose/arr/gluetun")
	if err := st.SetBackupPaths("arr-gluetun-1", []string{root + "/user/appdata/compose/arr/gluetun"}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root+"/user/appdata/dockhand/stacks/DXP6800/arr/gluetun", 0o750); err != nil {
		t.Fatal(err)
	}

	now := bindsTo("/mnt/user/appdata/dockhand/stacks/DXP6800/arr/gluetun")
	err := s.emptyBackupRefusal("arr-gluetun-1", now, s.effectiveBackupPaths("arr-gluetun-1", now))
	if err == nil || !strings.Contains(err.Error(), "choose its backup folders again") {
		t.Fatalf("want a refusal that asks for the folders again, got %v", err)
	}
}

// The folders are what the user has to act on, so the run history and the
// API keep them.
func TestEmptyBackupRefusalsNameTheirFoldersWhereTheUserReadsThem(t *testing.T) {
	s, st, root := hostDataService(t)
	in := bindsTo("/mnt/user/appdata/gluetun")
	seedBackedUp(t, st, "arr-gluetun-1", in, root+"/user/appdata/gluetun")
	unmounted := s.emptyBackupRefusal("arr-gluetun-1", in, nil)

	before := bindsTo("/mnt/user/appdata/compose/arr/sonarr")
	seedBackedUp(t, st, "arr-sonarr-1", before, root+"/user/appdata/compose/arr/sonarr")
	if err := st.SetBackupPaths("arr-sonarr-1", []string{root + "/user/appdata/compose/arr/sonarr"}); err != nil {
		t.Fatal(err)
	}
	stale := s.emptyBackupRefusal("arr-sonarr-1", bindsTo("/mnt/user/appdata/dockhand/stacks/DXP6800/arr/sonarr"), nil)

	for folder, err := range map[string]error{"/mnt/user/appdata/gluetun": unmounted, "/mnt/user/appdata/compose/arr/sonarr": stale} {
		if err == nil {
			t.Fatalf("no refusal for %s", folder)
		}
		if folder == "/mnt/user/appdata/compose/arr/sonarr" && !strings.Contains(err.Error(), "choose its backup folders again") {
			t.Errorf("want the stale-selection refusal for %s, got %v", folder, err)
		}
		if got := truncateRunErr(err); !strings.Contains(got, folder) {
			t.Errorf("run history = %q, want it to name %s", got, folder)
		}
		if got := scrubError(err); !strings.Contains(got, folder) {
			t.Errorf("API error = %q, want it to name %s", got, folder)
		}
	}
}
