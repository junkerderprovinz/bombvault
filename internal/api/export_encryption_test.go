package api_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// exportEncTestSetup wires a container-export Service (mirroring
// TestServiceExportContainer) and returns the service plus the export dir sibling.
func exportEncTestSetup(t *testing.T, recipients string, enable bool) *api.Service {
	t.Helper()
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
		HostSourceRoot:    root,
		FlashTemplatesDir: flash,
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	s.ExportEncryptEnabled = enable
	s.ExportAgeRecipients = recipients
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	appdata := root + "/user/appdata/plex"
	if err := os.MkdirAll(appdata, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appdata, "prefs.xml"), []byte("<Prefs/>"), 0o600); err != nil {
		t.Fatal(err)
	}
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
	return api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{})
}

// TestExportContainerAgeRoundTrip: with export encryption on, the container export
// produces .tar.gz.age + .xml.age (no plaintext), the tar.gz.age is NOT a valid
// gzip on its own, and it decrypts with the in-test identity back to a valid tar.gz
// containing the expected appdata entry. The .xml.age decrypts to the template.
func TestExportContainerAgeRoundTrip(t *testing.T) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	svc := exportEncTestSetup(t, id.Recipient().String(), true)

	out, err := svc.ExportContainer(context.Background(), "plex")
	if err != nil {
		t.Fatalf("ExportContainer: %v", err)
	}

	// The plaintext names must NOT exist; the .age names must.
	for _, plain := range []string{"plex.tar.gz", "plex.xml"} {
		if _, err := os.Stat(filepath.Join(out, plain)); !os.IsNotExist(err) {
			t.Fatalf("plaintext artifact %s must not exist (stat err = %v)", plain, err)
		}
	}
	tarAge := filepath.Join(out, "plex.tar.gz.age")
	xmlAge := filepath.Join(out, "plex.xml.age")
	for _, p := range []string{tarAge, xmlAge} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected %s: %v", p, err)
		}
	}

	// The .age tar is not a valid gzip stream on its own.
	raw, err := os.ReadFile(tarAge) //nolint:gosec // test file
	if err != nil {
		t.Fatal(err)
	}
	if _, gzErr := gzip.NewReader(bytes.NewReader(raw)); gzErr == nil {
		t.Fatal("ciphertext must not parse as gzip")
	}

	// Decrypt and read the tar entries.
	names := decryptedTarEntryNames(t, raw, id)
	if !contains(names, "user/appdata/plex/prefs.xml") {
		t.Fatalf("decrypted tar should contain the appdata entry, got %v", names)
	}

	// The .xml.age decrypts to the template.
	xmlRaw, err := os.ReadFile(xmlAge) //nolint:gosec // test file
	if err != nil {
		t.Fatal(err)
	}
	dr, err := age.Decrypt(bytes.NewReader(xmlRaw), id)
	if err != nil {
		t.Fatalf("decrypt xml: %v", err)
	}
	xmlPlain, _ := io.ReadAll(dr)
	if !strings.Contains(string(xmlPlain), "<Name>plex</Name>") {
		t.Fatalf("decrypted xml mismatch: %q", xmlPlain)
	}
}

// TestExportContainerEncryptionNoRecipientFailsLoud: encryption on + no recipient
// must return an error and write NO artifact (never a plaintext fallback).
func TestExportContainerEncryptionNoRecipientFailsLoud(t *testing.T) {
	svc := exportEncTestSetup(t, "   ", true) // enabled but no valid recipient

	out, err := svc.ExportContainer(context.Background(), "plex")
	if err == nil {
		t.Fatal("expected a hard error when encryption is on with no recipient")
	}
	// exportDir is a deterministic sibling of the repo; assert it holds no export.
	if out != "" {
		if entries, rerr := os.ReadDir(out); rerr == nil {
			for _, e := range entries {
				t.Fatalf("no artifact may be written on fail-loud, found %s", e.Name())
			}
		}
	}
}

// decryptedTarEntryNames decrypts an age blob with id and returns the gzip-tar
// entry names inside.
func decryptedTarEntryNames(t *testing.T, cipher []byte, id *age.X25519Identity) []string {
	t.Helper()
	r, err := age.Decrypt(bytes.NewReader(cipher), id)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	gz, err := gzip.NewReader(r)
	if err != nil {
		t.Fatalf("gzip: %v", err)
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
