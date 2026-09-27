package api

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// RepoID names the repositories listed in ids and no other.
func (e *envEngine) RepoID(_ context.Context, repo string, _ restic.Mode) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if id, ok := e.ids[filepath.ToSlash(repo)]; ok {
		return id, nil
	}
	return "", errors.New("repository does not exist: unable to open config file")
}

func TestLocalFoldersAreProbedByWhatLiesInThem(t *testing.T) {
	f := newPlacementFixture(t)
	eng := newEnvEngine(f)
	base := f.root + "/user/bombvault"
	if err := os.MkdirAll(filepath.FromSlash(base+"/vms"), 0o750); err != nil {
		t.Fatal(err)
	}
	f.makeRepo(base + "/flash")
	eng.ids[base+"/flash"] = "id-flash"
	if err := os.MkdirAll(filepath.FromSlash(base+"/config"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.FromSlash(base+"/config/notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	res := f.svc.probeFolders(context.Background(), "user/bombvault", places.DefaultFolders(), f.svc.ModeFor(settingsOf(t, f.svc)))
	want := map[string]places.FolderState{
		"containers": places.FolderAbsent, "vms": places.FolderEmpty, "flash": places.FolderRepository,
		"config": places.FolderFailed, "files": places.FolderAbsent, "zfs": places.FolderAbsent,
	}
	if !maps.Equal(res.Folders, want) {
		t.Fatalf("folders = %v\nwant %v", res.Folders, want)
	}
	if !maps.Equal(res.RepoIDs, map[string]string{"flash": "id-flash"}) {
		t.Errorf("repository ids = %v", res.RepoIDs)
	}
	if res.OK || res.Errors["config"].Code != "place-probe-failed" {
		t.Errorf("ok = %v, errors = %v; want the folder with other files refused", res.OK, res.Errors)
	}
}

func TestALocalFolderThatCannotBeOpenedIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	newEnvEngine(f)
	if err := os.MkdirAll(filepath.FromSlash(f.root+"/user/bombvault"), 0o750); err != nil {
		t.Fatal(err)
	}
	// No file system takes a name this long, so opening it fails everywhere the
	// way a folder without read permission fails on the server.
	folders := places.Folders{"files": strings.Repeat("x", 300)}

	res := f.svc.probeFolders(context.Background(), "user/bombvault", folders, f.svc.ModeFor(settingsOf(t, f.svc)))
	if res.OK || res.Folders["files"] != places.FolderFailed || res.Errors["files"].Code != "place-probe-failed" {
		t.Fatalf("files = %s, %+v; want the folder refused", res.Folders["files"], res.Errors["files"])
	}
}

func TestRemoteFoldersAreOpenedWithThePlacesEnvironment(t *testing.T) {
	f := newPlacementFixture(t)
	eng := newEnvEngine(f)
	base := "rest:http://nas:8000/bv"
	eng.ids[base+"/container"] = "id-container"
	f.eng.opens[base+"/vms"] = false
	f.eng.opens[base+"/flash"] = false
	f.eng.openErr[base+"/flash"] = errors.New("unable to open config file: unexpected HTTP response (401): 401 Unauthorized")
	mode := f.svc.ModeFor(settingsOf(t, f.svc))
	mode.Env = []string{"RESTIC_REST_USERNAME=bv", "RESTIC_REST_PASSWORD=pw"}

	res := f.svc.probeFolders(context.Background(), base, places.Folders{"containers": "container", "vms": "vms", "flash": "flash"}, mode)
	want := map[string]places.FolderState{"containers": places.FolderRepository, "vms": places.FolderEmpty, "flash": places.FolderFailed}
	if !maps.Equal(res.Folders, want) {
		t.Fatalf("folders = %v\nwant %v", res.Folders, want)
	}
	if res.Errors["flash"].Code != "direct-access-denied" {
		t.Errorf("flash = %+v, want the refused key named", res.Errors["flash"])
	}
	if !maps.Equal(res.RepoIDs, map[string]string{"containers": "id-container"}) {
		t.Errorf("repository ids = %v", res.RepoIDs)
	}
	if env := eng.env(base + "/vms"); !slices.Equal(env, mode.Env) {
		t.Errorf("vms was opened with %v", env)
	}
}

func TestDomainsSharingThePlaceItselfShareItsRepository(t *testing.T) {
	f := newPlacementFixture(t)
	eng := newEnvEngine(f)
	base := "rest:http://nas:8000/bv/shared"
	eng.ids[base] = "id-shared"
	res := f.svc.probeFolders(context.Background(), base, places.Folders{"containers": "", "vms": ""}, f.svc.ModeFor(settingsOf(t, f.svc)))
	if !maps.Equal(res.RepoIDs, map[string]string{"containers": "id-shared", "vms": "id-shared"}) {
		t.Fatalf("repository ids = %v", res.RepoIDs)
	}
	if _, probed := res.Folders["flash"]; probed {
		t.Errorf("folders = %v, want only the domains the place offers", res.Folders)
	}
	if n := eng.openCount(base); n != 1 {
		t.Errorf("the shared address was opened %d times, want once", n)
	}
}

// oneModeIDs reads a repository's id only under one encryption setting.
type oneModeIDs struct {
	*envEngine
	encrypted bool
}

func (e oneModeIDs) RepoID(ctx context.Context, repo string, m restic.Mode) (string, error) {
	if m.Encrypted != e.encrypted {
		return "", errors.New("wrong password or no key found")
	}
	return e.envEngine.RepoID(ctx, repo, m)
}

func TestARepositoryUnderTheOtherEncryptionSettingStillGivesItsID(t *testing.T) {
	f := newPlacementFixture(t)
	eng := newEnvEngine(f)
	base := "rest:http://nas:8000/bv"
	eng.ids[base+"/container"] = "id-container"
	mode := f.svc.ModeFor(settingsOf(t, f.svc))
	f.svc.engine = oneModeIDs{envEngine: eng, encrypted: !mode.Encrypted}

	res := f.svc.probeFolders(context.Background(), base, places.Folders{"containers": "container"}, mode)
	if res.Folders["containers"] != places.FolderRepository || res.RepoIDs["containers"] != "id-container" {
		t.Fatalf("probe = %+v, want the repository and its id", res)
	}
}

func TestARestFolderUnderAnotherUsersNameSaysWhichNameIsWrong(t *testing.T) {
	f := newPlacementFixture(t)
	newEnvEngine(f)
	base := "rest:http://nas:8000/tower"
	f.eng.opens[base+"/container"] = false
	f.eng.openErr[base+"/container"] = errors.New("unable to open config file: unexpected HTTP response (401): 401 Unauthorized")
	mode := f.svc.ModeFor(settingsOf(t, f.svc))
	mode.Env = []string{"RESTIC_REST_USERNAME=bv", "RESTIC_REST_PASSWORD=pw"}

	res := f.svc.probeFolders(context.Background(), base, places.Folders{"containers": "container"}, mode)
	got := res.Errors["containers"]
	if res.Folders["containers"] != places.FolderFailed || got.Code != "" || !strings.Contains(got.Message, `"bv"`) || !strings.Contains(got.Message, `"tower"`) {
		t.Fatalf("containers = %s, %+v; want the user and the path's first part named", res.Folders["containers"], got)
	}
}
