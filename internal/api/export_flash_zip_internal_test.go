package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// flashZipFakeEngine is a minimal ResticEngine for the flash-zip-export tests.
// It embeds the interface (nil) so the struct satisfies ResticEngine without
// stubbing all ~22 methods; exportFlashZip only ever calls DumpZip, which is the
// single method implemented here (any other call would panic — none happens).
type flashZipFakeEngine struct {
	ResticEngine // embedded interface value is nil on purpose; only DumpZip is used
	dumpBytes    []byte
	dumpErr      error
	dumpCalls    int
}

func (f *flashZipFakeEngine) DumpZip(_ context.Context, _, _, _ string, w io.Writer, _ restic.Mode) error {
	f.dumpCalls++
	if f.dumpErr != nil {
		return f.dumpErr
	}
	_, err := w.Write(f.dumpBytes)
	return err
}

// TestExportFlashZipKeep0 proves the default (Keep==0) path: the snapshot lands
// at <dir>/flash-latest.zip with exactly the bytes DumpZip wrote, the atomic temp
// file is gone, and a second export overwrites flash-latest.zip in place.
func TestExportFlashZipKeep0(t *testing.T) {
	root := t.TempDir()
	fake := &flashZipFakeEngine{dumpBytes: []byte("PK\x03\x04first")}
	svc := &Service{
		cfg:    config.Config{HostMountRoot: root, FlashDir: "/boot"},
		engine: fake,
	}
	settings := store.Settings{
		FlashZipExportEnabled: true,
		FlashZipExportPath:    "export",
		FlashZipExportKeep:    0,
	}

	if err := svc.exportFlashZip(context.Background(), settings, "deadbeef", restic.Mode{}, "/repo"); err != nil {
		t.Fatalf("exportFlashZip: %v", err)
	}

	dir := filepath.Join(root, "export")
	latest := filepath.Join(dir, "flash-latest.zip")
	got, err := os.ReadFile(latest) //nolint:gosec // G304: latest is under the test's own TempDir, not user input
	if err != nil {
		t.Fatalf("read flash-latest.zip: %v", err)
	}
	if !bytes.Equal(got, []byte("PK\x03\x04first")) {
		t.Fatalf("flash-latest.zip bytes = %q, want the DumpZip payload", got)
	}
	if _, err := os.Stat(filepath.Join(dir, ".flash-export.tmp.zip")); !os.IsNotExist(err) {
		t.Fatalf("temp file must be gone after a successful export, stat err = %v", err)
	}

	// A second export overwrites flash-latest.zip with the new payload.
	fake.dumpBytes = []byte("PK\x03\x04second")
	if err := svc.exportFlashZip(context.Background(), settings, "cafebabe", restic.Mode{}, "/repo"); err != nil {
		t.Fatalf("second exportFlashZip: %v", err)
	}
	got, err = os.ReadFile(latest) //nolint:gosec // G304: latest is under the test's own TempDir, not user input
	if err != nil {
		t.Fatalf("read flash-latest.zip after overwrite: %v", err)
	}
	if !bytes.Equal(got, []byte("PK\x03\x04second")) {
		t.Fatalf("flash-latest.zip not overwritten, bytes = %q", got)
	}
	// Only the single latest file (no timestamped clutter) when Keep==0.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "flash-latest.zip" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("Keep==0 export dir = %v, want only [flash-latest.zip]", names)
	}
}

// TestPruneFlashZips proves pruneFlashZips keeps only the newest `keep`
// timestamped flash-<ts>.zip files and never touches a non-matching file.
func TestPruneFlashZips(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"flash-20260101-000000.zip",
		"flash-20260102-000000.zip",
		"flash-20260103-000000.zip",
		"keepme.zip", // does not match flashZipRe → must survive
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	(&Service{}).pruneFlashZips(dir, 2)

	survivors := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		survivors[e.Name()] = true
	}

	want := []string{"flash-20260103-000000.zip", "flash-20260102-000000.zip", "keepme.zip"}
	for _, w := range want {
		if !survivors[w] {
			t.Errorf("%s should have survived pruning, dir now = %v", w, keys(survivors))
		}
	}
	if survivors["flash-20260101-000000.zip"] {
		t.Errorf("oldest timestamped zip should have been pruned, dir now = %v", keys(survivors))
	}
	if len(survivors) != len(want) {
		t.Errorf("survivor count = %d (%v), want %d", len(survivors), keys(survivors), len(want))
	}
}

