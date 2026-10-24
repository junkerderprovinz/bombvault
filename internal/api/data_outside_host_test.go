package api_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestBackupNoteLeavesOutAnonymousVolumesAndExcludedFolders(t *testing.T) {
	const anonName = "3f9c2a7e1b4d6058a9e7c3b1f2d4e6a8b0c2d4e6f8a0b2c4d6e8f0a2b4c6d8e0"
	const anon = "/var/lib/docker/volumes/" + anonName + "/_data"
	const named = "/var/lib/docker/volumes/beszel_cache/_data"
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{
		AppKey: strings.Repeat("a", 64), DataDir: dir,
		HostMountRoot: root, HostSourceRoot: root, DataRootSegments: []string{"appdata"},
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	appdata := root + "/appdata/beszel-agent"
	if err := os.MkdirAll(appdata, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appdata, "agent.conf"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name: "/beszel-agent", Image: "henrygd/beszel-agent:latest", Running: true,
		Mounts: []model.Mount{
			{Type: "bind", Source: appdata, Destination: "/config"},
			{Type: "volume", Name: anonName, Source: anon, Destination: "/var/lib/beszel-agent"},
			{Type: "volume", Name: "beszel_cache", Source: named, Destination: "/cache"},
		},
	}}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

	note := func(excludes ...string) string {
		t.Helper()
		if err := svc.SetExcludes(context.Background(), "beszel-agent", excludes); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Backup(context.Background(), "beszel-agent"); err != nil {
			t.Fatalf("backup: %v", err)
		}
		tg, err := st.GetTargetByContainer("beszel-agent")
		if err != nil {
			t.Fatal(err)
		}
		run, err := st.LastRunForTarget(tg.ID)
		if err != nil || run == nil || run.Status != "success" {
			t.Fatalf("run = %+v, %v, want a success", run, err)
		}
		return run.Error
	}

	if got, want := note(), store.NoteDataOutsideHost+": "+named; got != want {
		t.Fatalf("note without excludes = %q, want %q", got, want)
	}
	if got := note("/cache"); strings.Contains(got, store.NoteDataOutsideHost) {
		t.Fatalf("note with the named volume excluded = %q, want no word about data outside Host Data", got)
	}
	if got := note(named); strings.Contains(got, store.NoteDataOutsideHost) {
		t.Fatalf("note with the named volume's host path excluded = %q, want no word about data outside Host Data", got)
	}
	if slices.Contains(eng.lastExcludes, named) {
		t.Fatalf("restic excludes = %q, want no host path outside Host Data", eng.lastExcludes)
	}
}

func TestContainerMountsListsTheVolumesBombVaultCannotReach(t *testing.T) {
	const anonName = "3f9c2a7e1b4d6058a9e7c3b1f2d4e6a8b0c2d4e6f8a0b2c4d6e8f0a2b4c6d8e0"
	const anon = "/var/lib/docker/volumes/" + anonName + "/_data"
	const named = "/var/lib/docker/volumes/jellyfin_cache/_data"
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	for _, sub := range []string{"user/appdata/jellyfin", "user/volumes/reached/_data"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Config{
		AppKey: strings.Repeat("a", 64), DataDir: dir,
		HostMountRoot: root, HostSourceRoot: "/mnt", DataRootSegments: []string{"appdata"},
	}
	st := newMemStore(t)
	mustSettings(t, st)
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name: "/jellyfin", Image: "jellyfin/jellyfin:latest",
		Mounts: []model.Mount{
			{Type: "bind", Source: "/mnt/user/appdata/jellyfin", Destination: "/config"},
			{Type: "bind", Source: "/dev/shm", Destination: "/transcode"},
			{Type: "volume", Name: anonName, Source: anon, Destination: "/cache"},
			{Type: "volume", Name: "jellyfin_cache", Source: named, Destination: "/metadata"},
			{Type: "volume", Name: "reached", Source: "/mnt/user/volumes/reached/_data", Destination: "/reached"},
		},
	}}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{})

	mounts, _, _, _, err := svc.ContainerMounts(context.Background(), "jellyfin")
	if err != nil {
		t.Fatal(err)
	}
	want := []api.MountInfo{
		{Source: "/mnt/user/appdata/jellyfin", Dest: "/config", Selected: true, IsAppdata: true, Reachable: true},
		{Source: "/dev/shm", Dest: "/transcode"},
		{Source: anon, Dest: "/cache", Anonymous: true},
		{Source: named, Dest: "/metadata"},
	}
	if !slices.Equal(mounts, want) {
		t.Fatalf("mounts = %+v, want %+v", mounts, want)
	}
}
