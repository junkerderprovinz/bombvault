package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// listOnlyDocker answers List with fixed containers.
type listOnlyDocker struct {
	dockercli.Docker
	infos []dockercli.ContainerInfo
}

func (d *listOnlyDocker) List(context.Context) ([]dockercli.ContainerInfo, error) {
	return d.infos, nil
}

// dockhandStack is a Compose stack that Dockhand deployed from its own data
// folder, the way manilx runs it on Unraid: Dockhand keeps /mnt/user/appdata/
// dockhand at /app/data, and Compose labels the members with that inside path.
// Dockhand hands Compose the file on stdin, so the labels name no file.
// Another app mounts a folder at /app/data as well.
func dockhandStack(t *testing.T, withComposeFile bool) (*Service, model.Inspect, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "user", "appdata", "dockhand", "stacks", "DXP6800", "arr")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "user", "appdata", "notes", "stacks", "DXP6800", "arr"), 0o750); err != nil {
		t.Fatal(err)
	}
	if withComposeFile {
		if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte("services: {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{
		cfg: config.Config{HostSourceRoot: "/mnt", HostMountRoot: filepath.ToSlash(root), DataRootSegments: unraidSegments},
		docker: &listOnlyDocker{infos: []dockercli.ContainerInfo{
			{Name: "notes", Mounts: []dockercli.MountPoint{{Source: "/mnt/user/appdata/notes", Destination: "/app/data"}}},
			{Name: "dockhand", Mounts: []dockercli.MountPoint{
				{Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock"},
				{Source: "/mnt/user/appdata/dockhand", Destination: "/app/data"},
			}},
		}},
	}
	in := model.Inspect{Config: model.Config{Labels: map[string]string{
		"com.docker.compose.project":              "arr",
		"com.docker.compose.project.working_dir":  "/app/data/stacks/DXP6800/arr",
		"com.docker.compose.project.config_files": "-",
	}}}
	return s, in, filepath.ToSlash(dir)
}

func TestStackFolderOfAManagedStackIsFoundThroughTheManagersMount(t *testing.T) {
	s, in, want := dockhandStack(t, true)
	project, dir, ok := s.stackDirFor(context.Background(), in)
	if !ok || project != "arr" || dir != want {
		t.Fatalf("stackDirFor = %q, %q, %v, want arr at %q", project, dir, ok, want)
	}
	if got := s.dataOutsideHostMount(context.Background(), in, nil); len(got) != 0 {
		t.Fatalf("dataOutsideHostMount = %v, want nothing: the project folder is reachable at %s", got, want)
	}
}

func TestStackFolderIsNotGuessedWithoutItsComposeFile(t *testing.T) {
	s, in, _ := dockhandStack(t, false)
	if _, dir, ok := s.stackDirFor(context.Background(), in); ok {
		t.Fatalf("stackDirFor = %q, want nothing: no compose file proves a folder is the project's", dir)
	}
	got := s.dataOutsideHostMount(context.Background(), in, nil)
	if len(got) != 1 || got[0] != "/app/data/stacks/DXP6800/arr" {
		t.Fatalf("dataOutsideHostMount = %v, want the folder as Compose names it", got)
	}
}