// TestPruneFlashZipsKeepZeroDeletesAllTimestamped proves that pruning with
// keep==0 (latest mode) deletes ALL timestamped flash-<ts>.zip history left over
// from a previous history run, while flash-latest.zip and any non-matching file
// survive (they never match flashZipRe).
func TestPruneFlashZipsKeepZeroDeletesAllTimestamped(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"flash-20260101-000000.zip",
		"flash-20260102-000000.zip",
		"flash-20260103-000000.zip",
		"flash-latest.zip", // does not match flashZipRe → must survive
		"keepme.zip",       // does not match flashZipRe → must survive
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	(&Service{}).pruneFlashZips(dir, 0)

	survivors := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		survivors[e.Name()] = true
	}

	for _, ts := range []string{"flash-20260101-000000.zip", "flash-20260102-000000.zip", "flash-20260103-000000.zip"} {
		if survivors[ts] {
			t.Errorf("%s should have been deleted by keep==0 prune, dir now = %v", ts, keys(survivors))
		}
	}
	want := []string{"flash-latest.zip", "keepme.zip"}
	for _, w := range want {
		if !survivors[w] {
			t.Errorf("%s should have survived keep==0 prune, dir now = %v", w, keys(survivors))
		}
	}
	if len(survivors) != len(want) {
		t.Errorf("survivor count = %d (%v), want %d", len(survivors), keys(survivors), len(want))
	}
}

// TestExportFlashZipDumpError proves a DumpZip failure surfaces as an error and
// leaves NOTHING behind — no temp file and no flash-*.zip.
func TestExportFlashZipDumpError(t *testing.T) {
	root := t.TempDir()
	fake := &flashZipFakeEngine{dumpErr: errors.New("boom")}
	svc := &Service{
		cfg:    config.Config{HostMountRoot: root, FlashDir: "/boot"},
		engine: fake,
	}
	settings := store.Settings{
		FlashZipExportEnabled: true,
		FlashZipExportPath:    "export",
	}

	err := svc.exportFlashZip(context.Background(), settings, "deadbeef", restic.Mode{}, "/repo")
	if err == nil {
		t.Fatal("expected an error when DumpZip fails")
	}

	dir := filepath.Join(root, "export")
	if _, statErr := os.Stat(filepath.Join(dir, ".flash-export.tmp.zip")); !os.IsNotExist(statErr) {
		t.Fatalf("temp file must be removed on dump error, stat err = %v", statErr)
	}
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("no zip must be written on dump error, dir = %v", names)
	}
}

// TestExportFlashZipDisabled proves the export is a no-op (nil, nothing written,
// DumpZip never called) when disabled or when the path is empty.
func TestExportFlashZipDisabled(t *testing.T) {
	cases := []struct {
		name     string
		settings store.Settings
	}{
		{"disabled", store.Settings{FlashZipExportEnabled: false, FlashZipExportPath: "export"}},
		{"empty path", store.Settings{FlashZipExportEnabled: true, FlashZipExportPath: ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			fake := &flashZipFakeEngine{dumpBytes: []byte("nope")}
			svc := &Service{
				cfg:    config.Config{HostMountRoot: root, FlashDir: "/boot"},
				engine: fake,
			}

			if err := svc.exportFlashZip(context.Background(), tc.settings, "deadbeef", restic.Mode{}, "/repo"); err != nil {
				t.Fatalf("exportFlashZip should be a nil no-op, got %v", err)
			}
			if fake.dumpCalls != 0 {
				t.Fatalf("DumpZip must not be called (calls = %d)", fake.dumpCalls)
			}
			if _, err := os.Stat(filepath.Join(root, "export")); !os.IsNotExist(err) {
				t.Fatalf("no output folder should be created, stat err = %v", err)
			}
		})
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
