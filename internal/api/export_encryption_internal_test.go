package api

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ageRoundTripDecrypt decrypts an age blob with id and returns the plaintext.
func ageRoundTripDecrypt(t *testing.T, cipher []byte, id *age.X25519Identity) []byte {
	t.Helper()
	r, err := age.Decrypt(bytes.NewReader(cipher), id)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return out
}

// TestExportRecipients covers the resolve-once gate: off yields nothing, on with a
// valid recipient yields it, on with an empty/invalid list is a hard error.
func TestExportRecipients(t *testing.T) {
	svc := &Service{}
	id, _ := age.GenerateX25519Identity()

	if rs, on, err := svc.exportRecipients(store.Settings{}); err != nil || on || rs != nil {
		t.Fatalf("disabled must yield (nil,false,nil), got (%v,%v,%v)", rs, on, err)
	}
	rs, on, err := svc.exportRecipients(store.Settings{ExportEncryptEnabled: true, ExportAgeRecipients: id.Recipient().String()})
	if err != nil || !on || len(rs) != 1 {
		t.Fatalf("enabled+valid must yield one recipient, got (%v,%v,%v)", rs, on, err)
	}
	for _, bad := range []string{"", "   ", "garbage"} {
		if _, on, err := svc.exportRecipients(store.Settings{ExportEncryptEnabled: true, ExportAgeRecipients: bad}); err == nil || !on {
			t.Fatalf("enabled+%q must be a hard error with on=true, got on=%v err=%v", bad, on, err)
		}
	}
}

