package api

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const fileBrowserImage = "filebrowser/filebrowser:v2"

// fileBrowserAtBackup is what the backup of the file browser holds, by path
// below the Host Data mount.
var fileBrowserAtBackup = map[string]string{
	"appdata/filebrowser/data/notes.txt":  "notes",
	"appdata/filebrowser/data/custom.css": "body {}",
	"appdata/filebrowser/filebrowser.db":  "database at backup time",
	"appdata/filebrowser.json":            `{"port":80}`,
}

// seedFileBrowser puts a container on the rig that binds a folder, a file
// inside that folder, a database file next to it and a settings file directly
// in the appdata share, the way FileBrowser is set up on Unraid, and has the
// service back it up.
func (r *restoreRig) seedFileBrowser(t *testing.T) (store.Target, string) {
	t.Helper()
	r.write(t, fileBrowserAtBackup)
	if err := os.Chmod(filepath.Join(r.root, "appdata/filebrowser/filebrowser.db"), 0o640); err != nil { //nolint:gosec // G302: a mode the restore has to bring back
		t.Fatal(err)
	}
	binds := [][2]string{
		{"/mnt/user/appdata/filebrowser/data", "/srv"},
		{"/mnt/user/appdata/filebrowser/data/custom.css", "/branding/custom.css"},
		{"/mnt/user/appdata/filebrowser/filebrowser.db", "/database.db"},
		{"/mnt/user/appdata/filebrowser.json", "/.filebrowser.json"},
	}
	in := model.Inspect{ID: "id-filebrowser", Name: "/filebrowser", Running: true, Config: model.Config{Image: fileBrowserImage}}
	for _, b := range binds {
		in.HostConfig.Binds = append(in.HostConfig.Binds, b[0]+":"+b[1])
		in.Mounts = append(in.Mounts, model.Mount{Type: "bind", Source: b[0], Destination: b[1]})
	}
	in.HostConfig.PortBindings = map[string][]model.PortBinding{"80/tcp": {{HostPort: "8090"}}}
	r.docker.containers["filebrowser"] = in
	r.docker.images[fileBrowserImage] = true

	if _, err := r.svc.Backup(context.Background(), "filebrowser"); err != nil {
		t.Fatalf("seed backup: %v", err)
	}
	tg, err := r.st.GetTargetByContainer("filebrowser")
	if err != nil {
		t.Fatal(err)
	}
	return tg, r.lastRun(t, tg.ID, "backup").SnapshotID
}

// holds fails unless every file below base has the content the backup held.
func (r *restoreRig) holds(t *testing.T, base func(string) string) {
	t.Helper()
	for rel, want := range fileBrowserAtBackup {
		p := base(filepath.Join(r.root, rel))
		info, err := os.Lstat(p)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("%s is not a file after the restore: %v %v", p, info, err)
		}
		if got := read(t, p); got != want {
			t.Fatalf("%s = %q, want %q", p, got, want)
		}
	}
}

func (r *restoreRig) restoreFileBrowser(t *testing.T, tg store.Target, snap string, opts RestoreOptions) {
	t.Helper()
	started, err := r.svc.StartRestore(context.Background(), "filebrowser", snap, "local", opts)
	if err != nil || !started {
		t.Fatalf("StartRestore: started=%v err=%v", started, err)
	}
	r.wait(t)
	if run := r.lastRun(t, tg.ID, "restore"); run.Status != "success" {
		t.Fatalf("restore run = %s: %s", run.Status, run.Error)
	}
}

func TestRestoreContainerWithFileBindsAfterTheContainerAndItsFilesWereDeleted(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedFileBrowser(t)
	delete(r.docker.containers, "filebrowser")
	for _, rel := range []string{"appdata/filebrowser", "appdata/filebrowser.json"} {
		if err := os.RemoveAll(filepath.Join(r.root, rel)); err != nil {
			t.Fatal(err)
		}
	}
	r.docker.forget()

	res := r.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "filebrowser", SnapshotID: snap})
	if !res.Ready || res.Plan == nil || res.Plan.Error != "" {
		t.Fatalf("ready=%v checks=%+v plan=%+v", res.Ready, res.Checks, res.Plan)
	}
	for _, rel := range []string{"appdata/filebrowser/filebrowser.db", "appdata/filebrowser.json"} {
		want := PlanFile{Path: "/mnt/user/" + rel, Change: changeAdded}
		if !slices.Contains(res.Plan.Files, want) {
			t.Fatalf("the plan does not add %s: %+v", rel, res.Plan.Files)
		}
	}

	r.restoreFileBrowser(t, tg, snap, RestoreOptions{})

	r.holds(t, func(p string) string { return p })
	if info, err := os.Stat(filepath.Join(r.root, "appdata/filebrowser/filebrowser.db")); err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("the database lost its mode: %v %v", info, err)
	}
	if calls := r.docker.did("create"); !slices.Equal(calls, []string{"create:filebrowser:start=true"}) {
		t.Fatalf("docker calls = %v", calls)
	}
}

