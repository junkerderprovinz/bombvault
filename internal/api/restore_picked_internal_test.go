package api

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestPickedStepsStartAtTheFolderThePicksShare(t *testing.T) {
	cases := []struct {
		name    string
		picks   []string
		inPlace bool
		want    []pickedPath
	}{
		{
			name:  "one file",
			picks: []string{"/host/user/appdata/dockhand/stacks/DXP6800/immich/compose.yaml"},
			want:  []pickedPath{{"/host/user/appdata/dockhand/stacks/DXP6800/immich", "/compose.yaml"}},
		},
		{
			name:  "one folder",
			picks: []string{"/host/user/appdata/dockhand/stacks/DXP6800/immich"},
			want:  []pickedPath{{"/host/user/appdata/dockhand/stacks/DXP6800", "/immich"}},
		},
		{
			name: "files from two folders",
			picks: []string{
				"/host/user/appdata/dockhand/stacks/DXP6800/immich/compose.yaml",
				"/host/user/appdata/dockhand/stacks/DXP6800/arr/compose.yaml",
			},
			want: []pickedPath{
				{"/host/user/appdata/dockhand/stacks/DXP6800", "/immich/compose.yaml"},
				{"/host/user/appdata/dockhand/stacks/DXP6800", "/arr/compose.yaml"},
			},
		},
		{
			name:  "folders that only share a name prefix",
			picks: []string{"/data/media/a.txt", "/data/medialib/b.txt"},
			want:  []pickedPath{{"/data", "/media/a.txt"}, {"/data", "/medialib/b.txt"}},
		},
		{
			name:  "picks at the top of the snapshot",
			picks: []string{"/notes.txt", "/docs/a.txt"},
			want:  []pickedPath{{"", "/notes.txt"}, {"", "/docs/a.txt"}},
		},
		{
			name:  "names that read as a glob",
			picks: []string{"/media/[old]/Inception (2010) [1080p]"},
			want:  []pickedPath{{"/media/[old]", `/Inception (2010) \[1080p\]`}},
		},
		{
			name:    "in place",
			picks:   []string{"/host/user/appdata/plex/a.conf", "/host/user/appdata/plex/sub/b.dat"},
			inPlace: true,
			want:    []pickedPath{{"", "/host/user/appdata/plex/a.conf"}, {"", "/host/user/appdata/plex/sub/b.dat"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pickedSteps(c.picks, c.inPlace); !slices.Equal(got, c.want) {
				t.Fatalf("pickedSteps(%q) = %+v, want %+v", c.picks, got, c.want)
			}
		})
	}
}

const dockhandImage = "fnsys/dockhand:latest"

// seedDockhand puts a container on the rig whose appdata holds compose stacks
// a few folders deep, and has the service back it up.
func (r *restoreRig) seedDockhand(t *testing.T) (store.Target, string) {
	t.Helper()
	r.write(t, map[string]string{
		"appdata/dockhand/stacks/DXP6800/immich/compose.yaml": "immich",
		"appdata/dockhand/stacks/DXP6800/immich/.env":         "UPLOAD_LOCATION=/mnt/user/fotos",
		"appdata/dockhand/stacks/DXP6800/arr/compose.yaml":    "arr",
		"appdata/dockhand/dockhand.db":                        "db",
	})
	in := model.Inspect{ID: "id-dockhand", Name: "/dockhand", Running: true, Config: model.Config{Image: dockhandImage}}
	in.HostConfig.Binds = []string{"/mnt/user/appdata/dockhand:/app/data"}
	in.Mounts = []model.Mount{{Type: "bind", Source: "/mnt/user/appdata/dockhand", Destination: "/app/data"}}
	r.docker.containers["dockhand"] = in
	r.docker.images[dockhandImage] = true

	if _, err := r.svc.Backup(context.Background(), "dockhand"); err != nil {
		t.Fatalf("seed backup: %v", err)
	}
	tg, err := r.st.GetTargetByContainer("dockhand")
	if err != nil {
		t.Fatal(err)
	}
	return tg, r.lastRun(t, tg.ID, "backup").SnapshotID
}

func TestPickedContainerFilesLandDirectlyInTheFolder(t *testing.T) {
	cases := []struct {
		name  string
		picks []string
		want  []string
	}{
		{"one file", []string{"stacks/DXP6800/immich/compose.yaml"}, []string{"compose.yaml"}},
		{"one folder", []string{"stacks/DXP6800/immich"}, []string{"immich/.env", "immich/compose.yaml"}},
		{
			"files from two folders",
			[]string{"stacks/DXP6800/immich/compose.yaml", "stacks/DXP6800/arr/compose.yaml"},
			[]string{"arr/compose.yaml", "immich/compose.yaml"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRestoreRig(t)
			tg, snap := r.seedDockhand(t)
			picks := make([]string, len(c.picks))
			for i, p := range c.picks {
				picks[i] = filepath.ToSlash(filepath.Join(r.root, "appdata/dockhand", p))
			}

			res := r.check(t, RestoreCheckRequest{Kind: checkContainerFiles, Name: "dockhand", SnapshotID: snap, Paths: picks, TargetPath: "temp/restores"})
			if !res.Ready || res.Plan == nil || res.Plan.Error != "" {
				t.Fatalf("ready=%v plan=%+v", res.Ready, res.Plan)
			}
			var planned []string
			for _, f := range res.Plan.Files {
				planned = append(planned, strings.TrimPrefix(f.Path, "/mnt/user/temp/restores/"))
			}
			slices.Sort(planned)
			if !slices.Equal(planned, c.want) {
				t.Fatalf("the plan lists %v, want %v", planned, c.want)
			}

			if _, started, err := r.svc.StartRestoreFiles(context.Background(), "dockhand", "local", snap, picks, "temp/restores", true); err != nil || !started {
				t.Fatalf("StartRestoreFiles: started=%v err=%v", started, err)
			}
			r.wait(t)
			if run := r.lastRun(t, tg.ID, "restore"); run.Status != "success" {
				t.Fatalf("restore run = %s: %s", run.Status, run.Error)
			}
			if got := filesBelow(t, filepath.Join(r.root, "temp/restores")); !slices.Equal(got, c.want) {
				t.Fatalf("the folder holds %v, want %v", got, c.want)
			}
		})
	}
}

func TestPickedFileSetFilesFromTwoFoldersKeepThePathBelowTheirSharedFolder(t *testing.T) {
	r := newRestoreRig(t)
	set, snap := r.seedFotos(t)
	picks := []string{
		filepath.ToSlash(filepath.Join(r.root, "fotos/2024/urlaub/strand.jpg")),
		filepath.ToSlash(filepath.Join(r.root, "fotos/2025/garten.jpg")),
	}
	if _, started, err := r.svc.StartRestoreFileSetFiles(context.Background(), set.ID, "local", snap, picks, "restore/fotos", true); err != nil || !started {
		t.Fatalf("StartRestoreFileSetFiles: started=%v err=%v", started, err)
	}
	r.wait(t)
	if run := r.lastRun(t, set.ID, "restore"); run.Status != "success" {
		t.Fatalf("restore run = %s: %s", run.Status, run.Error)
	}
	want := []string{"2024/urlaub/strand.jpg", "2025/garten.jpg"}
	if got := filesBelow(t, filepath.Join(r.root, "restore/fotos")); !slices.Equal(got, want) {
		t.Fatalf("the folder holds %v, want %v", got, want)
	}
}