// TestWriteTarGzAgeRoundTrip: writeTarGz with recipients writes dest+".age"
// ciphertext (not dest), and the archive decrypts to a valid gzip tar.
func TestWriteTarGzAgeRoundTrip(t *testing.T) {
	root := t.TempDir()
	// A source file under the mount root.
	src := filepath.Join(root, "data")
	if err := os.MkdirAll(src, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "f.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := &Service{cfg: config.Config{HostMountRoot: root}}
	id, _ := age.GenerateX25519Identity()

	dest := filepath.Join(root, "out.tar.gz")
	final, err := svc.writeTarGz(dest, []string{src}, []age.Recipient{id.Recipient()})
	if err != nil {
		t.Fatal(err)
	}
	if final != dest+".age" {
		t.Fatalf("final path = %q, want %q", final, dest+".age")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("plaintext dest must not exist, stat err = %v", err)
	}
	raw, err := os.ReadFile(final) //nolint:gosec // test file
	if err != nil {
		t.Fatal(err)
	}
	// Decrypt -> should be a valid gzip tar carrying data/f.txt.
	plain := ageRoundTripDecrypt(t, raw, id)
	if !bytes.HasPrefix(plain, []byte{0x1f, 0x8b}) {
		t.Fatal("decrypted payload is not gzip")
	}
}

// TestWriteExportFileAge: writeExportFile with recipients writes path+".age" and
// decrypts back to the original bytes; without recipients it writes plaintext.
func TestWriteExportFileAge(t *testing.T) {
	dir := t.TempDir()
	id, _ := age.GenerateX25519Identity()
	payload := []byte("<domain/>")

	// Encrypted.
	final, err := writeExportFile(filepath.Join(dir, "vm.xml"), payload, []age.Recipient{id.Recipient()})
	if err != nil {
		t.Fatal(err)
	}
	if final != filepath.Join(dir, "vm.xml.age") {
		t.Fatalf("final = %q", final)
	}
	raw, _ := os.ReadFile(final) //nolint:gosec // test file
	if got := ageRoundTripDecrypt(t, raw, id); !bytes.Equal(got, payload) {
		t.Fatalf("round-trip mismatch: %q", got)
	}

	// Plaintext.
	pfinal, err := writeExportFile(filepath.Join(dir, "plain.xml"), payload, nil)
	if err != nil {
		t.Fatal(err)
	}
	if pfinal != filepath.Join(dir, "plain.xml") {
		t.Fatalf("plain final = %q", pfinal)
	}
	if got, _ := os.ReadFile(pfinal); !bytes.Equal(got, payload) { //nolint:gosec // test file
		t.Fatal("plaintext write mismatch")
	}
}

// TestExportFlashZipAgeRoundTrip: with encryption on, exportFlashZip writes
// flash-latest.zip.age (no plaintext .zip) and it decrypts to the DumpZip payload.
func TestExportFlashZipAgeRoundTrip(t *testing.T) {
	root := t.TempDir()
	payload := []byte("PK\x03\x04sealed")
	fake := &flashZipFakeEngine{dumpBytes: payload}
	id, _ := age.GenerateX25519Identity()
	svc := &Service{
		cfg:    config.Config{HostMountRoot: root, FlashDir: "/boot"},
		engine: fake,
		store:  newFlashExportStore(t),
	}
	settings := store.Settings{
		FlashZipExportEnabled: true,
		FlashZipExportPath:    "export",
		ExportEncryptEnabled:  true,
		ExportAgeRecipients:   id.Recipient().String(),
	}
	if err := svc.exportFlashZip(context.Background(), settings, "deadbeef", restic.Mode{}, "/repo"); err != nil {
		t.Fatalf("exportFlashZip: %v", err)
	}
	dir := filepath.Join(root, "export")
	if _, err := os.Stat(filepath.Join(dir, "flash-latest.zip")); !os.IsNotExist(err) {
		t.Fatalf("plaintext zip must not exist, stat err = %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "flash-latest.zip.age")) //nolint:gosec // test file
	if err != nil {
		t.Fatalf("read sealed zip: %v", err)
	}
	if got := ageRoundTripDecrypt(t, raw, id); !bytes.Equal(got, payload) {
		t.Fatalf("decrypted zip = %q, want the DumpZip payload", got)
	}
	// The plaintext temp must be gone.
	if _, err := os.Stat(filepath.Join(dir, ".flash-export.tmp.zip")); !os.IsNotExist(err) {
		t.Fatalf("temp must be removed, stat err = %v", err)
	}
}

// TestExportFlashZipEncryptionNoRecipientFailsLoud: encryption on + no recipient
// errors and writes NO artifact (no zip, no temp, no export folder content).
func TestExportFlashZipEncryptionNoRecipientFailsLoud(t *testing.T) {
	root := t.TempDir()
	fake := &flashZipFakeEngine{dumpBytes: []byte("nope")}
	svc := &Service{
		cfg:    config.Config{HostMountRoot: root, FlashDir: "/boot"},
		engine: fake,
		store:  newFlashExportStore(t),
	}
	settings := store.Settings{
		FlashZipExportEnabled: true,
		FlashZipExportPath:    "export",
		ExportEncryptEnabled:  true,
		ExportAgeRecipients:   "", // no recipient
	}
	if err := svc.exportFlashZip(context.Background(), settings, "deadbeef", restic.Mode{}, "/repo"); err == nil {
		t.Fatal("expected a hard error when encryption is on with no recipient")
	}
	if fake.dumpCalls != 0 {
		t.Fatalf("DumpZip must not run before recipients are resolved (calls = %d)", fake.dumpCalls)
	}
	if entries, err := os.ReadDir(filepath.Join(root, "export")); err == nil {
		for _, e := range entries {
			t.Fatalf("no artifact may be written on fail-loud, found %s", e.Name())
		}
	}
}

// downloadFakeEngine serves Snapshots + DumpZip for DownloadFlashZip tests.
type downloadFakeEngine struct {
	ResticEngine
	snaps     []restic.Snapshot
	dumpBytes []byte
	dumpCalls int
}

func (f *downloadFakeEngine) Snapshots(_ context.Context, _ string, _ restic.Mode) ([]restic.Snapshot, error) {
	return f.snaps, nil
}

func (f *downloadFakeEngine) DumpZip(_ context.Context, _, _, _ string, w io.Writer, _ restic.Mode) error {
	f.dumpCalls++
	_, err := w.Write(f.dumpBytes)
	return err
}

// TestDownloadFlashZipAgeRoundTrip: with encryption on, the streamed bytes are an
// age blob that decrypts to the DumpZip payload.
func TestDownloadFlashZipAgeRoundTrip(t *testing.T) {
	root := t.TempDir()
	payload := []byte("PK\x03\x04streamed")
	fake := &downloadFakeEngine{
		snaps:     []restic.Snapshot{{ID: "aaaa1111bbbb2222"}},
		dumpBytes: payload,
	}
	id, _ := age.GenerateX25519Identity()
	st := newFlashExportStore(t)
	s := mustLoadSettings(t, st)
	s.FlashPath = "backups/flash"
	s.ExportEncryptEnabled = true
	s.ExportAgeRecipients = id.Recipient().String()
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	svc := &Service{cfg: config.Config{HostMountRoot: root, FlashDir: "/boot", AppKey: strings.Repeat("a", 64)}, engine: fake, store: st}

	var buf bytes.Buffer
	var resolved string
	if err := svc.DownloadFlashZip(context.Background(), "latest", "", func(id string) { resolved = id }, &buf); err != nil {
		t.Fatalf("DownloadFlashZip: %v", err)
	}
	if resolved != "aaaa1111bbbb2222" {
		t.Fatalf("resolved = %q", resolved)
	}
	if bytes.Equal(buf.Bytes(), payload) {
		t.Fatal("stream must be ciphertext, not the raw zip payload")
	}
	if got := ageRoundTripDecrypt(t, buf.Bytes(), id); !bytes.Equal(got, payload) {
		t.Fatalf("decrypted stream = %q, want the payload", got)
	}
}

// TestDownloadFlashZipEncryptionNoRecipientFailsLoud: encryption on + no recipient
// errors BEFORE onResolved fires and BEFORE any bytes are written.
func TestDownloadFlashZipEncryptionNoRecipientFailsLoud(t *testing.T) {
	root := t.TempDir()
	fake := &downloadFakeEngine{snaps: []restic.Snapshot{{ID: "aaaa1111bbbb2222"}}, dumpBytes: []byte("x")}
	st := newFlashExportStore(t)
	s := mustLoadSettings(t, st)
	s.FlashPath = "backups/flash"
	s.ExportEncryptEnabled = true
	s.ExportAgeRecipients = "" // no recipient
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	svc := &Service{cfg: config.Config{HostMountRoot: root, FlashDir: "/boot", AppKey: strings.Repeat("a", 64)}, engine: fake, store: st}

	var buf bytes.Buffer
	called := false
	err := svc.DownloadFlashZip(context.Background(), "latest", "", func(string) { called = true }, &buf)
	if err == nil {
		t.Fatal("expected a hard error when encryption is on with no recipient")
	}
	if called {
		t.Fatal("onResolved must not fire on fail-loud (headers would be committed)")
	}
	if buf.Len() != 0 {
		t.Fatal("no bytes may be written on fail-loud")
	}
	if fake.dumpCalls != 0 {
		t.Fatalf("DumpZip must not run (calls = %d)", fake.dumpCalls)
	}
}

// mustLoadSettings returns the migrated store's current settings row.
func mustLoadSettings(t *testing.T, st *store.Repo) store.Settings {
	t.Helper()
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	return s
}
