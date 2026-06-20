package api_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// TestServiceExportContainer verifies the plain (tool-free) export: a container
// with appdata + an Unraid template produces a browsable <name>.tar.gz (whose
// entries reconstruct the appdata layout) and a <name>.xml next to the repo.
func TestServiceExportContainer(t *testing.T) {
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	flash := filepath.Join(dir, "flash")
	if err := os.MkdirAll(flash, 0o750); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		AppKey:            strings.Repeat("a", 64),
		DataDir:           dir,
		HostMountRoot:     root,
		HostSourceRoot:    root, // identity translation: bind source == container path
		FlashTemplatesDir: flash,
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers" // local repo → export folder is its sibling
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	// Appdata to export, with a real file inside it.
	appdata := root + "/user/appdata/plex"
	if err := os.MkdirAll(appdata, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appdata, "prefs.xml"), []byte("<Prefs/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	// The Unraid template, stored as my-<name>.xml.
	if err := os.WriteFile(filepath.Join(flash, "my-plex.xml"), []byte("<Container><Name>plex</Name></Container>"), 0o600); err != nil {
		t.Fatal(err)
	}

	d := &fakeServiceDocker{inspect: model.Inspect{
		Name:  "/plex",
		Image: "plexinc/pms-docker:latest",
		Mounts: []model.Mount{
			{Type: "bind", Source: root + "/user/appdata/plex", Destination: "/config"},
		},
	}}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{})

	out, err := svc.ExportContainer(context.Background(), "plex")
	if err != nil {
		t.Fatalf("ExportContainer: %v", err)
	}

	// Both artifacts exist.
	xmlPath := filepath.Join(out, "plex.xml")
	tarPath := filepath.Join(out, "plex.tar.gz")
	if _, err := os.Stat(xmlPath); err != nil {
		t.Fatalf("expected template xml at %s: %v", xmlPath, err)
	}
	if _, err := os.Stat(tarPath); err != nil {
		t.Fatalf("expected tar.gz at %s: %v", tarPath, err)
	}

	// The tar reconstructs the appdata file relative to the host mount root.
	names := tarEntryNames(t, tarPath)
	want := "user/appdata/plex/prefs.xml"
	if !contains(names, want) {
		t.Fatalf("tar should contain %q, got %v", want, names)
	}
}

// TestServiceExportContainerRejectsUnsafeName guards the filename-injection check.
func TestServiceExportContainerRejectsUnsafeName(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: filepath.ToSlash(dir)}
	st := newMemStore(t)
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})

	for _, bad := range []string{"../etc", "a/b", `a\b`} {
		if _, err := svc.ExportContainer(context.Background(), bad); err == nil {
			t.Fatalf("expected ExportContainer(%q) to be rejected", bad)
		}
	}
}

// tarEntryNames returns the slash-joined entry names in a gzip-compressed tar.
func tarEntryNames(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // test
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close() //nolint:errcheck // test
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, hdr.Name)
	}
	return names
}
