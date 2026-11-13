package api

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// memberWithRestic is a member of a shared repository in a real restic
// repository, which this server and Cube both backed up a folder set called
// docs into, and Cube a set called photos as well. The server backs up under
// key.
func memberWithRestic(t *testing.T, key string) (*resticFixture, store.SharedRepo, string, restic.Mode) {
	t.Helper()
	f := newResticFixture(t)
	f.svc.cfg.AppKey = key
	f.svc.forgetBackupKey()
	here := f.svc.resticHost()
	sr, err := f.st.PutSharedRepo(store.SharedRepo{Repo: "backups/shared", Owner: "atlas-id", Members: []store.SharedMember{
		cube, {Host: here, Instance: "here-id", Name: "Here"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	target, err := f.st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "files", Name: "Shared with Atlas", Repo: "backups/shared", Enabled: true, Immutable: true})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	dest, err := f.svc.resolveRepo(target.Repo)
	if err != nil {
		t.Fatal(err)
	}
	mode := f.svc.offsiteModeForTarget(settings, target)
	ctx := context.Background()
	if err := f.svc.engine.Init(ctx, dest, mode); err != nil {
		t.Fatal(err)
	}
	f.write("here/docs", map[string]string{"letter.txt": "written here"})
	f.write("cube/docs", map[string]string{"letter.txt": "written on Cube"})
	f.write("cube/photos", map[string]string{"beach.jpg": "a beach on Cube"})
	for _, b := range []struct{ host, dir, tag string }{
		{here, "here/docs", "fileset:docs"},
		{cubeHost, "cube/docs", "fileset:docs"},
		{cubeHost, "cube/photos", "fileset:photos"},
	} {
		m := mode
		m.Host = b.host
		if _, err := f.svc.engine.Backup(ctx, dest, []string{filepath.Join(filepath.FromSlash(f.root), "src", filepath.FromSlash(b.dir))}, []string{b.tag}, m); err != nil {
			t.Fatal(err)
		}
	}
	return f, sr, dest, mode
}

func TestThisServerListsOnlyItsOwnBackupsInASharedRepositoryWithRestic(t *testing.T) {
	f, _, dest, mode := memberWithRestic(t, strings.Repeat("a", 64))
	snaps, err := f.svc.listSnapshots(context.Background(), dest, mode)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].Hostname != f.svc.resticHost() {
		t.Fatalf("this server lists %+v", snaps)
	}
}

func TestAnotherServersFolderSetComesBackFromASharedRepositoryWithRestic(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("restic's path filters and <id>:<path> selector cannot carry a Windows drive letter")
	}
	f, sr, _, _ := memberWithRestic(t, strings.Repeat("a", 64))

	var servers []sharedOtherServer
	decodeInto(t, f.do("GET", "/api/shared-repo/others", nil), "servers", &servers)
	if len(servers) != 1 || servers[0].Host != cubeHost || servers[0].Name != "Cube" || servers[0].Entries != 2 {
		t.Fatalf("other servers %+v", servers)
	}

	res := f.do("POST", "/api/shared-repo/others/open", map[string]any{"repo": sr.ID, "host": cubeHost})
	var inv ForeignInventory
	decodeInto(t, res, "inventory", &inv)
	names := []string{}
	for _, it := range inv.FileSets {
		names = append(names, it.Name)
		for _, sn := range it.Snapshots {
			if sn.Hostname != cubeHost {
				t.Errorf("%s holds a snapshot of %s", it.Name, sn.Hostname)
			}
		}
	}
	if !slices.Equal(names, []string{"docs", "photos"}) {
		t.Fatalf("Cube's folder sets read %v", names)
	}

	f.do("POST", "/api/foreign/restore", map[string]any{
		"session": res["session"], "domain": "files", "item": "docs", "snapshot": "latest", "confirm": true, "target": "restored/docs",
	})
	f.wait()
	if got := readBelow(t, filepath.Join(filepath.FromSlash(f.root), "restored", "docs"), "letter.txt"); got != "written on Cube" {
		t.Fatalf("the restored letter reads %q, want Cube's", got)
	}
}

// readBelow reads the first file called name anywhere below dir.
func readBelow(t *testing.T, dir, name string) string {
	t.Helper()
	var found string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || found != "" || d.IsDir() || d.Name() != name {
			return err
		}
		b, err := os.ReadFile(p) //nolint:gosec // a file the test restored
		found = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}
