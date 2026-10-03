package api

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// flashFakeEngine serves one flash snapshot from a map of file contents and
// restores the files an include pattern selects, the way restic does: a match
// on a folder brings everything below it.
type flashFakeEngine struct {
	ResticEngine
	root  string
	files map[string]string
}

func (f *flashFakeEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return []restic.Snapshot{{ID: "f1a5b00100000000", Paths: []string{f.root}, Tags: []string{"flash"}}}, nil
}

func (f *flashFakeEngine) Ls(context.Context, string, string, restic.Mode) ([]restic.FileEntry, error) {
	var out []restic.FileEntry
	for p, c := range f.files {
		out = append(out, restic.FileEntry{Path: p, Type: "file", Size: int64(len(c))})
	}
	return out, nil
}

func (f *flashFakeEngine) RestoreInclude(_ context.Context, _, _, include, target string, _ restic.Mode) error {
	for p, c := range f.files {
		hit := false
		for q := p; q != "/" && q != "."; q = path.Dir(q) {
			if ok, _ := path.Match(include, q); ok {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		dst := filepath.Join(target, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(dst, []byte(c), 0o600); err != nil {
			return err
		}
	}
	return nil
}

const unassignedPlg = `<?xml version='1.0' standalone='yes'?>
<!DOCTYPE PLUGIN [
<!ENTITY name      "unassigned.devices">
<!ENTITY version   "2026.09.01">
<!ENTITY pkg       "/boot/config/plugins/&name;/&name;-&version;.tgz">
]>
<PLUGIN name="&name;" version="&version;" min="6.12">
<FILE Name="&pkg;" Run="upgradepkg --install-new">
<URL>https://example.invalid/&name;.tgz</URL>
</FILE>
<FILE Name="/boot/extra/helper-1.0.txz" Run="upgradepkg --install-new"/>
<FILE Run="/bin/bash"><INLINE>
[ -f /tmp/x ] && echo ok
</INLINE></FILE>
</PLUGIN>`

func flashPluginFixture(t *testing.T) (*Service, string, *flashFakeEngine) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	flash := filepath.Join(dir, "boot")
	if err := os.MkdirAll(filepath.Join(flash, "config", "plugins", "other"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(flash, "config", "plugins", "other", "keep.cfg"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := "/host/boot"
	eng := &flashFakeEngine{root: root, files: map[string]string{
		root + "/config/plugins/unassigned.devices.plg":                               unassignedPlg,
		root + "/config/plugins/unassigned.devices/unassigned.devices.cfg":            "cfg",
		root + "/config/plugins/unassigned.devices/unassigned.devices-2026.09.01.tgz": "package!",
		root + "/extra/helper-1.0.txz":                                                "helper",
		root + "/extra/unrelated.txz":                                                 "unrelated",
		root + "/config/plugins/other.plg":                                            `<PLUGIN name="other" version='3'></PLUGIN>`,
		root + "/config/plugins-removed/gone.plg":                                     "<PLUGIN version=\"1\"/>",
		root + "/config/ident.cfg":                                                    "ident",
	}}
	svc := &Service{
		cfg:    config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: filepath.ToSlash(dir), FlashDir: flash},
		store:  store.New(db),
		engine: eng,
	}
	return svc, flash, eng
}

func TestFlashPluginsListsNameVersionAndSize(t *testing.T) {
	svc, _, _ := flashPluginFixture(t)
	got, err := svc.FlashPlugins(context.Background(), "latest", "local")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "other" || got[1].Name != "unassigned.devices" {
		t.Fatalf("plugins = %+v, want other and unassigned.devices (removed plugins stay out)", got)
	}
	ud := got[1]
	if ud.Version != "2026.09.01" {
		t.Fatalf("version = %q, want the entity expanded", ud.Version)
	}
	want := int64(len(unassignedPlg) + len("cfg") + len("package!") + len("helper"))
	if ud.Size != want {
		t.Fatalf("size = %d, want %d (plg, folder and the package outside it)", ud.Size, want)
	}
	if len(ud.Packages) != 1 || ud.Packages[0] != "/boot/extra/helper-1.0.txz" {
		t.Fatalf("packages = %v, want only the one outside the plugin folder", ud.Packages)
	}
	if got[0].Version != "3" {
		t.Fatalf("single-quoted version = %q", got[0].Version)
	}
}

func TestRestoreFlashPluginWritesOnlyThatPlugin(t *testing.T) {
	svc, flash, _ := flashPluginFixture(t)
	started, err := svc.StartRestoreFlashPlugin(context.Background(), "latest", "local", "unassigned.devices", true)
	if err != nil || !started {
		t.Fatalf("start: %v %v", started, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for svc.batchActive.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	for rel, want := range map[string]string{
		"config/plugins/unassigned.devices.plg":                               unassignedPlg,
		"config/plugins/unassigned.devices/unassigned.devices-2026.09.01.tgz": "package!",
		"extra/helper-1.0.txz":                                                "helper",
		"config/plugins/other/keep.cfg":                                       "keep",
	} {
		b, err := os.ReadFile(filepath.Join(flash, filepath.FromSlash(rel))) //nolint:gosec // G304: a path inside the test's own temp folder
		if err != nil || string(b) != want {
			t.Errorf("%s = %q, %v; want %q", rel, b, err, want)
		}
	}
	for _, rel := range []string{"extra/unrelated.txz", "config/ident.cfg", "config/plugins/other.plg"} {
		if _, err := os.Stat(filepath.Join(flash, filepath.FromSlash(rel))); err == nil {
			t.Errorf("%s was written, but it belongs to no restored plugin", rel)
		}
	}
	leftovers, _ := filepath.Glob(filepath.Join(svc.cfg.DataDir, "flash-plugin*"))
	if len(leftovers) != 0 {
		t.Errorf("scratch folders left behind: %v", leftovers)
	}
}

func TestRestoreFlashPluginRefusesUnsafeOrUnknownNames(t *testing.T) {
	svc, _, _ := flashPluginFixture(t)
	for _, name := range []string{"", "..", "../../config", "a/b", "missing"} {
		if started, err := svc.StartRestoreFlashPlugin(context.Background(), "latest", "local", name, true); err == nil || started {
			t.Errorf("name %q was accepted", name)
		}
	}
	if _, err := svc.StartRestoreFlashPlugin(context.Background(), "latest", "local", "other", false); err == nil {
		t.Error("a restore without confirmation was accepted")
	}
}