func TestRestoreContainerWithFileBindsOverTheFilesThatAreStillThere(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedFileBrowser(t)
	r.write(t, map[string]string{
		"appdata/filebrowser/filebrowser.db":  "a newer database",
		"appdata/filebrowser.json":            `{"port":8080}`,
		"appdata/filebrowser/never-bound.txt": "not in any bind",
	})
	r.docker.forget()

	res := r.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "filebrowser", SnapshotID: snap, BackupFirst: true})
	if !res.Ready || res.Plan == nil || res.Plan.Error != "" {
		t.Fatalf("ready=%v checks=%+v plan=%+v", res.Ready, res.Checks, res.Plan)
	}
	for _, rel := range []string{"appdata/filebrowser/filebrowser.db", "appdata/filebrowser.json"} {
		want := PlanFile{Path: "/mnt/user/" + rel, Change: changeChanged}
		if !slices.Contains(res.Plan.Files, want) {
			t.Fatalf("the plan does not replace %s: %+v", rel, res.Plan.Files)
		}
	}
	if slices.ContainsFunc(res.Plan.Files, func(f PlanFile) bool { return strings.HasSuffix(f.Path, "never-bound.txt") }) {
		t.Fatalf("the plan lists a file no bind reaches: %+v", res.Plan.Files)
	}

	r.restoreFileBrowser(t, tg, snap, RestoreOptions{BackupFirst: true})

	r.holds(t, func(p string) string { return p })
	if got := read(t, filepath.Join(r.root, "appdata/filebrowser/never-bound.txt")); got != "not in any bind" {
		t.Fatalf("a file no bind reaches changed: %q", got)
	}
	before := r.stateBefore(t, "containers", "container:filebrowser", snap)
	if !slices.Contains(before.Paths, filepath.Join(r.root, "appdata/filebrowser/filebrowser.db")) {
		t.Fatalf("the backup before the restore holds %v", before.Paths)
	}
}

func TestRestoreContainerWithFileBindsBesideTheOriginal(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedFileBrowser(t)
	r.write(t, map[string]string{"appdata/filebrowser/filebrowser.db": "a newer database"})
	appdata := filepath.Join(r.root, "appdata")
	before := treeSum(t, appdata, true)
	r.docker.forget()

	res := r.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "filebrowser", SnapshotID: snap, Target: targetBeside})
	if !res.Ready || res.Plan == nil || res.Plan.Error != "" {
		t.Fatalf("ready=%v checks=%+v plan=%+v", res.Ready, res.Checks, res.Plan)
	}
	wantFolders := []BesideFolder{
		{Original: "/mnt/user/appdata/filebrowser", Copy: "/mnt/user/appdata/filebrowser-restored"},
		{Original: "/mnt/user/appdata/filebrowser.json", Copy: "/mnt/user/appdata/filebrowser.json-restored"},
	}
	if p := res.Plan; p.Beside == nil || !slices.Equal(p.Beside.Folders, wantFolders) || p.Changed != 0 || p.Extra != 0 {
		t.Fatalf("plan = %+v, beside %+v", p, p.Beside)
	}
	for _, rel := range []string{"appdata/filebrowser-restored/filebrowser.db", "appdata/filebrowser.json-restored"} {
		want := PlanFile{Path: "/mnt/user/" + rel, Change: changeAdded}
		if !slices.Contains(res.Plan.Files, want) {
			t.Fatalf("the plan does not add %s: %+v", rel, res.Plan.Files)
		}
	}
	if treeSum(t, appdata, true) != before {
		t.Fatal("a check changed the appdata share")
	}

	started, err := r.svc.StartRestore(context.Background(), "filebrowser", snap, "local", RestoreOptions{Beside: true})
	if err != nil || !started {
		t.Fatalf("StartRestore: started=%v err=%v", started, err)
	}
	r.wait(t)
	if run := r.lastRun(t, tg.ID, "restore"); run.Status != "success" {
		t.Fatalf("restore run = %s: %s", run.Status, run.Error)
	}

	r.holds(t, func(p string) string {
		rel, _ := filepath.Rel(appdata, p)
		top, rest, _ := strings.Cut(filepath.ToSlash(rel), "/")
		return filepath.Join(appdata, top+besideSuffix, rest)
	})
	if got := entries(t, appdata); !slices.Equal(got, []string{"filebrowser", "filebrowser-restored", "filebrowser.json", "filebrowser.json-restored"}) {
		t.Fatalf("the appdata share holds %v", got)
	}
	if got := read(t, filepath.Join(appdata, "filebrowser/filebrowser.db")); got != "a newer database" {
		t.Fatalf("the original's database = %q", got)
	}
	copied, ok := r.docker.container("filebrowser" + besideSuffix)
	if !ok {
		t.Fatal("no copy was created")
	}
	wantBinds := []string{
		"/mnt/user/appdata/filebrowser-restored/data:/srv",
		"/mnt/user/appdata/filebrowser-restored/data/custom.css:/branding/custom.css",
		"/mnt/user/appdata/filebrowser-restored/filebrowser.db:/database.db",
		"/mnt/user/appdata/filebrowser.json-restored:/.filebrowser.json",
	}
	if !slices.Equal(copied.HostConfig.Binds, wantBinds) {
		t.Fatalf("the copy binds %v", copied.HostConfig.Binds)
	}
}
